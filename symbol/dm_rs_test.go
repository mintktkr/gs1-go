package symbol

import (
	"bytes"
	"encoding/hex"
	"fmt"
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

// TestDMECCSyndromes checks, for every symbol size and block, that the data
// codewords of a block followed by its ECC codewords form a Reed-Solomon
// codeword: the block polynomial is zero at alpha^1..alpha^n.
func TestDMECCSyndromes(t *testing.T) {
	for _, s := range dmSizes {
		t.Run(fmt.Sprintf("%dx%d", s.Rows, s.Cols), func(t *testing.T) {
			data := dmTestCodewords(s.DataCW)
			got := dmECC(data, s)
			if len(got) != s.DataCW+s.ECCCW {
				t.Fatalf("len(dmECC) = %d, want %d", len(got), s.DataCW+s.ECCCW)
			}
			if !bytes.Equal(got[:s.DataCW], data) {
				t.Fatalf("data prefix of dmECC differs from input")
			}
			n := s.ECCCW / s.Blocks
			for b := 0; b < s.Blocks; b++ {
				block := make([]byte, 0, s.DataCW/s.Blocks+1+n)
				for i := b; i < s.DataCW; i += s.Blocks {
					block = append(block, got[i])
				}
				for j := 0; j < n; j++ {
					block = append(block, got[dmECCPos(s, b, j)])
				}
				for r, syn := range dmSyndromes(block, n) {
					if syn != 0 {
						t.Errorf("block %d: syndrome at alpha^%d is %d, want 0", b, r+1, syn)
					}
				}
			}
		})
	}
}

// TestDMGenLog checks every cached generator: it must have degree n, no zero
// coefficients (log form cannot store a zero), and the roots alpha^1..alpha^n,
// which fixes g(x) = (x-a^1)...(x-a^n).
func TestDMGenLog(t *testing.T) {
	seen := make(map[int]bool)
	for _, s := range dmSizes {
		n := s.ECCCW / s.Blocks
		if n > dmMaxECC {
			t.Fatalf("%dx%d needs %d ECC codewords per block, more than dmMaxECC", s.Rows, s.Cols, n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		glog := dmGenLog[n]
		if len(glog) != n {
			t.Fatalf("dmGenLog[%d] holds %d coefficients, want %d", n, len(glog), n)
		}
		for i, c := range dmGenerator(n)[1:] {
			if c == 0 {
				t.Fatalf("generator of degree %d has a zero coefficient", n)
			}
			if got := dmGFExp[glog[i]]; got != c {
				t.Errorf("dmGenLog[%d][%d] = log(%d), want log(%d)", n, i, got, c)
			}
		}
		for r := 1; r <= n; r++ {
			x := dmGFExp[r]
			acc := byte(1)
			for _, logC := range glog {
				acc = dmMul(acc, x) ^ dmGFExp[logC]
			}
			if acc != 0 {
				t.Errorf("generator of degree %d at alpha^%d = %d, want 0", n, r, acc)
			}
		}
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

// dmSyndromes evaluates a block, highest degree coefficient first, at
// alpha^1..alpha^n. Every syndrome of a valid ECC 200 block is zero.
func dmSyndromes(block []byte, n int) []byte {
	syn := make([]byte, n)
	for r := 1; r <= n; r++ {
		x := dmGFExp[r]
		acc := byte(0)
		for _, c := range block {
			acc = dmMul(acc, x) ^ c
		}
		syn[r-1] = acc
	}
	return syn
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
