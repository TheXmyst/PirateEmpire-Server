#!/usr/bin/env bash
# Compile la version navigateur de SeaDogs (WebAssembly).
#
#   ./client/web/build.sh            -> résultat dans client/web/dist/
#   ./client/web/build.sh /un/dossier -> résultat dans ce dossier
#
# Le dossier obtenu contient tout ce qu'il faut servir sur /play :
#   index.html, wasm_exec.js, game.wasm (+ game.wasm.gz), build-info.json
#   et resources/ (musiques et images téléchargées à la demande par le jeu).
set -euo pipefail

CLIENT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$CLIENT_DIR/web/dist}"
mkdir -p "$(dirname "$OUT")"
OUT="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
TMP="$OUT.tmp"

rm -rf "$TMP"
mkdir -p "$TMP"

echo "Compilation du jeu pour le navigateur..."
(cd "$CLIENT_DIR" && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$TMP/game.wasm" ./cmd)

GOROOT_DIR="$(cd "$CLIENT_DIR" && go env GOROOT)"
if [ -f "$GOROOT_DIR/lib/wasm/wasm_exec.js" ]; then
	cp "$GOROOT_DIR/lib/wasm/wasm_exec.js" "$TMP/"
else
	cp "$GOROOT_DIR/misc/wasm/wasm_exec.js" "$TMP/"
fi
cp "$CLIENT_DIR/web/index.html" "$TMP/"
cp -r "$CLIENT_DIR/internal/game/resources" "$TMP/resources"

# Version compressée, envoyée aux navigateurs qui l'acceptent (tous).
gzip -9 -k "$TMP/game.wasm"

SIZE=$(wc -c < "$TMP/game.wasm" | tr -d ' ')
VERSION=$(sha256sum "$TMP/game.wasm" | cut -c1-12)
printf '{"wasmSize": %s, "version": "%s"}\n' "$SIZE" "$VERSION" > "$TMP/build-info.json"

rm -rf "$OUT"
mv "$TMP" "$OUT"

GZ=$(wc -c < "$OUT/game.wasm.gz" | tr -d ' ')
echo "Version web prête dans $OUT"
echo "  game.wasm : $((SIZE / 1048576)) Mo ($((GZ / 1048576)) Mo à télécharger)"
