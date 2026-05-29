# Demo CA — throwaway, committed on purpose

`rootCA.crt` and `rootCA.key` here are a **disposable demo certificate
authority**, committed to the repo so the `mpc-server` demo deploys with zero
extra configuration (the `Dockerfile` bakes them into the image).

**This is not a secret and must never be used for anything real.** Anyone with
this repo has the private key.

For real use, generate your own CA locally and keep it out of the repo:

```bash
scripts/gen-ca.sh          # writes to ./secrets/ (gitignored)
```

Then override at runtime via `CA_CERT_PEM` / `CA_KEY_PEM` (raw PEM, e.g. pasted
into Render env vars) or `CA_CERT` / `CA_KEY` (file paths). Any of those take
precedence over the baked-in demo CA.
