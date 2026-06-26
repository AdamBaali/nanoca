# Fleet built‑in ACME CA → attestation for every device

**Phase 1 plan + macOS 27 research.** Working notes, kept out of the Fleet repo. Drafted 2026‑06‑26.

> Origin: an off‑the‑record conversation with a senior MDM engineer after the SCEP article.
> The source asked not to be attributed (employer reasons), so this doc stays anonymous and lives
> in `nanoca`, not `fleetdm/fleet`. The ideas are theirs; the framing here is ours to verify.

---

## 0. TL;DR — the plan changed once we read the code

The first sketch assumed we'd *build* a built‑in CA. We won't — **Fleet already shipped one.**

- **Fleet 4.84** added a built‑in **ACME server** for MDM enrollment identity, with a Premium
  **"Require hardware attestation"** toggle. Only DEP‑synced, attestation‑passing **Apple Silicon Macs**
  get ACME today. (FR **#15611** → story **#31289**, sub‑tasks **#40991/#40994**.)
- **Fleet 4.86** surfaced hardware‑bound ACME certs in Host details (**#42827/#46725**) and handled
  **renewal** via `$FLEET_VAR_CERTIFICATE_RENEWAL_ID` (**#40639**).
- In code: `server/mdm/acme/**` implements all 10 RFC 8555 endpoints, is **wired live** in
  `cmd/fleet/serve.go`, validates Apple `device-attest-01` end‑to‑end, and **already shares the CA
  signer and `identity_certificates` table with SCEP** (migration `20260401153000`).

So "Phase 1 — unify the CA core" is **largely done in production.** The real frontier — and what this
plan targets — is the open work:

1. **Extend ACME + attestation beyond Apple‑DEP‑Macs to *every* device type** (the user's core ask).
2. **Leverage macOS 27 ("Golden Gate")**, where attested ACME identities become the credential for
   DDM‑delivered VPN, Wi‑Fi, and Platform SSO.
3. **Make "Fleet CA" a coherent product surface**, including the open BYO/external‑CA requests
   (**#44789**, **#43376**).
4. **Harden** the shipped server (bug **#46282**; missing `revokeCert`/`keyChange`; attestation
   public‑key persistence TODO).

`nanoca` is the prototype vehicle for #1–#2: it's the same architecture as Fleet's `server/mdm/acme`,
it's *not* the Fleet repo, and it already has the pluggable seams we need.

---

## 1. Current state (verified against the repos, 2026‑06‑26)

| Capability | State | Where |
|---|---|---|
| Built‑in ACME server (RFC 8555, 10 endpoints) | **Shipped 4.84** | `server/mdm/acme/internal/service/handler.go` |
| Apple `device-attest-01` validation | **Shipped** | `…/internal/service/challenge.go` (Apple root CA chain → nonce → serial → DEP check) |
| "Require hardware attestation" setting | **Shipped** | `apple_require_hardware_attestation` (#40994) |
| ACME ⇄ SCEP shared signer + `identity_certificates` | **Shipped** | migration `20260401153000`; `scepdepot.Signer` used by both |
| ACME certs in Host details + renewal | **Shipped 4.86** | #42827, #46725, #40639 |
| `revokeCert` / `keyChange` endpoints | **In directory, not implemented** | gap |
| Attestation **leaf public‑key** persistence | **TODO** | `challenge.go:170` |
| Non‑Apple attestation (TPM / Android) | **Not started** | only `format == "apple"` |
| Attestation source = **DEP only** | **Hard‑coded** | `IsDEPEnrolled` provider |
| First‑class "Fleet CA" CA type / BYO‑ACME / external CA | **Not started** | open FRs #44789, #43376 |
| CRL / OCSP | **Gap** (DB `revoked` flag only) | — |
| `/api/mdm/acme` 5XX on bad account id | **Open bug #46282** | — |
| Linux host identity = TPM key, **SCEP**, no attestation | **Shipped, gap** | `ee/orbit/pkg/securehw` + `tpm-backed-http-signing.md` ("switch to ACME" is the stated long‑term plan) |

**Relevant open FRs** (the live frontier): **#44789** configure a custom ACME server for attestation;
**#43376** external CA for MDM enrollment certs; **#18122** *Research: ACME v. SCEP* (still open);
**#37286** TPM EK pubkey for Smallstep attestation (non‑Apple); **#46282** ACME 5XX bug.

---

## 2. macOS 27 "Golden Gate" — what it changes for this

Sources: Apple Platform Deployment (ACME payload + Managed Device Attestation), WWDC22 s.10143,
Apple "What's new in macOS Tahoe 26," and `articles/wwdc-2026-what-it-admins-need-to-know.md`.
**Pre‑release — confirmed vs. speculative is marked.**

**Confirmed and material:**

- **macOS 27 is Apple‑Silicon‑only.** Tahoe (26) was the last Intel release. Consequence: **every
  macOS 27 Mac has a Secure Enclave**, so **hardware‑bound, attested ACME keys are universally
  available** across the Mac fleet — no Intel exception to design around.
- **DDM delivers VPN configs, with credentials as declarative assets that auto‑renew.** This is the
  ideal *consumer* of an attested ACME identity.
- **Extensible/Platform SSO move into DDM** (`com.apple.configuration.extensible-sso`); Platform SSO
  gains web‑IdP auth at login/FileVault.
- **`device.system.health`** (iPhone/iPad) reports hardware‑component genuineness — a tamper signal
  that complements attestation.
- **Stricter TLS 1.2/ATS floor** for management traffic; legacy software‑update MDM commands removed.

**Speculative / unverified:** No published evidence of **new ACME‑payload keys or new attestation
OIDs** specific to 27. The Golden Gate story is *plumbing that consumes* attested certs (DDM VPN
credential assets, DDM SSO) — not changes to the attestation protocol itself. Treat any "new
device‑attest mechanics in 27" claim as unconfirmed until Apple's deployment docs catch up.

**The opportunity:** time a release so that Fleet's built‑in ACME CA issues the attested identity
that macOS 27's DDM VPN/SSO assets then *consume*. That's the "soak up the interest" window — the CA
isn't just enrollment identity anymore, it's the root of the device's network & SSO credentials.

### ACME / Managed Device Attestation capability matrix

| Platform | ACME payload | HW‑bound SE key | Attestation | Min OS | Silicon |
|---|---|---|---|---|---|
| iPhone | ✅ | ✅ | ✅ | iOS 16 | A11+ |
| iPad | ✅ | ✅ | ✅ | iPadOS 16.1 | A11+ |
| Mac | ✅ (device + user channel) | ✅ | ✅ | macOS 14 | **Apple Silicon** (moot in 27 — AS‑only) |
| Apple TV | ✅ | ✅ | ✅ | tvOS 16 | A11+ |
| Vision Pro | ✅ | ✅ | ✅ | visionOS 1.1 | AS |
| Apple Watch | ✅ (payload) | limited | not a primary target | watchOS 10 | — |

Constraints: attestation requires EC `ECSECPrimeRandom` **P‑256/384** (P‑384 recommended) and
`Attest` ⇒ `HardwareBound`; ~**1 fresh attestation / device / 7 days**; up to **10** attested ACME
payloads/device; SE keys are **not** restored from backup; **serial/UDID OIDs are omitted under User
Enrollment** (BYOD privacy — design for it).

---

## 3. The thesis: one built‑in ACME CA, strongest attestation each platform allows

Extend the *existing* built‑in ACME CA from "Apple DEP Macs only" to **every managed device**, each
getting the strongest hardware‑backed identity its silicon supports:

- **Apple (Mac / iOS / iPadOS / tvOS / visionOS)** — `device-attest-01`, Secure Enclave, EC P‑384.
  Already works for DEP Macs; extend to iOS/iPadOS and to **BYOD/User Enrollment** (no serial/UDID —
  authorize on the Managed Apple Account / enrollment binding instead of ABM membership).
- **Linux & Windows (TPM 2.0)** — TPM attestation (`draft-ietf-acme-device-attest` TPM format / EK +
  AK quote). This is the documented long‑term replacement for Fleet's host‑identity **SCEP** path and
  **closes the attestation gap** the in‑repo doc already calls out. Ties to **#37286**.
- **Android** — Key Attestation / Play Integrity (later; Fleet's Android MDM is newer).

### Why this is more secure than SCEP (the #18122 answer, 5 points)

1. **No shared secret to steal.** SCEP trusts a challenge password in the profile; leak/replay it and
   *any* machine mints a corporate cert. ACME uses a per‑request nonce + signed attestation — nothing
   static to exfiltrate.
2. **Proof the key is hardware‑bound and non‑exportable.** ACME verifies the CSR key *is* the
   SE/TPM key (public‑key match in the attestation). SCEP can't tell an SE key from a file on disk.
3. **Cryptographic proof of a genuine, specific device.** Apple's (or the TPM's) attestation chain
   vouches serial/UDID/sepOS — Apple‑signed, not self‑asserted.
4. **Stolen profile is inert.** Without the physical chip, a captured ACME profile issues nothing.
5. **Identity bound end‑to‑end.** The same SE/TPM key signs the attestation, the CSR, the issued cert,
   and the subsequent TLS — relying parties know they're talking to *that* device. Real Zero Trust.

---

## 4. nanoca ⇄ Fleet — why prototype here

`nanoca` (`github.com/brandonweeks/nanoca`, this fork) is a pluggable ACME CA whose interfaces are the
**same shape** as Fleet's bounded context:

| Concept | nanoca | Fleet `server/mdm/acme` |
|---|---|---|
| Validate attestation | `AttestationVerifier.Format()/Verify()` | `challenge.go` `format` switch (apple only) |
| Decide who may enroll | `Authorizer.Authorize(DeviceInfo)` | `DataProviders.IsDEPEnrolled(serial)` |
| Sign the cert | `CertificateIssuer` / signer | `CSRSigner.SignCSR` (= `scepdepot.Signer`) |
| Post‑issuance hook | `IssuanceObserver.OnIssuance` | (Fleet activities / host cert ingest) |

nanoca already has: an **Apple verifier**, a **WebAuthn verifier**, an **ABM authorizer**, an ABM API
client, a `HardwareModule` (TPM) identifier type, and a standalone server (`cmd/mpc-server`,
deployed at `cert.mpc.ad`). What it lacks is exactly the frontier: a **TPM verifier**, and the prod
server still runs the **`null` authorizer** (any attested device gets a cert — README flags this).

**Plan:** prove the multi‑format verifier + real authorizer model in nanoca (fast, isolated, no Fleet
review cycle), then port the validated patterns into Fleet's `challenge.go`/`providers.go`.

---

## 5. Phase 1 implementation roadmap

Sequenced so each increment ships value and de‑risks the next. **Prototype increments land in
`nanoca`; Fleet‑side increments are scoped here but not coded until the §6 decisions are made.**

**1a — Harden the shipped Fleet ACME server.** *(Fleet; small, high‑confidence)*
Fix #46282 (5XX → proper ACME problem doc on bad account id); implement `revokeCert` and `keyChange`
(in the directory, unbuilt); persist the attestation **leaf public key** (`challenge.go:170`) for
later re‑validation. Low risk, immediately useful, no design decisions required.

**1b — First‑class "Fleet CA" + external/BYO CA.** *(Fleet; needs §6 decisions)*
Add `CATypeFleetCA` alongside ndes/digicert/etc. in `server/fleet/certificate_authorities.go`, so the
built‑in CA is a visible, configurable object — and the slot where **#44789** (BYO ACME) and **#43376**
(external CA for enrollment certs) plug in. One settings surface; one mental model.

**1c — Generalize the attestation layer to multiple formats.** *(nanoca first, then Fleet)*
In nanoca: add `verifiers/tpm` implementing `AttestationVerifier` for the TPM format (EK cert chain →
AK quote → nonce), exercised against the `HardwareModule` identifier path. Then mirror in Fleet's
`challenge.go` format switch (`apple` → `apple|tpm|android`).

**1d — Generalize the authorizer beyond DEP.** *(nanoca first, then Fleet)*
Today Fleet = `IsDEPEnrolled(serial)`; nanoca prod = `null`. Define `IsDeviceEnrolled` semantics per
platform: ABM membership (Apple ADE), Managed Apple Account (BYOD/User Enrollment — no serial),
enroll‑secret/inventory match (TPM hosts). Harden nanoca's ABM authorizer: it currently lists **all**
org devices on every request — switch to a single‑serial lookup + short TTL cache (ABM rate limits).

**1e — Migrate Linux host identity SCEP → ACME w/ TPM attestation.** *(Fleet + orbit; larger)*
The in‑repo `tpm-backed-http-signing.md` already names ACME as the fix for renewal‑interruption bugs.
Reuse the 1c TPM verifier; retire the Windows/macOS `securehw` stubs over time.

**1f — macOS 27 consumers.** *(Fleet; time to Golden Gate)*
Wire attested ACME identities into DDM‑delivered **VPN credential assets** and **Platform SSO**, so the
built‑in CA roots the device's network + SSO credentials, not just MDM enrollment identity.

---

## 6. Open decisions (for the YVR conversation with Mike)

1. **What is `server/mdm/acme` *meant* to become** — Apple‑MDA‑only, or the general Fleet CA? Gates 1b–1d.
2. **Built‑in vs. external CA as the default**, and how #44789/#43376 relate to the built‑in path.
3. **Trust model & key custody** — per‑tenant roots vs. Fleet root + per‑tenant intermediates; CA key
   in DB‑encrypted assets (today) vs. KMS/HSM. The security crux.
4. **Revocation posture** — CRL, OCSP, or **short‑lived certs + frequent ACME renewal** (arguably the
   cleanest modern answer, and it sidesteps revocation infra).
5. **User Enrollment / BYOD privacy** — serial/UDID OIDs are absent; authorize on Managed Apple Account.
6. **Tiering** — built‑in CA for the 80%; external/BYO as the enterprise path.

---

## 7. What was *not* done here, and why

- **No changes to the Fleet repo.** Per the standing instruction, all output stays out of
  `fleetdm/fleet`. Fleet‑side increments (1a, 1b, 1e, 1f) are scoped, not coded.
- **No speculative feature code yet** — the highest‑value builds (TPM verifier wiring, BYO‑CA) depend
  on the §6 decisions (trust model, BYO‑vs‑built‑in). Coding them before that risks rework.
- **Recommended first action on a green light:** increment **1c** in nanoca — add `verifiers/tpm` and
  prove the multi‑format attestation model end‑to‑end against the existing `HardwareModule` path. It's
  isolated, fully testable offline, needs no Fleet review, and directly demonstrates "ACME for all
  device types." 1a (Fleet hardening) is the safest *Fleet‑side* starting point once repo output is OK.

---

*File references verified by read‑only passes of `fleetdm/fleet` and this `nanoca` fork on 2026‑06‑26.
Re‑verify before implementation. macOS 27 details are pre‑release.*
