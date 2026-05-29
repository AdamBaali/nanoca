# Test configuration profiles

Two macOS profiles that enroll an Apple device identity certificate from the live
test server at `https://cert.mpc.ad`. They carry no secrets and point at the
public test CA. Apply them through Fleet (Controls, OS settings, Custom settings)
or another MDM.

## Profiles

- `nanoca-root-ca-trust.mobileconfig` (`com.apple.security.root`): trusts the test
  root CA so issued certs validate. Embeds the public demo CA from
  `deploy/demo-ca/rootCA.crt`.
- `nanoca-acme.mobileconfig` (`com.apple.security.acme`): requests an identity
  cert. Hardware bound, Secure Enclave attestation, EC P-384. Client identifier
  and subject CN come from `$FLEET_VAR_HOST_HARDWARE_SERIAL`, a Fleet variable.
  With another MDM, use its per-host serial variable or a static value.

Install the trust profile before, or with, the ACME profile.

## Verify

```bash
# Server reachable
curl -s https://cert.mpc.ad/acme/directory

# Root CA installed on the Mac
security find-certificate -c "MPC Demo Root CA" /Library/Keychains/System.keychain

# Issued cert (export it first, or pass the keychain label)
openssl x509 -in issued.pem -noout -subject -issuer -ext extendedKeyUsage
```

A successful run issues a cert whose CN is the host serial, signed by the test CA:

```
subject=CN = <host serial>
issuer=CN = MPC Demo Root CA, O = Mountain Path Consulting, C = AD
X509v3 Extended Key Usage: TLS Web Client Authentication
```

## Limits

POC only. The `null` authorizer issues to any attested device. It is not linked to
Apple Business Manager tokens, so ABM membership is not checked. The demo CA key
is public. Do not rely on these certs for production identity.
