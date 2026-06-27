# How the CAs do it — and where Fleet wins

Concrete competitive teardown for the built-in ACME CA strategy. June 2026.
Companion to `docs/tpm-attestation-approach.md`.

## How step-ca does it (the reference)

step-ca is the open-source CA behind Smallstep — and behind **Jamf's** own ACME
server. Device attestation, concretely:

1. ACME provisioner with `challenges: ["device-attest-01"]` (off by default) and
   `attestationFormats: ["apple","tpm","step"]`.
2. `attestationRoots` = base64 PEM bundle. Apple default-trusted; **TPM trusts
   nothing until you supply vendor EK roots**.
3. Device returns an attestation; step-ca verifies it chains to a trusted root +
   key authorization matches. **Proves the hardware is genuine.**
4. It does **not** prove the device is yours. That decision is an `AUTHORIZING`
   **webhook you build and host** that returns `{"allow": true}`, mapping the
   attested `permanentIdentifier` to your inventory.

Defaults: 24h certs + automated renewal; **passive revocation only** (no CRL/OCSP
in OSS — that's paid). MySQL/Postgres for HA; PKCS#11/KMS for keys.

**OSS vs paid:** OSS = full CA + ACME + attestation *verification* + self-host /
air-gap. Paid = active revocation (CRL+OCSP), dashboard, **device inventory**, the
**Smallstep Agent (cloud-locked)**, MDM/IdP sync. Per-endpoint-month, sales-gated.

## The strategic seam: every standalone CA is fleet-blind

`device-attest-01` answers "is this genuine hardware?" — never "is this device
mine?" Two tells:

- Smallstep's platform lists **Fleet DM as a device-inventory source** — they
  import the context an MDM owns.
- Their entire paid tier is the device-context layer they had to invent because
  the CA engine lacks it. Jamf built its CA on step-ca and still only covers Apple.

## Capability matrix (decisive columns)

| Product | Built-in CA | ACME attest (Apple) | TPM attest | Self-host/air-gap | Owns fleet | Agent |
|---|---|---|---|---|---|---|
| Jamf Pro | ✅ | ✅ (step-ca, Apple-only) | ❌ | cloud-first | ✅ | ✅ |
| EJBCA | ✅ | ✅ (PoC?) | ✅ | ✅ | ❌ | ❌ |
| MS Cloud PKI / Intune | ✅ | ❌ (SCEP) | ❌ | ❌ | via Intune | via Intune |
| SCEPman | ✅ | ❌ | ❌ | own tenant | ❌ | ❌ |
| Kandji/Mosyle/Addigy | ❌ | broker | ❌ | ❌ | ✅ | ✅ |
| Vault PKI | ✅ | ❌ (HTTP/DNS only) | ❌ | ✅ | ❌ | ❌ |
| Venafi Firefly | ephemeral | ❌ | ❌ | dev only | ❌ | ❌ |
| Google CAS / AWS PCA | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| HydrantID | ✅ | EAB (unverified) | ❌ | ❌ | ❌ | ❌ |
| JumpCloud | ✅ | ❌ (SCEP) | ❌ | ❌ | ✅ | ✅ |
| **Fleet (target)** | ✅ | **shipped** | **building** | ✅ | ✅ | ✅ |

## White space (unserved by any single incumbent)

CAs that do Apple+TPM attestation and self-host (EJBCA, partly Vault) have **no
fleet/agent/ABM**. MDMs that own fleet/agent/ABM (Jamf, Kandji, JumpCloud) have no
attestation, broker it, or are Apple-only + cloud-first. Cloud CAs are cloud-only
+ SCEP/EAB. **Nobody** is simultaneously (a) the fleet owner, (b) an
attestation-validating ACME CA across Apple + TPM, and (c) self-hostable/air-gap.

## Where Fleet beats them (mapped to what exists)

- **Built** — in-process `Authorizer.Authorize(deviceInfo)` (ABM-backed) replaces
  step-ca's DIY allow/deny webhook. No second service.
- **Built** — Apple **and** TPM verifiers in one CA (Jamf can't do TPM); TPM
  verifier on `claude/tpm-attestation-verifier` via go-attestation.
- **Built** — fully offline TPM verification (bundled/operator root pool, no
  network). Smallstep's agent is cloud-locked.
- **Built** — ABM zero-touch authorization (authorizer + client present).
- **To build** — native active revocation driven by device lifecycle (Fleet knows
  retire/wipe/unenroll); or short-lived + ACME renewal. step-ca OSS has neither.
- **To build** — issue/install/renew through the agent already on every host
  (no second agent).

## Proof the demand is real

Fleet **#37998** (Hydrant SCEP→ACME, summer 2026) explicitly wants
hardware-attestation-backed ACME where a tampered device's certs auto-invalidate,
driven by an MDM owning inventory + agent + ABM — this white space, verbatim.
**#15611** is the standing promise.

*Verify SCEPman / EJBCA-GA / HydrantID specifics before external citation.*
