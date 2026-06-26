# TPM 2.0 device attestation for the built-in ACME CA — approach

**Branch:** `claude/tpm-attestation-verifier`. Extends nanoca's attestation
support from Apple-only to TPM 2.0 (Linux, Windows), so the built-in ACME CA can
issue hardware-attested certs to non-Apple devices — the gap Brandon called out.

## Constraints this design is built around

- **Self-hosted / on-prem, possibly air-gapped.** Fleet's market runs the server
  themselves. Verification is therefore **fully offline**: no AIA fetch, no EK
  `CertificateURL` dereference, no manufacturer callout at issuance. Go's
  `x509.Verify` does not fetch AIA, and we never call the network — the agent
  must send the full AK chain in `x5c`, and any needed intermediates (e.g. Intel
  EICA/ODCA) are bundled.
- **No implicit trust.** `tpm.New(...)` refuses to start without an explicit
  manufacturer root pool (mirrors step-ca's `attestationRoots`). Roots are
  bundled (`WithEmbeddedRoots`) and/or operator-supplied (`WithRootsDir`).
- **Follow existing patterns.** Same shape as `verifiers/apple` (struct with
  logger + trust pool, `Format()`/`Verify()`), and the same libraries Fleet's
  agent already depends on — `github.com/google/go-tpm` (v0.9.8, matching Fleet)
  and Brandon's `github.com/google/go-attestation` (v0.6.1).

## What's implemented (`verifiers/tpm/`)

`AttestationVerifier` implements nanoca's `AttestationVerifier` interface for the
WebAuthn `tpm` format (the format `draft-ietf-acme-device-attest` uses for TPM).
`Verify` performs, in order:

1. `ver == "2.0"`; pull `certInfo`, `pubArea`, `sig`, `x5c`, `alg`.
2. **AK chain → trusted manufacturer root**, offline, using bundled intermediates
   + those sent in `x5c`. This is what proves the key is in genuine TPM hardware.
3. **AIK constraints** on the AK cert (X.509 v3, non-CA, carries
   `tcg-kp-AIKCertificate` EKU `2.23.133.8.3`).
4. **Core certification check** via `attest.CertificationParameters.Verify` —
   signature over `certInfo` by the AK, `magic == TPM_GENERATED_VALUE`, secure
   key length/curve, key attributes (fixedTPM / non-exportable / non-duplicable /
   TPM-generated), and `certInfo` name binds to `pubArea`. (v0.6.1 handles both
   RSA and ECDSA AKs, so the historical "RSA-only" caveat does not apply.)
5. **Freshness** — `certInfo.extraData == SHA-256(challenge)`, constant-time.
6. Decode the attested credential key from `pubArea` (for CSR binding, below).

Trust pool: `roots.go` (`WithRootsPEM` / `WithIntermediatesPEM` / `WithRootsDir`
/ `WithEmbeddedRoots`, plus `WithInsecureSkipChainVerification` for tests only).
The embedded bundle (`roots/`) is intentionally empty in this scaffold —
shipping unverified roots is worse than shipping none; see `roots/README.md` for
sourcing (Microsoft `TrustedTpm.cab` + vendor PKI).

Tests cover the security defaults (no-implicit-trust), format/field validation,
and the COSE-alg→hash mapping. `go build ./... && go vet ./... && go test` pass.

## Required follow-ups before this is production-grade (no shortcuts hidden)

1. **Bind the attested key to the CSR at finalize.** The verifier interface only
   receives the challenge, not the order CSR, so today it cannot enforce that the
   cert is issued for the *attested* key. Without this, a device could attest a
   TPM key but finalize with a different (software) key — defeating the point.
   Fix: carry the attested public key (from `pubArea`) out of `Verify` and, at
   `finalizeCertificate`, require `CSR.PublicKey == attestedKey`. Minimal change:
   add `AttestedKey crypto.PublicKey` to `DeviceInfo`, persist it on the
   authorization, compare at finalize.
2. **Draft-compliant nonce.** `draft-ietf-acme-device-attest` binds the *key
   authorization* (`token || "." || base64url(thumbprint(accountKey))`), not the
   bare token. nanoca currently passes `challenge.Token`. Plumb the account-key
   thumbprint through so `extraData == SHA-256(keyAuthorization)`. (The Apple
   verifier shares this and should move together.)
3. **Linux bare-TPM path (EK + credential activation).** Many Linux TPMs have an
   EK cert but **no AK cert**, so the x5c-to-root path can't run. Add the
   `attest.ActivationParameters` flow: trust the **EK** cert to a manufacturer
   root, then bind a fresh AK to that EK via `ActivateCredential` (one extra
   round trip). This needs agent-side support in Fleet's `orbit` (which today
   mints a TPM key via go-tpm but exposes neither an AK cert nor the EK).
4. **DeviceInfo enrichment.** Derive the stable identity from the **EK public
   key** (canonical TPM identity) and parse manufacturer/model/version from the
   AK cert SAN, rather than hashing the AK SPKI as now.
5. **Curated root bundle** with `PROVENANCE.md` (fingerprints + sources).

## How this ports into Fleet (`server/mdm/acme`)

Fleet's `challenge.go` already has a `format` switch that only handles `apple`.
The mapping is 1:1:

- Add a `tpm` case mirroring this verifier; reuse `go-tpm`/`go-attestation`
  (go-tpm is already in Fleet's `go.mod`; add go-attestation).
- Fleet's `DataProviders` is the analogue of nanoca's `Authorizer`: extend
  `IsDEPEnrolled(serial)` to an `IsDeviceEnrolled` that, for TPM hosts, matches
  against Fleet's host inventory / enroll secret (#37286 wants the TPM EK pub in
  inventory — that's the join key).
- Trust roots: store the manufacturer bundle as a Fleet config asset
  (encrypted), surfaced in the CA settings UI, defaulting to the embedded bundle.
- The agent side (`orbit`/`securehw`) must produce an AK + attestation (and,
  for the bare-TPM path, expose the EK cert). This is the larger piece of work
  and the natural Fleet-side milestone after the verifier lands.

## Libraries added to the fork

`github.com/google/go-attestation v0.6.1`, `github.com/google/go-tpm v0.9.8`
(pinned to Fleet's version). Both pure-Go, no network at verify time.
