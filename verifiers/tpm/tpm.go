// Package tpm verifies WebAuthn "tpm" attestation statements as used by the
// ACME device-attest-01 challenge (draft-ietf-acme-device-attest) for TPM 2.0
// devices such as Linux and Windows hosts.
//
// It follows the same structure as the Apple Managed Device Attestation
// verifier in this repo, and reuses Google's go-attestation
// (github.com/google/go-attestation) layered on go-tpm — the same libraries
// Fleet's agent already uses for TPM key handling — so the verification logic
// matches the patterns in Brandon Weeks' attestation work.
//
// Verification is FULLY OFFLINE. The Attestation Identity Key (AK) certificate
// chain is validated only against a caller-supplied pool of TPM manufacturer
// root CAs; no AIA or EK CertificateURL is ever dereferenced. This is a hard
// requirement: Fleet's customers self-host, often air-gapped, so the CA must
// never phone home at issuance time.
package tpm

import (
	"context"
	"crypto"
	_ "crypto/sha1" // register SHA-1 for legacy AK signing schemes
	"crypto/sha256"
	_ "crypto/sha512" // register SHA-384/512 for AK signing schemes
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"log/slog"

	"github.com/brandonweeks/nanoca"
	"github.com/google/go-attestation/attest"
	tpm2 "github.com/google/go-tpm/legacy/tpm2"
)

var (
	// oidAIKCertificate is the TCG OID tcg-kp-AIKCertificate (2.23.133.8.3),
	// the extended key usage that marks a certificate as an Attestation
	// Identity Key certificate.
	oidAIKCertificate = asn1.ObjectIdentifier{2, 23, 133, 8, 3}

	// hwTypeTPM2 identifies a TPM 2.0 hardware module (TCG 2.23.133.1.2). Used
	// as the HardwareModuleName hwType when reporting device identity.
	hwTypeTPM2 = asn1.ObjectIdentifier{2, 23, 133, 1, 2}
)

// AttestationVerifier verifies the "tpm" attestation format.
type AttestationVerifier struct {
	logger        *slog.Logger
	trustedRoots  *x509.CertPool
	intermediates []*x509.Certificate
	skipChain     bool
}

// New builds a TPM attestation verifier. At least one trusted manufacturer root
// must be supplied (WithEmbeddedRoots / WithRootsPEM / WithRootsDir), unless
// WithInsecureSkipChainVerification is set — which is for tests only and is
// named to be impossible to enable by accident.
func New(logger *slog.Logger, opts ...Option) (*AttestationVerifier, error) {
	cfg := &rootsConfig{roots: x509.NewCertPool()}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, fmt.Errorf("tpm verifier: %w", err)
		}
	}
	if !cfg.skipChain && cfg.rootCount == 0 {
		return nil, errors.New("tpm verifier: no trusted TPM manufacturer roots configured; " +
			"use WithEmbeddedRoots/WithRootsPEM/WithRootsDir (or WithInsecureSkipChainVerification for tests)")
	}
	return &AttestationVerifier{
		logger:        logger,
		trustedRoots:  cfg.roots,
		intermediates: cfg.intermediates,
		skipChain:     cfg.skipChain,
	}, nil
}

// Format returns the attestation format identifier.
func (v *AttestationVerifier) Format() string { return "tpm" }

// Verify validates a TPM 2.0 attestation statement and returns the attested
// device identity. challenge is the ACME challenge value to bind; the verifier
// expects certInfo.extraData to equal its SHA-256 (see the package approach doc
// for the draft-compliant key-authorization binding).
func (v *AttestationVerifier) Verify(ctx context.Context, stmt nanoca.AttestationStatement, challenge []byte) (*nanoca.DeviceInfo, error) {
	if stmt.Format != "tpm" {
		return nil, fmt.Errorf("format mismatch: expected tpm, got %s", stmt.Format)
	}
	if ver, _ := stmt.AttStmt["ver"].(string); ver != "2.0" {
		return nil, fmt.Errorf("unsupported TPM attestation version %q (only 2.0 is supported)", ver)
	}

	certInfo, err := bytesField(stmt.AttStmt, "certInfo")
	if err != nil {
		return nil, err
	}
	pubArea, err := bytesField(stmt.AttStmt, "pubArea")
	if err != nil {
		return nil, err
	}
	sig, err := bytesField(stmt.AttStmt, "sig")
	if err != nil {
		return nil, err
	}
	certChain, err := parseX5C(stmt.AttStmt)
	if err != nil {
		return nil, err
	}
	akCert := certChain[0]

	// 1. The AK certificate must chain to a trusted TPM manufacturer root,
	//    using only the offline pool and the intermediates sent in x5c — never
	//    the network. This is what proves the key lives in genuine TPM hardware.
	if err := v.verifyAKChain(akCert, certChain[1:]); err != nil {
		return nil, fmt.Errorf("AK certificate chain verification failed: %w", err)
	}
	// 2. The AK certificate must be a well-formed AIK.
	if err := validateAKCertificate(akCert); err != nil {
		return nil, fmt.Errorf("AK certificate is not a valid AIK: %w", err)
	}

	// 3. Core TPM certification check (go-attestation): verifies the signature
	//    over certInfo by the AK, that magic == TPM_GENERATED_VALUE, the key
	//    length/curve is secure, the key attributes prove it is TPM-resident,
	//    non-exportable, non-duplicable and TPM-generated, and that certInfo's
	//    attested name matches pubArea.
	hash, err := coseHash(stmt.AttStmt["alg"])
	if err != nil {
		return nil, err
	}
	params := attest.CertificationParameters{
		Public:            pubArea,
		CreateAttestation: certInfo,
		CreateSignature:   sig,
	}
	if err := params.Verify(attest.VerifyOpts{Public: akCert.PublicKey, Hash: hash}); err != nil {
		return nil, fmt.Errorf("TPM key certification failed: %w", err)
	}

	// 4. Freshness: certInfo.extraData must equal SHA-256(challenge). Constant
	//    time to avoid leaking the comparison.
	att, err := tpm2.DecodeAttestationData(certInfo)
	if err != nil {
		return nil, fmt.Errorf("decode attestation data: %w", err)
	}
	expected := sha256.Sum256(challenge)
	if subtle.ConstantTimeCompare([]byte(att.ExtraData), expected[:]) != 1 {
		return nil, errors.New("attestation nonce mismatch: certInfo.extraData does not match challenge")
	}

	// 5. The attested (credential) key — this MUST be bound to the order CSR at
	//    finalize so the cert is issued for the attested key and not a
	//    substituted one. See the approach doc; the binding is a required
	//    follow-up in the finalize path.
	pub, err := tpm2.DecodePublic(pubArea)
	if err != nil {
		return nil, fmt.Errorf("decode pubArea: %w", err)
	}
	if _, err := pub.Key(); err != nil {
		return nil, fmt.Errorf("extract attested public key: %w", err)
	}

	v.logger.DebugContext(ctx, "TPM attestation verified",
		"ak_subject", akCert.Subject.String(),
		"ak_serial", akCert.SerialNumber.String(),
	)
	return deviceInfo(akCert), nil
}

// verifyAKChain validates the AK certificate against the offline root pool.
func (v *AttestationVerifier) verifyAKChain(ak *x509.Certificate, sent []*x509.Certificate) error {
	if v.skipChain {
		return nil
	}
	inter := x509.NewCertPool()
	for _, c := range v.intermediates {
		inter.AddCert(c)
	}
	for _, c := range sent {
		inter.AddCert(c)
	}
	_, err := ak.Verify(x509.VerifyOptions{
		Roots:         v.trustedRoots,
		Intermediates: inter,
		// AK certs carry the AIK EKU, not standard TLS usages.
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	return err
}

// validateAKCertificate enforces the TCG AIK certificate constraints.
func validateAKCertificate(cert *x509.Certificate) error {
	if cert.Version != 3 {
		return errors.New("certificate is not X.509 v3")
	}
	if cert.IsCA {
		return errors.New("AIK certificate must not be a CA certificate")
	}
	if !hasAIKEKU(cert) {
		return errors.New("missing tcg-kp-AIKCertificate extended key usage (2.23.133.8.3)")
	}
	return nil
}

func hasAIKEKU(cert *x509.Certificate) bool {
	for _, oid := range cert.UnknownExtKeyUsage {
		if oid.Equal(oidAIKCertificate) {
			return true
		}
	}
	return false
}

// deviceInfo derives the device identity reported to the Authorizer. The
// identifier is currently the SHA-256 of the AK public key. NOTE: the canonical
// TPM identity is the EK public key; richer extraction (manufacturer/model from
// the SAN, EK-pub-hash) is a documented follow-up in the approach doc.
func deviceInfo(ak *x509.Certificate) *nanoca.DeviceInfo {
	sum := sha256.Sum256(ak.RawSubjectPublicKeyInfo)
	return &nanoca.DeviceInfo{
		HardwareModule: &nanoca.HardwareModule{
			Type:  hwTypeTPM2,
			Value: sum[:],
		},
	}
}

// coseHash maps a COSE algorithm identifier (WebAuthn attStmt "alg") to a hash.
// Defaults to SHA-256 when absent, which is what TPM AKs commonly sign with.
func coseHash(alg any) (crypto.Hash, error) {
	a, ok := toInt64(alg)
	if !ok {
		return crypto.SHA256, nil
	}
	switch a {
	case -257, -7, -37: // RS256, ES256, PS256
		return crypto.SHA256, nil
	case -258, -35, -38: // RS384, ES384, PS384
		return crypto.SHA384, nil
	case -259, -36, -39: // RS512, ES512, PS512
		return crypto.SHA512, nil
	case -65535: // RS1
		return crypto.SHA1, nil
	default:
		return 0, fmt.Errorf("unsupported COSE algorithm %d", a)
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case uint64:
		return int64(n), true
	default:
		return 0, false
	}
}

func bytesField(m map[string]any, k string) ([]byte, error) {
	raw, ok := m[k]
	if !ok {
		return nil, fmt.Errorf("tpm attestation statement missing %q", k)
	}
	b, ok := raw.([]byte)
	if !ok {
		return nil, fmt.Errorf("tpm attestation %q must be a byte string", k)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("tpm attestation %q is empty", k)
	}
	return b, nil
}

func parseX5C(m map[string]any) ([]*x509.Certificate, error) {
	raw, ok := m["x5c"]
	if !ok {
		return nil, errors.New("tpm attestation statement missing x5c")
	}
	slice, ok := raw.([]any)
	if !ok || len(slice) == 0 {
		return nil, errors.New("x5c must be a non-empty array")
	}
	chain := make([]*x509.Certificate, 0, len(slice))
	for i, e := range slice {
		der, ok := e.([]byte)
		if !ok {
			return nil, fmt.Errorf("x5c[%d] must be a byte string", i)
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse x5c[%d]: %w", i, err)
		}
		chain = append(chain, c)
	}
	return chain, nil
}
