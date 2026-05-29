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

## Standalone server (fork addition)

`cmd/mpc-server/` wraps nanoca as a standalone ACME server. `Dockerfile` and
`render.yaml` deploy it to Render. A live instance runs at `https://cert.mpc.ad`.

### Deploy

The image bakes in a demo CA (`deploy/demo-ca/`), so it deploys with no CA
config. The demo CA is disposable and its key is public. Never use it for real
certificates.

To use your own CA, generate one and pass it in:

```bash
scripts/gen-ca.sh   # writes PKCS#8 rootCA.key + rootCA.crt to secrets/, prints PEM for Render
```

The key must be PKCS#8 (a `PRIVATE KEY` block). `gen-ca.sh` uses
`openssl genpkey`; the older `openssl genrsa` emits PKCS#1, which the file signer
rejects. Set the subject with `CN`, `O`, `C`, `DAYS`, `BITS`. On Render, paste
the printed PEM into `CA_CERT_PEM` and `CA_KEY_PEM`.

### Configuration

| Var | Default | Purpose |
|-----|---------|---------|
| `PORT` | `10000` | Listen port |
| `BASE_URL` | `https://cert.mpc.ad` | Base URL in the ACME directory |
| `CA_CERT_PEM` | _(unset)_ | Root CA cert as PEM |
| `CA_KEY_PEM` | _(unset)_ | Root CA key as PEM |
| `CA_CERT` | `/etc/secrets/rootCA.crt` | Root CA cert file |
| `CA_KEY` | `/etc/secrets/rootCA.key` | Root CA key file (PKCS#8) |
| `BADGER_DIR` | `/data/badger` | Storage path |
| `TRUST_FORWARDED_PROTO` | `true` | Trust `X-Forwarded-Proto` from a proxy |

CA precedence: `*_PEM` > file paths > baked-in demo CA. The PEM parser accepts
PKCS#8, PKCS#1 RSA, and SEC1 EC keys.

### Behind a TLS-terminating proxy

Render and Cloudflare terminate TLS and forward over plain HTTP, so `r.TLS` is
nil and nanoca's RFC 8555 URL check rejects every signed POST with
"HTTPS is required" (HTTP 400). The wrapper reads `X-Forwarded-Proto` and
restores the HTTPS state. Set `TRUST_FORWARDED_PROTO=false` only when the process
terminates TLS itself.

### Test profiles

`deploy/profiles/` holds two macOS profiles that run the attestation flow against
`https://cert.mpc.ad`. They carry no secrets and point at the public test CA.
Apply them through Fleet or another MDM.

| Profile | Payload | Purpose |
|---------|---------|---------|
| `nanoca-root-ca-trust.mobileconfig` | `com.apple.security.root` | Trusts the test root CA so issued certs validate |
| `nanoca-acme.mobileconfig` | `com.apple.security.acme` | Requests an identity cert: hardware bound, Secure Enclave attestation, EC P-384 |

The ACME profile sets the client identifier and subject CN from
`$FLEET_VAR_HOST_HARDWARE_SERIAL`, a Fleet variable. With another MDM, use its
per-host serial variable or a static value. See `deploy/profiles/README.md` to
verify issuance.

### Limits

POC only. The server uses the `null` authorizer, so any attested device gets a
cert. It is not linked to Apple Business Manager tokens, so ABM membership is not
checked. The demo CA key is public. Use a real authorizer and a real CA for
production.
