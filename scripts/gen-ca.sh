#!/usr/bin/env bash
#
# gen-ca.sh — generate a self-signed root CA for the nanoca mpc-server demo.
#
# Produces a PKCS#8 private key (rootCA.key) and a self-signed certificate
# (rootCA.crt). The key MUST be PKCS#8 ("PRIVATE KEY" PEM block) — that is what
# the nanoca file signer expects, and what `openssl genpkey` emits (unlike the
# older `openssl genrsa`, which emits a PKCS#1 "RSA PRIVATE KEY").
#
# Usage:
#   scripts/gen-ca.sh [output-dir]
#
# Configurable via environment variables (with defaults):
#   OUT_DIR   output directory               (default: secrets, or $1 if given)
#   CN        certificate Common Name        (default: "MPC Test Root CA")
#   O         Organization                   (default: "Mountain Path Consulting")
#   C         Country code                    (default: "AD")
#   DAYS      validity in days               (default: 3650)
#   BITS      RSA key size in bits           (default: 4096)
#
# After running, paste the printed CA_CERT_PEM / CA_KEY_PEM values into your
# Render service's environment variables (Render exposes secrets as env vars,
# not mounted files, so the mpc-server reads them from CA_CERT_PEM/CA_KEY_PEM).

set -euo pipefail

OUT_DIR="${1:-${OUT_DIR:-secrets}}"
CN="${CN:-MPC Test Root CA}"
O="${O:-Mountain Path Consulting}"
C="${C:-AD}"
DAYS="${DAYS:-3650}"
BITS="${BITS:-4096}"

KEY="${OUT_DIR}/rootCA.key"
CRT="${OUT_DIR}/rootCA.crt"

mkdir -p "${OUT_DIR}"

if [[ -e "${KEY}" || -e "${CRT}" ]]; then
  echo "Refusing to overwrite existing ${KEY} or ${CRT}." >&2
  echo "Delete them first (or choose a different OUT_DIR) to regenerate." >&2
  exit 1
fi

echo "Generating ${BITS}-bit PKCS#8 RSA key -> ${KEY}"
openssl genpkey -algorithm RSA -pkeyopt "rsa_keygen_bits:${BITS}" -out "${KEY}"
chmod 600 "${KEY}"

echo "Generating self-signed cert (${DAYS} days) -> ${CRT}"
openssl req -x509 -new -nodes -key "${KEY}" -sha256 -days "${DAYS}" \
  -subj "/CN=${CN}/O=${O}/C=${C}" \
  -out "${CRT}"

echo
echo "=== sanity check ==="
openssl x509 -in "${CRT}" -noout -subject -dates -issuer

cat <<EOF

=== Render environment variables ===
Set these in your Render service (Environment tab). Both accept multi-line PEM.

--- CA_CERT_PEM ---
EOF
cat "${CRT}"
cat <<EOF

--- CA_KEY_PEM ---
EOF
cat "${KEY}"
cat <<EOF

Done. Files written to ${OUT_DIR}/ (gitignored by default).
Back them up somewhere safe — this is a demo CA but regenerating it invalidates
every certificate it has issued.
EOF
