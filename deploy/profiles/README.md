# Test configuration profiles

Two macOS configuration profiles that enroll an Apple device identity certificate
from the live test server at `https://cert.mpc.ad`. They carry no secrets and
point at the public test CA, so they are safe to copy and share. Apply them
through Fleet (Controls, OS settings, Custom settings) or another MDM.

## Profiles

- `nanoca-root-ca-trust.mobileconfig` (`com.apple.security.root`): installs the
  test root CA so the issued certificate chains validate. The embedded
  certificate is the public demo CA from `deploy/demo-ca/rootCA.crt`.
- `nanoca-acme.mobileconfig` (`com.apple.security.acme`): requests an identity
  certificate. Hardware bound, Secure Enclave attestation, EC P-384. The client
  identifier and subject common name come from `$FLEET_VAR_HOST_HARDWARE_SERIAL`,
  a Fleet profile variable. With another MDM, replace it with that MDM's per-host
  serial variable or a static value.

Install the trust profile before, or together with, the ACME profile.

## Verify

Confirm the server is reachable:

```bash
curl -s https://cert.mpc.ad/acme/directory
```

Confirm the root CA installed on the Mac:

```bash
security find-certificate -c "MPC Demo Root CA" /Library/Keychains/System.keychain
```

Inspect the issued identity certificate (replace with the keychain label or
export the cert first):

```bash
openssl x509 -in issued.pem -noout -subject -issuer -dates -ext extendedKeyUsage
```

A successful run produces a certificate like this. The common name is the host
serial and the issuer is the test CA:

```
subject=CN = <host serial>
issuer=CN = MPC Demo Root CA, O = Mountain Path Consulting, C = AD
X509v3 Extended Key Usage: TLS Web Client Authentication
```

## Limitations

POC only. The server uses the `null` authorizer, so any attested device gets a
certificate. It is not linked to Apple Business Manager tokens yet, so ABM
organization membership is not checked. The signing CA is a throwaway demo CA
whose private key is public. Do not rely on these certificates for production
identity.
