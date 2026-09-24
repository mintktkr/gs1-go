#!/usr/bin/env bash
# Renders GS1 DataMatrix symbols with the gs1 CLI and decodes them with an
# independent decoder (zxing-cpp, run through uv). Checks that every symbol
# decodes as GS1 DataMatrix (]d2) back to the same element string.
#
# Needs Go and uv; nothing is added to go.mod.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/gs1" "$root/cmd/gs1"

# Long internal fields (AIs 91-99, up to 90 characters each) appended one
# at a time to walk through the symbol sizes up to 144x144.
filler=$(printf 'ABCDEFGHIJ%.0s' $(seq 1 9))
cases=(
	"(01)04150000021126"
	"(01)04150000021126(17)250630(10)ABC123"
	"(01)09506000134352(17)201231(10)4512(21)12345678901234"
	"(00)106141411234567897"
	"(01)04150000021126(10)LOT-42/X(21)SN.0001"
)
extra=""
for i in 91 92 93 94 95 96 97 98 99 91 92 93 94 95 96 97 98; do
	extra+="($i)$filler"
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
		printf '%s\t%s\n' "$tmp/$n.png" "$c" >> "$tmp/list"
		n=$((n + 1))
	done
done

uv run --quiet --with zxing-cpp --with pillow python - "$tmp/list" <<'EOF'
import sys, zxingcpp
from PIL import Image
fail = 0
for line in open(sys.argv[1]):
    path, want = line.rstrip("\n").split("\t")
    img = Image.open(path)
    got = zxingcpp.read_barcodes(img, formats=zxingcpp.BarcodeFormat.DataMatrix)
    size = "%dx%d" % (img.height // 4 - 4, img.width // 4 - 4)
    if len(got) != 1 or got[0].symbology_identifier != "]d2" or got[0].text != want:
        fail += 1
        print("FAIL", size, want, [(g.symbology_identifier, g.text) for g in got])
    else:
        print("ok  ", size.ljust(8), want[:70])
print("%d symbols, %d failed" % (sum(1 for _ in open(sys.argv[1])), fail))
sys.exit(1 if fail else 0)
EOF
