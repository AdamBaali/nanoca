# Built-in CA — product brief

Demand, market, ICP, use cases, pricing, positioning, and the honest strategic
tension. Synthesis of Fleet-internal demand (issues + handbook) and external
market research, June 2026. Planning doc — out of the Fleet repo.

## TL;DR

The demand is real, validated, and large — but **bimodal**, and that's the most
important finding. Fleet should ship **both** a built-in CA (the differentiated
wedge) **and** an external-CA broker (where the loudest current enterprise demand
sits). The built-in CA is the harder-to-copy moat; the broker is table stakes for
the 700+-host regulated ICP. The market timing — the CA/Browser Forum 47-day cliff
starting **March 2026** — is the strongest external forcing function we have.

## 1. Demand — strong, validated, bimodal

Among Fleet's hottest clusters: **~101 cert-related issues**, **43 distinct named
customers**, **22 `~customer promise`**, **4 `~feature fest`**. Demand is measured
in stacked customer codenames + sales-call citations, not +1s (these are
PM-authored from Gong/Slack).

**The tension to be honest about — two distinct asks:**

| Direction | Loudest evidence | What customers want |
|---|---|---|
| **Broker / deploy from *their* CA** | #21096 (**19 customers**), #13420 (**12** + CEO "Fleet as CA broker" vision), #43376, #37998, #44789 | Enterprises that already run DigiCert / AD CS / NDES / Google CA and (often for governance) **must** keep their own CA. |
| **Fleet *is* the CA (built-in + attestation)** | #15611 (shipped 4.84), #46723/#46725, #28974 Hydrant + Request-a-Cert API | The underserved 80% with no PKI, plus the hardware-attestation / Zero-Trust frontier. |

So "Fleet becomes the CA" (the thrust of this whole project) is the **differentiated
wedge** — but it is *not* where the largest validated current demand sits. The
biggest issues are broker/BYO-CA. **Recommendation: don't pick one.** The built-in
CA is the moat (no competitor has it cross-platform + self-hostable + fleet-aware);
the broker serves today's loudest enterprise asks and the regulated ICP that can't
use a Fleet-issued root. They're complementary, and Fleet already shipped pieces of
both.

**The dominant *use case* behind nearly all of it is Wi-Fi / VPN / 802.1X network
access** — followed by Zero-Trust / conditional access, MDM enrollment identity, and
mTLS for fleetd. Recurring secondary theme: OS parity (loudest for **Linux + Windows**,
where Apple-only competitors can't follow).

## 2. Market & tailwinds

- **The 47-day cliff (the #1 driver).** CA/Browser Forum Ballot SC-081v3 (Apple-led,
  unanimous): max public TLS lifetime 398 → **200 (Mar 2026)** → 100 (Mar 2027) →
  **47 days + 10-day revalidation (Mar 2029)**. Public-TLS only, but it resets the
  whole industry's default to "short-lived + auto-renewed via ACME" for *every*
  cert lifecycle. Manual PKI becomes untenable. **Hard dates, imminent, inevitable.**
- **Microsoft is migrating Intune off SCEP to ACME + device attestation** — validates
  the protocol direction and the "SCEP is legacy" narrative (shared-secret leak,
  no proof-of-possession, hardware-blind).
- **Machine identity went mainstream:** CyberArk bought Venafi for **~$1.54B** (Oct
  2024), citing a ~$60B TAM; Gartner stood up dedicated Machine IAM coverage.
- **Regulatory pull toward hardware-backed device identity:** NIST 800-63-4 **AAL3**
  (non-exportable hardware keys), CMMC 2.0 / 800-171 §3.5.2 (802.1X EAP-TLS), EO
  14028 / M-22-09 federal Zero Trust. An MDM-issued attested cert checks these boxes.
- Adjacent markets all grow **~12–25% CAGR** (PKI, CLM, machine identity, ZTNA).

**Single most compelling driver right now: the 47-day cliff.** Fleet shipping a
built-in ACME CA lands exactly as "short-lived + ACME everywhere" becomes the
default expectation.

## 3. ICP & personas

Fleet sells to **mid-to-large enterprises (700+ hosts, up to Fortune 1000)** with
cross-platform fleets that outgrew single-vendor tools. Buyers/users are **IT and
security teams jointly** (the pricing table splits every feature by IT vs Security).
Why Fleet: **open-source transparency, GitOps/API-first, cross-platform parity,
tool consolidation.** These are precisely the orgs running 802.1X Wi-Fi/VPN and
Zero-Trust programs — the native consumers of a CA. (Note: this ICP is also why the
**broker/BYO-CA** demand is loud — big regulated orgs already have PKI.)

## 4. Use cases, ranked (jobs to be done)

1. **Wi-Fi / 802.1X EAP-TLS** — kill PSK; per-device, revocable network access. The #1 driver behind the cert demand.
2. **VPN client-cert auth** — same identity, remote access.
3. **Zero Trust / conditional access** — Fleet already ships this (Okta/Entra); device certs are the missing cryptographic anchor.
4. **MDM enrollment identity** — already shipped (built-in ACME, 4.84).
5. **mTLS for fleetd / host identity** — #25951; the TPM attestation work feeds this.
6. **Service/app identity** — longer tail.

## 5. Positioning

A built-in CA amplifies Fleet's existing pitch on every axis:

- **Tool consolidation under one per-device license** — no separate PKI/CA product,
  no Jamf-Connect-style add-on. (Jamf marks "Deploy certificates" = yes, so this is
  parity → a built-in CA + cross-platform attestation extends the lead.)
- **Device trust / Zero Trust / BeyondCorp / device attestation** — Fleet's *stated*
  conditional-access direction; the CA is the missing anchor.
- **Cross-platform parity** — the cert demand is loudest for **Linux + Windows**,
  exactly where Apple-only competitors (Jamf, Kandji, Mosyle) can't follow.
- **Open-source, self-hostable, GitOps-managed** — vs. Smallstep's cloud-locked
  agent and the cloud-only CAs. Air-gap-capable is a real differentiator for the
  regulated ICP.

**The wedge (from the competitive teardown):** every standalone CA is *fleet-blind*
— Smallstep even imports Fleet as an inventory *source*. Fleet owns the fleet,
agent, and ABM. In-process per-device authorization, no webhook, no second agent.

## 6. Pricing & packaging

Fleet tiers: **Free $0 / Premium $7/host/mo / Custom**. "Deploy certificates" and
"Conditional access" are already **Premium**. A built-in CA belongs in **Premium
(likely Custom/Enterprise** given the 700+-host ICP).

External anchors: **Microsoft $2/user/mo** add-on (per-seat comparator), **AWS
$50/CA/mo** short-lived mode (infra comparator), SecureW2/JumpCloud ~$2–3/user/mo
equivalent. **Recommended packaging: bundled into the per-host license — no separate
per-cert or per-CA SKU.** "Your CA is included" undercuts Microsoft's $2 add-on and
AWS's per-CA fees, and reinforces the consolidation pitch. Resist per-cert metering
(Smallstep's per-endpoint-month model is friction the bundled story avoids).

## 7. GTM & timing

- **Ride two windows at once:** the 47-day cliff (Mar 2026+) normalizing ACME, and
  **macOS 27 "Golden Gate"** (fall 2026) driving managed-config / attestation
  interest. Launch the built-in-CA + attestation story into that crossfire.
- **Sequence:** lead with what's shipped (built-in ACME + attestation for Apple),
  extend to Linux/Windows TPM (the cross-platform parity nobody else has), then the
  lifecycle-revocation differentiator (the free version of step-ca's paid feature).
- **Content wedge:** "SCEP is legacy / here's why ACME + attestation," tied to the
  47-day narrative and Microsoft's own migration — Fleet as the cross-platform,
  open-source, self-hostable answer.

## 8. Risks & objections

- **BYO-CA / governance.** The biggest customers (#43376, #44789) may be *unable* to
  use a Fleet-issued root. Mitigation: ship the broker too; let the built-in CA be
  the default, not the only option.
- **CA key custody / trust.** Holding signing keys raises the security bar (HSM/KMS,
  rollover #46214). Regulated buyers will scrutinize this. Air-gap + self-host helps.
- **Revocation infra** (CRL/OCSP) is real work — but it's also the differentiator
  (step-ca charges for it). See the revocation build plan.
- **Support burden** of being a CA (outages = no Wi-Fi/VPN). Short-lived + renewal
  needs to be bulletproof.
- **Cannibalization?** No — it deepens the per-host license and consolidation pitch.

## Recommendation

**Build it — as the wedge, alongside the broker.** Demand is validated and large,
the tailwinds are concrete and timed, the ICP is the exact buyer, and it extends
Fleet's stated Zero-Trust / consolidation / cross-platform positioning into ground
no competitor holds. The honest caveat: the built-in CA is the *moat*, but the
broker/BYO-CA is where the loudest current revenue-bearing demand is — ship both,
default to built-in. Start with the cross-platform attestation + lifecycle
revocation that no incumbent can match.
