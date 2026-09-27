#!/usr/bin/env bash
# Renders Data Matrix symbols and decodes them with an independent decoder
# (zxing-cpp, run through uv):
#
#   - GS1 element strings through the gs1 CLI, which must decode as GS1
#     DataMatrix (]d2) back to the same element string;
#   - plain data filling every one of the 30 symbol sizes, which must decode
#     as plain Data Matrix (]d1) back to the same data. A valid GS1 element
#     string cannot fill 144x144 with ASCII encodation, since no AI may
#     repeat, so the plain symbols are what covers the largest sizes.
#
# Needs Go and uv; nothing is added to go.mod.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/gs1" "$root/cmd/gs1"

# A throwaway module outside the repository renders plain Data Matrix.
mkdir "$tmp/plain"
cat > "$tmp/plain/go.mod" <<EOF
module plain
go 1.23
require github.com/galenzo17/gs1-go v0.0.0
replace github.com/galenzo17/gs1-go => $root
EOF
cat > "$tmp/plain/main.go" <<'EOF'
package main

import (
	"os"
	"strings"

	"github.com/galenzo17/gs1-go/symbol"
)

// usage: plain DATA square|rect OUT.png
func main() {
	opts := symbol.DataMatrixOptions{Rectangular: os.Args[2] == "rect"}
	m, err := symbol.DataMatrix(os.Args[1], opts)
	if err == nil {
		var f *os.File
		if f, err = os.Create(os.Args[3]); err == nil {
			err = m.PNG(f, 4, 2)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil {
		os.Stderr.WriteString(strings.TrimSpace(err.Error()) + "\n")
		os.Exit(1)
	}
}
EOF
(cd "$tmp/plain" && go build -o "$tmp/plainbin" .)

# Distinct AIs with long alphanumeric data, appended one at a time to walk
# through the symbol sizes a GS1 element string can reach.
f90=$(printf 'ABCDEFGHIJ%.0s' $(seq 1 9))
long=(91:$f90 92:$f90 93:$f90 94:$f90 95:$f90 96:$f90 97:$f90 98:$f90 99:$f90
	90:${f90:0:30} 240:${f90:0:30} 241:${f90:0:30} 401:${f90:0:30} 8004:${f90:0:30}
	710:${f90:0:20} 711:${f90:0:20} 712:${f90:0:20} 713:${f90:0:20} 714:${f90:0:20})
cases=(
	"(01)04150000021126"
	"(01)04150000021126(17)250630(10)ABC123"
	"(01)09506000134352(17)201231(10)4512(21)12345678901234"
	"(00)106141411234567897"
	"(01)04150000021126(10)LOT-42/X(21)SN.0001"
	"(402)12345678901234560(10)LOT1"
)
extra=""
for kv in "${long[@]}"; do
	extra+="(${kv%%:*})${kv#*:}"
	cases+=("(01)04150000021126$extra")
done

n=0
for c in "${cases[@]}"; do
	for shape in "" "-rect"; do
		# Rectangular symbols top out at 49 codewords; skip what cannot fit.
		# shellcheck disable=SC2086
		if ! err=$("$tmp/gs1" datamatrix $shape -scale 4 -quiet 2 -o "$tmp/$n.png" "$c" 2>&1); then
			echo "skip ${shape:-square} ${#c} chars: $err"
			continue
		fi
		printf '%s\t]d2\t%s\n' "$tmp/$n.png" "$c" >> "$tmp/list"
		n=$((n + 1))
	done
done

# Data codewords of the 24 square and 6 rectangular sizes. Letters cost one
# codeword each, so data of exactly that length selects that size.
caps=(3 5 8 12 18 22 30 36 44 62 86 114 144 174 204 280 368 456 576 696 816 1050 1304 1558)
rects=(5 10 16 22 32 49)
letters=$(printf 'ABCDEFGHIJKLMNOPQRSTUVWXYZ%.0s' $(seq 1 60))
for shape in square rect; do
	if [ "$shape" = square ]; then sizes=("${caps[@]}"); else sizes=("${rects[@]}"); fi
	for cap in "${sizes[@]}"; do
		d=${letters:0:$cap}
		"$tmp/plainbin" "$d" "$shape" "$tmp/$n.png"
		printf '%s\t]d1\t%s\n' "$tmp/$n.png" "$d" >> "$tmp/list"
		n=$((n + 1))
	done
done

uv run --quiet --with zxing-cpp --with pillow python - "$tmp/list" <<'EOF'
import sys, zxingcpp
from PIL import Image
fail = 0
for line in open(sys.argv[1]):
    path, sym, want = line.rstrip("\n").split("\t")
    img = Image.open(path)
    got = zxingcpp.read_barcodes(img, formats=zxingcpp.BarcodeFormat.DataMatrix)
    size = "%dx%d" % (img.height // 4 - 4, img.width // 4 - 4)
    if len(got) != 1 or got[0].symbology_identifier != sym or got[0].text != want:
        fail += 1
        print("FAIL", size, sym, want[:60], [(g.symbology_identifier, g.text[:60]) for g in got])
    else:
        print("ok  ", size.ljust(8), sym, want[:60])
print("%d symbols, %d failed" % (sum(1 for _ in open(sys.argv[1])), fail))
sys.exit(1 if fail else 0)
EOF
