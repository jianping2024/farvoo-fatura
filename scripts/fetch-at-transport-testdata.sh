#!/usr/bin/env bash
# Refresh AT Documentos de Transporte test materials into
# apps/fiscal-agent/internal/fiscal/at/testdata/transport/
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/apps/fiscal-agent/internal/fiscal/at/testdata/transport"
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36'
mkdir -p "$DEST/certificados/current" "$DEST/certificados/stale-official-zip"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> 1) official certificados.zip (often stale; keep for audit)"
if curl -fsSL -A "$UA" -L --max-time 60 \
  -o "$TMP/certificados.zip" \
  "https://faturas.portaldasfinancas.gov.pt/factemipf_static/java/certificados.zip"; then
  cp "$TMP/certificados.zip" "$DEST/certificados.zip"
  python3 - "$TMP/certificados.zip" "$DEST/certificados/stale-official-zip" <<'PY'
import zipfile, pathlib, sys
zf = zipfile.ZipFile(sys.argv[1])
out = pathlib.Path(sys.argv[2])
out.mkdir(parents=True, exist_ok=True)
for info in zf.infolist():
    if info.is_dir() or info.filename.endswith("/"):
        continue
    raw = info.filename
    try:
        name = raw.encode("cp437").decode("utf-8")
    except Exception:
        try:
            name = raw.encode("cp437").decode("latin-1")
        except Exception:
            name = raw
    base = pathlib.Path(name).name
    if "Chave" in base or "Cifra" in base or "Produ" in base:
        base = "ChavePublicaAT.cer"
    (out / base).write_bytes(zf.read(info))
    print(" stale wrote", base)
PY
else
  echo " official zip failed (non-fatal)"
fi

echo "==> 2) current test certs from community mirror (elsif)"
# Prefer newest TesteWebservices_*.pfx by listing page filenames; fallback known good.
MIRROR=http://downloads.elsif.pt/AT_Certificados
LIST="$TMP/list.html"
curl -fsSL -A "$UA" -L --max-time 30 -o "$LIST" "$MIRROR/" || true
PFX_NAME="$(grep -oE 'TesteWebservices_[0-9]{8}\.pfx' "$LIST" 2>/dev/null | sort -u | tail -1 || true)"
CER_NAME="$(grep -oE 'ChaveCifraPublicaAT[0-9]{8}\.cer' "$LIST" 2>/dev/null | sort -u | tail -1 || true)"
if [[ -z "${PFX_NAME}" ]]; then PFX_NAME=TesteWebservices_20270106.pfx; fi
if [[ -z "${CER_NAME}" ]]; then CER_NAME=ChaveCifraPublicaAT20250415.cer; fi
echo " using $PFX_NAME + $CER_NAME"
curl -fsSL -A "$UA" -L --max-time 60 -o "$DEST/certificados/current/TesteWebservices.pfx" "$MIRROR/$PFX_NAME"
curl -fsSL -A "$UA" -L --max-time 60 -o "$DEST/certificados/current/ChavePublicaAT.cer" "$MIRROR/$CER_NAME"
curl -fsSL -A "$UA" -L --max-time 30 -o "$DEST/certificados/current/SOURCE-elsif-readme.txt" "$MIRROR/_Readme.txt" || true
cp "$DEST/certificados/current/TesteWebservices.pfx" "$DEST/certificados/TesteWebservices.pfx"
cp "$DEST/certificados/current/ChavePublicaAT.cer" "$DEST/certificados/ChavePublicaAT.cer"

echo "==> 3) WSDL"
WSDL="$DEST/documentosTransporte.wsdl"
if curl -fsSL -A "$UA" -L --max-time 30 -o "$WSDL" \
  "https://info.portaldasfinancas.gov.pt/pt/apoio_contribuinte/Documents/documentosTransporte.wsdl" \
  && [[ $(wc -c < "$WSDL") -gt 1000 ]]; then
  echo " official WSDL ok"
else
  curl -fsSL -A "$UA" -L --max-time 30 -o "$WSDL" \
    "https://raw.githubusercontent.com/eSkiSo/codigo_at/master/wsdl/documentosTransporte.wsdl"
  echo " community WSDL mirror used"
fi

echo "==> 4) openssl dates (current)"
PFX="$DEST/certificados/TesteWebservices.pfx"
openssl pkcs12 -in "$PFX" -passin pass:TESTEwebservice -nokeys -clcerts -legacy 2>/dev/null \
  | openssl x509 -noout -dates -subject || true
openssl x509 -in "$DEST/certificados/ChavePublicaAT.cer" -noout -dates -subject || true
echo "done → $DEST"
