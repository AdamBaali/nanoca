# Lifecycle-driven revocation for Fleet's built-in CA — step-by-step build plan

The one paid-only step-ca feature Fleet can give away **and** do better: revocation
that fires automatically off real device state (delete / unenroll / wipe), in
Fleet's own process, no webhook, no SaaS. This breaks it into PR-sized chunks
with concrete files. Planning doc — kept out of the Fleet repo until you start.

## Design decision (settle this first)

Two layers, both driven by Fleet's lifecycle knowledge:

1. **Active revocation via CRL** — the instant-kill path. Fleet knows the moment a
   device is deleted/unenrolled/wiped, so it revokes immediately and publishes a
   CRL that relying parties (802.1X/RADIUS, VPN) consume. This is the
   differentiator; step-ca OSS has no CRL/OCSP at all.
2. **Short-lived certs + renewal-gating** — the eventual-consistency backstop. A
   device that left management simply can't renew. Cheap, and it bounds exposure
   even if a relying party ignores the CRL.

Ship CRL first (chunks 0–3); renewal-gating (chunk 4) is the complement. OCSP
(chunk 5) is optional and only if a customer needs real-time status.

## The data-model wrinkle that shapes everything

`identity_certificates` (shared by SCEP + ACME) has no `host_id`. The links are:

- **Host-identity SCEP**: `host_identity_scep_certificates.host_id` → host. Easy.
- **ACME**: host serial → `acme_enrollments.host_identifier` →
  `acme_accounts` → `acme_orders.issued_certificate_serial` →
  `identity_certificates.serial`. A 4-table traversal, keyed on the hardware
  serial, not a FK.

And `DeleteHost` **hard-deletes** `host_identity_scep_certificates` in the
`deleteHosts` cascade (`server/datastore/mysql/hosts.go:678`). So revocation
cannot live only in the cert rows — a CRL built after a delete would lose the
serial. **We need a durable `certificate_revocations` table** that outlives the
cert row. That requirement drives chunk 0 and chunk 2.

---

## Chunk 0 — Durable revocation record + one "revoke all certs for a host" method

*Goal: a single datastore primitive every later chunk builds on. No behavior
change yet.* **Risk: low. Start here.**

- **Migration** (`make migration name=AddCertificateRevocations`):
  - New table `certificate_revocations`: `serial BIGINT PRIMARY KEY`,
    `revoked_at DATETIME(6)`, `reason TINYINT` (RFC 5280 CRLReason),
    `not_valid_after DATETIME(6)` (so we can prune entries once the cert would
    have expired anyway), `ca_key_id VARBINARY` (which CA key signed it — matters
    for rollover), timestamps.
  - Add `revoked_at DATETIME(6) NULL` + `revocation_reason TINYINT NULL` to
    `identity_certificates` (today it's a bare boolean) for audit + CRL time.
- **Datastore** (`server/datastore/mysql/`): `RevokeHostIdentityCertificates(ctx, host *fleet.Host, reason fleet.CRLReason) (revoked int, err error)`:
  - SCEP certs: `UPDATE host_identity_scep_certificates SET revoked=1 ... WHERE host_id=?`.
  - ACME certs: resolve via the serial traversal above, set
    `identity_certificates.revoked=1, revoked_at=NOW()` and mark
    `acme_enrollments.revoked`.
  - **Insert each revoked serial into `certificate_revocations`** (the durable
    record) within the same tx.
- **Interface + mocks**: add to `server/fleet/datastore.go`; regenerate mocks.
  Per CLAUDE.md, **run `go test ./server/service/`** after — uninitialized mocks
  crash unrelated tests.
- **Tests** (`MYSQL_TEST=1`): revoke a host with both SCEP + ACME certs; assert
  rows flagged and `certificate_revocations` populated; idempotent on re-call.
- Precedent to copy: `server/datastore/mysql/conditional_access_scep.go:57`.

## Chunk 1 — Fire revocation on the trust-boundary lifecycle events

*Goal: wire chunk 0 into the events where a device leaves trust.* **Risk: medium
(touches hot paths).**

- **Which events revoke** (decision baked in):
  - **Delete host** — yes. Revoke *before* the hard-delete cascade so serials land
    in `certificate_revocations` first. `server/service/hosts.go:1106 DeleteHost`,
    `:593 DeleteHosts`.
  - **MDM turn-off / unenroll** — yes. `server/service/mdm.go TurnOffMDM`
    (datastore `apple_mdm.go:2035 MDMTurnOff`), and the Android unenroll path.
  - **Wipe** — yes. `ee/server/service/hosts.go:261 WipeHost`.
  - **Lock** — no (temporary). **Team transfer** — no by default (rotate, don't
    revoke; revisit if teams imply trust boundaries).
- **Where to hook**: extend the central `server/mdm/lifecycle/lifecycle.go`
  (already invoked on delete + turn-off) with a `HostActionRevokeIdentity`, rather
  than sprinkling calls. Non-MDM delete path calls the datastore method directly.
- **Audit**: new `ActivityTypeRevokedHostIdentity` in
  `server/fleet/activities.go` (+ document in
  `docs/Contributing/reference/audit-logs.md`), emitted via `svc.NewActivity`.
- **Tests**: service tests asserting revocation is invoked for delete/unenroll/
  wipe and *not* for lock; activity emitted.

## Chunk 2 — Build & sign the CRL on a schedule

*Goal: turn revocation records into a signed CRL.* **Risk: low (additive, cron).**

- **CRL builder**: read non-expired rows from `certificate_revocations`, load the
  CA keypair via `assets.CAKeyPair(ctx, ds)` (`server/mdm/assets/assets.go:19` —
  returns a `crypto.Signer` + leaf), call `x509.CreateRevocationList`. Store DER +
  `this_update`/`next_update` in a `certificate_revocation_list` table (or cache).
- **CA rollover**: sign per active CA key using
  `assets.CACertsAndKeyForDecryption`; emit a CRL per `ca_key_id`.
- **Cron**: register `build_crl` in `cmd/fleet/cron.go` exactly like
  `revoke_old_conditional_access_certs` (`cron.go:1517`) — `schedule.WithJob`,
  Locker, stats. Short interval (e.g. 5 min) or trigger-on-revoke.
- **Tests**: revoked serials appear in the CRL; CRL signature verifies against the
  CA cert; `next_update` set; expired entries pruned.

## Chunk 3 — Distribute the CRL + stamp new certs with the CDP

*Goal: relying parties can fetch it.* **Risk: low–medium (cert template change).**

- **Endpoint**: unauthenticated `GET /api/mdm/crl/{ca_key_id}` returning cached
  DER, with `ETag`/`Last-Modified` + rate-limit. Register in the unauthenticated
  block of `server/service/handler.go` (mirror the SCEP/ACME registration).
- **CRL Distribution Point**: add `CRLDistributionPoints` to the signer template
  so newly issued certs advertise the URL (`server/mdm/scep/depot/signer.go`
  `Signx509CSR`, and the ACME signing path). Only affects new certs — existing
  ones are covered by short-lived + renewal (chunk 4).
- **Tests**: endpoint serves a valid CRL; a freshly issued cert carries the CDP.

## Chunk 4 — Short-lived certs + renewal-gating (the backstop)

*Goal: a device that left management can't renew.* **Risk: medium (agent + policy).**

- Make the built-in-CA cert lifetime configurable; default the built-in CA to
  shorter validity with automated ACME renewal.
- **Gate renewal on host standing**: in the ACME order / SCEP renewal path
  (`ee/server/service/hostidentity/scep.go renewalMiddleware`, and the ACME
  `IsDeviceEnrolled` authorizer we're adding), refuse renewal if the host is
  deleted/unenrolled/revoked. This is the in-process equivalent of step-ca's
  authorizing webhook — but backed by real Fleet inventory.
- **Tests**: revoked/deleted host is denied renewal; healthy host renews.

## Chunk 5 — (Optional) OCSP + admin surface

*Goal: real-time status + operator control.* **Risk: medium.**

- **OCSP responder**: `golang.org/x/crypto/ocsp` `CreateResponse`, signed by the
  CA (or a delegated OCSP signer cert), endpoint in `handler.go`. Only if a
  customer needs sub-CRL-interval freshness.
- **UI/API**: show revoked state + a manual "Revoke certificate" action on host
  details (the cert table from #42827 already exists), `fleetctl` + REST for
  manual revoke with reason. Premium-gate per Fleet tier conventions.

---

## Cross-cutting

- **Migrations**: `make migration name=...`; if your migration timestamp predates
  one merged to main, use the bump-migration flow.
- **Mocks**: after any `Datastore` interface change, regenerate mocks and run
  `go test ./server/service/` (CLAUDE.md).
- **Tiering**: the built-in CA is almost certainly Premium — follow the
  `!isPremiumTier` / license-check patterns at the service layer.
- **Reuse, don't reinvent**: `RevokeOldConditionalAccessCerts` + its cron is the
  template for the datastore method *and* the schedule registration. Mirror it.
- **CA key custody / rollover** is the one genuinely hard part — the CRL must be
  signed by the right key, and `ca_key_id` threads through chunks 0/2/3 to handle
  it. Don't skip it.

## Start here (first PR)

**Chunk 0**, sliced even thinner to de-risk: the migration +
`RevokeHostIdentityCertificates` + tests, with no callers yet. It's pure additive
datastore work, fully testable under `MYSQL_TEST=1`, touches no hot paths, and
every later chunk depends on it. Land that, then chunk 1 wires it to lifecycle
events one at a time (delete → unenroll → wipe), each its own small PR.
