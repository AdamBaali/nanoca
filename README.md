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

On Render: `render.yaml` declares `CA_CERT_PEM` and `CA_KEY_PEM` with
`sync: false`, so you paste the PEM values into the dashboard (they're not
committed). The `*_PEM` parser accepts PKCS#8, PKCS#1 RSA, and SEC1 EC keys.

POC use only: uses the `null` authorizer (any attested Apple device gets a 
cert). Production deployments need a real authorizer.
