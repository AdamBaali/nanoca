package tpm

import (
	"crypto/x509"
	"embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// embeddedRootsFS holds the curated TPM manufacturer root/intermediate CAs that
// ship with nanoca. See roots/README.md for how the bundle is sourced and how
// operators add their own. Embedding keeps verification fully offline.
//
//go:embed roots
var embeddedRootsFS embed.FS

// rootsConfig accumulates trust material while applying Options.
type rootsConfig struct {
	roots         *x509.CertPool
	rootCount     int
	intermediates []*x509.Certificate
	skipChain     bool
}

// Option configures the TPM verifier's trust pool.
type Option func(*rootsConfig) error

// WithRootsPEM adds trusted TPM manufacturer root CAs from PEM bytes.
func WithRootsPEM(pemBytes []byte) Option {
	return func(c *rootsConfig) error {
		n, err := appendRoots(c.roots, pemBytes)
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("WithRootsPEM: no certificates found in PEM")
		}
		c.rootCount += n
		return nil
	}
}

// WithIntermediatesPEM bundles intermediate CAs (e.g. Intel EICA/ODCA) so the
// AK chain can be built offline without dereferencing AIA URLs.
func WithIntermediatesPEM(pemBytes []byte) Option {
	return func(c *rootsConfig) error {
		certs, err := parseCerts(pemBytes)
		if err != nil {
			return err
		}
		c.intermediates = append(c.intermediates, certs...)
		return nil
	}
}

// WithRootsDir loads every *.pem / *.crt file in dir as trusted roots. This is
// the operator-supplied, on-prem trust store (mirrors step-ca's
// attestationRoots): point it at your organization's TPM CA bundle.
func WithRootsDir(dir string) Option {
	return func(c *rootsConfig) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("WithRootsDir: %w", err)
		}
		var total int
		for _, e := range entries {
			if e.IsDir() || !isCertFile(e.Name()) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return fmt.Errorf("WithRootsDir: read %s: %w", e.Name(), err)
			}
			n, err := appendRoots(c.roots, b)
			if err != nil {
				return fmt.Errorf("WithRootsDir: %s: %w", e.Name(), err)
			}
			total += n
		}
		if total == 0 {
			return fmt.Errorf("WithRootsDir: no certificates found in %s", dir)
		}
		c.rootCount += total
		return nil
	}
}

// WithEmbeddedRoots loads the curated manufacturer roots bundled with nanoca.
// It is a no-op (adds zero roots) if the bundle is empty, so it composes safely
// with WithRootsDir/WithRootsPEM.
func WithEmbeddedRoots() Option {
	return func(c *rootsConfig) error {
		return fs.WalkDir(embeddedRootsFS, "roots", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !isCertFile(d.Name()) {
				return nil
			}
			b, err := embeddedRootsFS.ReadFile(path)
			if err != nil {
				return err
			}
			n, err := appendRoots(c.roots, b)
			if err != nil {
				return fmt.Errorf("embedded root %s: %w", path, err)
			}
			c.rootCount += n
			return nil
		})
	}
}

// WithInsecureSkipChainVerification disables AK-certificate chain validation.
// FOR TESTS ONLY. It removes the proof that the key lives in genuine TPM
// hardware and must never be enabled in production.
func WithInsecureSkipChainVerification() Option {
	return func(c *rootsConfig) error {
		c.skipChain = true
		return nil
	}
}

func appendRoots(pool *x509.CertPool, pemBytes []byte) (int, error) {
	certs, err := parseCerts(pemBytes)
	if err != nil {
		return 0, err
	}
	for _, c := range certs {
		pool.AddCert(c)
	}
	return len(certs), nil
}

func parseCerts(pemBytes []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, nil
}

func isCertFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pem", ".crt", ".cer":
		return true
	default:
		return false
	}
}
