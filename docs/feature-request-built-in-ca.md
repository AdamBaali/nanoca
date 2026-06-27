<!--
Feature request draft for fleetdm/fleet — paste into the 💡 Feature request
template (labels: :product). Problem-first, per Fleet's FR conventions.
Kept in the nanoca fork for now; not filed.
-->

# 💡 Feature request: Built-in certificate authority

## The problem

Customers want Fleet to give their devices certificates — for Wi‑Fi (802.1X), VPN, and Zero Trust access — without standing up and babysitting their own PKI.

Today they can't, unless they already run a CA (NDES, DigiCert, AD CS) and wire it in by hand. Teams without a CA — or tired of NDES breaking — have no path at all. Teams that do have one still glue it to Fleet manually.

This is one of our most-requested areas: **~101 cert-related issues, 43 named customers, 22 customer promises.** The two loudest:

- **#21096** — deploy & renew certs on Linux — **19 customers**
- **#13420** — connect end users to Wi‑Fi/VPN with certs — **12 customers**

The job underneath almost all of it: **per-device network access (802.1X EAP‑TLS) and Zero Trust**, loudest on **Linux and Windows** — where Apple-only tools can't help.

## Ideal workflow

> **As an** IT or security admin,
> **I want** Fleet to issue and renew device certificates from a built-in CA (and broker to my own CA when I have one),
> **so that** my devices get Wi‑Fi / VPN / Zero‑Trust identity without me running PKI.

## Why now

- **Short-lived certs are becoming the norm.** The CA/Browser Forum is cutting max public TLS lifetime to **47 days by 2029** (200 days from **March 2026**). Manual cert management is ending; automated ACME issuance is becoming the default expectation for every certificate, not just public TLS.
- **Microsoft is moving Intune off SCEP onto ACME + device attestation.** SCEP's shared-secret model (a leaked challenge lets any machine get a corporate cert) is now the legacy, insecure path.
- **macOS 27 ("Golden Gate," this fall)** drives managed-config and attestation interest — a window to ship into.

## Why this is Fleet's to win

Every standalone CA (Smallstep, EJBCA, …) is **fleet-blind**: it can prove a device's hardware is genuine, but not that the device is *ours*. Smallstep even imports Fleet as a device-inventory *source*.

Fleet already owns the fleet, the agent, and ABM — so we can decide "is this device allowed a cert?" **in-process**: no webhook, no second agent, no SaaS. And we can do it **cross-platform** (Apple Secure Enclave + Windows/Linux TPM) and **fully self-hosted / air-gapped** — a combination no incumbent ships. Jamf is Apple-only; Microsoft is Windows + cloud-only + SCEP. It reinforces the pitch we already make: one per-device license, device trust / Zero Trust, cross-platform, open-source.

## We've already started

Fleet **4.84** shipped a built-in ACME server with Apple hardware attestation for DEP Macs (**#15611**), plus the Hydrant integration and Request-a-Certificate API (**#28974**). This request is about **productizing that into a general "Fleet CA"**: more device types, the Wi‑Fi/VPN/identity use cases, and automatic revocation.

## Two modes, not one (the honest read)

- **Built-in CA** — the default for the ~80% who don't want to run PKI. Our differentiator.
- **Broker / bring-your-own-CA** — for large, regulated customers who must keep their own root (**#43376, #44789**). Partly built already.

Default to built-in; keep BYO for those who need it. They're complementary.

## Proposed first slices (for the story to spec)

1. Extend the built-in ACME CA beyond Apple DEP Macs → **Linux/Windows TPM attestation**.
2. **Automatic revocation driven by device lifecycle** (delete / unenroll / wipe) — the one thing Smallstep gates behind its paid tier that we can give away *and* do better, because we already know when a device leaves the fleet.

## Open questions for product

- **Tier:** Premium (consistent with "Deploy certificates" and "Conditional access").
- **Trust model:** per-tenant root vs. Fleet root + per-tenant intermediates; key custody (DB-encrypted today vs. HSM/KMS).
- **Revocation posture:** short-lived + ACME renewal vs. CRL/OCSP.
