#!/usr/bin/env bash
# Builds the demo into this directory. Serve it with any static file server,
# for example: python3 -m http.server -d examples/datamatrix-wasm 8080
set -euo pipefail
dir=$(cd "$(dirname "$0")" && pwd)
cd "$dir/../.."
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o "$dir/demo.wasm" ./examples/datamatrix-wasm/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$dir/wasm_exec.js"
echo "built $dir/demo.wasm ($(du -h "$dir/demo.wasm" | cut -f1))"
