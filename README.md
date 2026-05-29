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

Configuration via env vars:

| Var          | Default                   | Purpose                                  |
|--------------|---------------------------|------------------------------------------|
| `PORT`       | `10000`                   | HTTP listen port                         |
| `BASE_URL`   | `https://cert.mpc.ad`     | Public base URL in ACME directory        |
| `CA_CERT`    | `/etc/secrets/rootCA.crt` | Root CA cert (PEM)                       |
| `CA_KEY`     | `/etc/secrets/rootCA.key` | Root CA key (PEM)                        |
| `BADGER_DIR` | `/data/badger`            | Persistent storage path                  |

POC use only: uses the `null` authorizer (any attested Apple device gets a 
cert). Production deployments need a real authorizer.
