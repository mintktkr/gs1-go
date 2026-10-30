package symbol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestDMECCGoldenVector checks the worked example of ISO/IEC 16022 for the
// 10x10 symbol: data codewords 142 164 186 with ECC codewords 114 25 5 88 102.
func TestDMECCGoldenVector(t *testing.T) {
	got := dmECC([]byte{142, 164, 186}, dmSizes[0])
	want := []byte{142, 164, 186, 114, 25, 5, 88, 102}
	if !bytes.Equal(got, want) {
		t.Errorf("dmECC([142 164 186], 10x10) = %v, want %v", got, want)
	}
}

// TestDMECCInterleavedBlocks checks the interleaving of a symbol with more
// than one Reed-Solomon block: 52x52 carries 204 data codewords and 84 ECC
// codewords in 2 blocks. The expected ECC codewords were cross-checked
// against an independent ECC 200 implementation, so a wrong block split or
// interleave order fails here even though it would still satisfy the
// syndromes of TestDMECCSyndromes.
func TestDMECCInterleavedBlocks(t *testing.T) {
	want, err := hex.DecodeString(
		"b2533b2929905b21fa58cc207b671286f424d9cfba8ce159ec4fd527a49666d3" +
			"f0beaaa40e28701027d8ad57f2992269918bdbe6e8e60cebf1228a0ee58bb67c" +
			"6f58475588a4ae855ddad9e5056c031a5b6249fb")
	if err != nil {
		t.Fatal(err)
	}
	s := dmTestSize(t, 52, 52)
	if len(want) != s.ECCCW {
		t.Fatalf("test vector has %d ECC codewords, want %d", len(want), s.ECCCW)
	}
	data := dmTestCodewords(s.DataCW)
	got := dmECC(data, s)
	if !bytes.Equal(got[s.DataCW:], want) {
		t.Errorf("52x52 ECC = %x, want %x", got[s.DataCW:], want)
	}
}

// TestDMECCKeepsInput checks that dmECC does not write to data.
func TestDMECCKeepsInput(t *testing.T) {
	for _, s := range dmSizes {
		data := dmTestCodewords(s.DataCW)
		orig := append([]byte(nil), data...)
		dmECC(data, s)
		if !bytes.Equal(data, orig) {
			t.Errorf("%dx%d: dmECC changed its input", s.Rows, s.Cols)
		}
	}
}

// dmTestSize returns the dmSizes entry with the given symbol size.
func dmTestSize(t *testing.T, rows, cols int) dmSize {
	t.Helper()
	for _, s := range dmSizes {
		if s.Rows == rows && s.Cols == cols {
			return s
		}
	}
	t.Fatalf("no dmSizes entry for %dx%d", rows, cols)
	return dmSize{}
}

// dmTestCodewords returns n deterministic codewords for property tests.
func dmTestCodewords(n int) []byte {
	b := make([]byte, n)
	x := uint32(12345)
	for i := range b {
		x = x*1103515245 + 12345
		b[i] = byte(x >> 16)
	}
	return b
}

// TestDMECCPos pins the 144x144 layout: the ECC of the two shorter blocks
// (8 and 9) comes first, as zint and zxing expect.
func TestDMECCPos(t *testing.T) {
	s := dmTestSize(t, 144, 144)
	tests := []struct{ b, j, want int }{
		{8, 0, 1558}, {9, 0, 1559}, {0, 0, 1560}, {7, 0, 1567},
		{8, 1, 1568}, {7, 61, 2177},
	}
	for _, tt := range tests {
		if got := dmECCPos(s, tt.b, tt.j); got != tt.want {
			t.Errorf("dmECCPos(144x144, b=%d, j=%d) = %d, want %d", tt.b, tt.j, got, tt.want)
		}
	}
	even := dmTestSize(t, 52, 52)
	if got := dmECCPos(even, 1, 3); got != 204+3*2+1 {
		t.Errorf("dmECCPos(52x52, b=1, j=3) = %d, want %d", got, 204+3*2+1)
	}
}
