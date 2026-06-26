# TPM manufacturer attestation roots

This directory holds the curated TPM 2.0 manufacturer **EK/AK root and
intermediate CA** certificates that nanoca trusts when validating `tpm`
attestation statements. Drop `*.pem` / `*.crt` files here and they are embedded
into the binary at build time (`//go:embed roots`) and loaded by
`tpm.WithEmbeddedRoots()`.

## Why this exists (and the on-prem rule)

Verification must work **fully offline**. We never dereference a certificate's
AIA URL or a TPM's `EK.CertificateURL` at issuance time — Fleet customers
self-host, sometimes air-gapped, and the CA must never phone home. That means
every root and intermediate needed to build the AK chain has to be present
either in this bundle or sent by the agent in the attestation `x5c`.

`tpm.New(...)` refuses to start with an empty trust pool (no implicit trust),
mirroring step-ca's `attestationRoots`. Operators can extend or override this
bundle at runtime with `tpm.WithRootsDir("/etc/nanoca/tpm-roots")`.

## Sourcing the bundle (manual, verify provenance)

These are intentionally **not** auto-fetched. Populate them deliberately:

- **Microsoft `TrustedTpm.cab`** — ships roots + intermediates for the major
  vendors (Infineon, STMicroelectronics, Nuvoton, Intel, AMD, Microsoft
  fTPM/Pluton). Download, expand, and convert per-vendor DER to PEM.
- **Vendor PKI pages** — Infineon (OPTIGA), STMicroelectronics, Nuvoton, Intel
  (ODCA / EKOP — include the EICA intermediates so the Intel PTT/fTPM chain
  builds without the network), AMD.

Name files by vendor, e.g. `infineon-optiga-rsa-root.pem`,
`stm-tpm-ecc-root.pem`, `intel-odca-eica-intermediate.pem`. Record where each
came from and its fingerprint in a sibling `PROVENANCE.md` before trusting it.

> The bundle is empty in this scaffold on purpose — shipping fabricated or
> unverified roots would be worse than shipping none. `WithEmbeddedRoots()` is a
> no-op until real, provenance-checked certs land here.
