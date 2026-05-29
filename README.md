# nanoca

A lightweight enterprise [ACME](https://datatracker.ietf.org/doc/html/rfc8555) Certificate Authority service with [device attestation](https://datatracker.ietf.org/doc/draft-ietf-acme-device-attest/) support. It provides just the HTTP handlers needed to implement ACME, it is intended to be integrated into [nanomdm](https://github.com/micromdm/nanomdm) or another service of your choosing. Storage, signing, authorization, and logging are implemented as pluggable interfaces to integrate into a wide variety of environments.

## Usage

```go
import (
	"github.com/brandonweeks/nanoca"
	"github.com/brandonweeks/nanoca/authorizers/null"
	"github.com/brandonweeks/nanoca/issuers/inprocess"
	"github.com/brandonweeks/nanoca/signers/file"
	"github.com/brandonweeks/nanoca/storage/badger"
)

signer, _ := file.LoadSigner("rootCA.key")
storage, _ := badger.New(badger.Options{InMemory: true})

ca, _ := nanoca.New(
	inprocess.New(signer),
	null.New(),
	storage,
	"https://localhost:8443",
	nanoca.WithPrefix("/acme"),
)
defer ca.Close()

mux := http.NewServeMux()
mux.Handle("/", ca.Handler())
```

---

## Deploying as a standalone server (AdamBaali fork addition)

This fork adds a deployable wrapper in `cmd/mpc-server/` to run nanoca as a 
standalone ACME server for testing Fleet's Apple ACME configuration profile 
flow. See `render.yaml` and `Dockerfile` for Render.com deployment.

### Demo CA (baked into the image)

For convenience this fork ships a throwaway demo CA in `deploy/demo-ca/`, which
the `Dockerfile` bakes into the image. The demo therefore deploys to Render with
**no CA configuration required**. That CA is disposable and public — never use
it for anything real (see `deploy/demo-ca/README.md`). For real use, generate
your own and override it (next sections).

### Generate a root CA

Run the helper script (writes to the gitignored `secrets/` dir by default):

```bash
scripts/gen-ca.sh
```

It generates a PKCS#8 `rootCA.key` and a self-signed `rootCA.crt`, runs a
sanity check, and prints the PEM blocks ready to paste into Render. Subject and
validity are configurable via env vars (`CN`, `O`, `C`, `DAYS`, `BITS`).

> The key must be PKCS#8 (a `PRIVATE KEY` PEM block). The script uses
> `openssl genpkey` for this; the older `openssl genrsa` emits a PKCS#1
> `RSA PRIVATE KEY` that the file signer rejects.

### Supplying the CA: env-var PEM or file path

The CA cert/key can be provided two ways. Raw PEM (`*_PEM`) takes precedence
over the file paths — handy on platforms like Render that expose secrets as
environment variables rather than mounted files.

| Var            | Default                   | Purpose                                       |
|----------------|---------------------------|-----------------------------------------------|
| `PORT`         | `10000`                   | HTTP listen port                              |
| `BASE_URL`     | `https://cert.mpc.ad`     | Public base URL in ACME directory             |
| `CA_CERT_PEM`  | _(unset)_                 | Root CA cert as raw PEM (wins over `CA_CERT`) |
| `CA_KEY_PEM`   | _(unset)_                 | Root CA key as raw PEM (wins over `CA_KEY`)   |
| `CA_CERT`      | `/etc/secrets/rootCA.crt` | Root CA cert file path (PEM)                  |
| `CA_KEY`       | `/etc/secrets/rootCA.key` | Root CA key file path (PKCS#8 PEM)            |
| `BADGER_DIR`   | `/data/badger`            | Persistent storage path                       |
| `TRUST_FORWARDED_PROTO` | `true`           | Trust `X-Forwarded-Proto` from a reverse proxy (see below) |

Precedence: `*_PEM` > `CA_CERT`/`CA_KEY` file paths > the baked-in demo CA
(the `Dockerfile` sets `CA_CERT`/`CA_KEY` to `/etc/nanoca/rootCA.*`). The
`*_PEM` parser accepts PKCS#8, PKCS#1 RSA, and SEC1 EC keys.

On Render, to use your own CA instead of the demo one, add `CA_CERT_PEM` and
`CA_KEY_PEM` in the dashboard with the PEM values printed by `scripts/gen-ca.sh`
(values stay out of the repo).

### Running behind a TLS-terminating proxy (Render, Cloudflare, etc.)

**Symptom.** The ACME directory and `new-nonce` succeed, but every signed POST
(`new-account`, `new-order`, ...) fails with HTTP 400
`urn:ietf:params:acme:error:malformed` and the detail
`JWS verification failed: URL validation failed: HTTPS is required`. On an Apple
device this surfaces as an MDM command error (`ErrorCode 400`, "bad request")
and the certificate never issues.

**Cause.** RFC 8555 §6.4 requires the `url` field in each JWS protected header to
match the request URL, and nanoca enforces that the request arrived over HTTPS by
checking `r.TLS != nil` (`jose.go`). Render and Cloudflare terminate TLS at the
edge and forward to the container over plain HTTP, so `r.TLS` is always `nil`
inside the process even though the client used HTTPS. The check therefore
rejects every authenticated request.

**Fix.** The wrapper trusts the proxy's `X-Forwarded-Proto` header: when it
reports `https`, the `forwardedTLS` middleware in `cmd/mpc-server/main.go`
synthesizes an empty `tls.ConnectionState` so nanoca's check passes. The
forwarded `Host` header already preserves `cert.mpc.ad`, so the reconstructed URL
matches the JWS `url`. This runs in the wrapper rather than the library so no
upstream files change.

`TRUST_FORWARDED_PROTO` defaults to `true`. Set it to `false` only when the
process is directly internet-facing over TLS (no proxy), since a client could
otherwise spoof the header to bypass the HTTPS requirement.

POC use only: uses the `null` authorizer (any attested Apple device gets a 
cert). Production deployments need a real authorizer.
