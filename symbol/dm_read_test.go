package symbol

import (
	"errors"
	"fmt"
	"math/bits"
	"strings"
	"sync"
	"testing"

	"github.com/galenzo17/gs1-go"
)

// dmRead decodes a Data Matrix symbol back to its data, the inverse of the
// encoder, so round trips run in go test without a decoder dependency.
// gs1 reports a leading FNC1; later FNC1 are returned as ASCII 29.
//
// Module positions come from probing dmPlace, whose output is checked
// module for module against zint in TestDataMatrixGolden, and whose layout
// is checked by an independent reader in verify-decode.sh. Reed-Solomon is
// verified through the syndromes of every block rather than by recomputing
// the ECC, and the ASCII decoding is written apart from dmEncodeASCII.
func dmRead(m *Matrix) (data string, gs1 bool, err error) {
	s, ok := dmSizeOf(m.Rows, m.Cols)
	if !ok {
		return "", false, fmt.Errorf("no symbol size %dx%d", m.Rows, m.Cols)
	}
	pos := dmPositions(s)
	stream := make([]byte, len(pos))
	for i, p := range pos {
		for b, at := range p {
			if m.mods[at] {
				stream[i] |= 0x80 >> b
			}
		}
	}
	if err := dmCheckSyndromes(stream, s); err != nil {
		return "", false, err
	}
	return dmDecodeASCII(stream[:s.DataCW])
}

func dmSizeOf(rows, cols int) (dmSize, bool) {
	for _, s := range dmSizes {
		if s.Rows == rows && s.Cols == cols {
			return s, true
		}
	}
	return dmSize{}, false
}

var dmPositionCache sync.Map // dmSize -> [][8]int

// dmPositions returns, for every codeword of the stream and every bit from
// the most significant, the index of its module in Matrix.mods. It places
// patterned streams: pass j sets every bit of codeword i when bit j of i is
// set, pass k sets bit b of every codeword when bit k of b is set. Modules
// that differ between an all-zero and an all-one stream hold data; the rest
// are finder, alignment and fixed corner modules.
func dmPositions(s dmSize) [][8]int {
	if p, ok := dmPositionCache.Load(s); ok {
		return p.([][8]int)
	}
	n := s.DataCW + s.ECCCW
	zero := dmProbe(s, nil, 0, func(int) byte { return 0 })
	one := dmProbe(s, nil, 0, func(int) byte { return 0xFF })
	idx := make([]int, len(zero))
	for j := 0; j < bits.Len(uint(n)); j++ {
		dmProbe(s, idx, 1<<j, func(i int) byte { return byte(0xFF * (i >> j & 1)) })
	}
	which := make([]int, len(zero))
	for k, c := range []byte{0x55, 0x33, 0x0F} {
		dmProbe(s, which, 1<<k, func(int) byte { return c })
	}
	pos := make([][8]int, n)
	found := make([]int, n)
	for at := range zero {
		if zero[at] != one[at] {
			pos[idx[at]][which[at]] = at
			found[idx[at]]++
		}
	}
	for i, f := range found {
		if f != 8 {
			panic(fmt.Sprintf("%dx%d: codeword %d has %d modules", s.Rows, s.Cols, i, f))
		}
	}
	dmPositionCache.Store(s, pos)
	return pos
}

// dmProbe places a stream whose codeword i is cw(i), ORs flag into acc for
// every dark module, and returns the modules.
func dmProbe(s dmSize, acc []int, flag int, cw func(i int) byte) []bool {
	stream := make([]byte, s.DataCW+s.ECCCW)
	for i := range stream {
		stream[i] = cw(i)
	}
	mods := dmPlace(stream, s).mods
	for at, dark := range mods {
		if dark && acc != nil {
			acc[at] |= flag
		}
	}
	return mods
}

// dmCheckSyndromes verifies every Reed-Solomon block: a valid codeword
// polynomial, data then ECC, evaluates to zero at alpha^1 ... alpha^n.
func dmCheckSyndromes(stream []byte, s dmSize) error {
	n := s.ECCCW / s.Blocks
	for b := 0; b < s.Blocks; b++ {
		var block []byte
		for i := b; i < s.DataCW; i += s.Blocks {
			block = append(block, stream[i])
		}
		for j := 0; j < n; j++ {
			block = append(block, stream[dmECCPos(s, b, j)])
		}
		for r := 1; r <= n; r++ {
			var v byte // Horner's rule at x = alpha^r
			for _, c := range block {
				v = dmMul(v, dmGFExp[r]) ^ c
			}
			if v != 0 {
				return fmt.Errorf("%dx%d: block %d syndrome %d is %d", s.Rows, s.Cols, b, r, v)
			}
		}
	}
	return nil
}

// dmDecodeASCII decodes ASCII-encodation data codewords up to the first pad
// (ISO/IEC 16022 5.2.3).
func dmDecodeASCII(cw []byte) (string, bool, error) {
	var b strings.Builder
	gs1 := false
	for i := 0; i < len(cw); i++ {
		c := cw[i]
		switch {
		case c >= 1 && c <= 128:
			b.WriteByte(c - 1)
		case c == 129:
			return b.String(), gs1, nil
		case c >= 130 && c <= 229:
			fmt.Fprintf(&b, "%02d", c-130)
		case c == 232 && i == 0:
			gs1 = true
		case c == 232:
			b.WriteByte(0x1D)
		case c == 235 && i+1 < len(cw):
			i++
			b.WriteByte(cw[i] + 127)
		default:
			return "", false, fmt.Errorf("codeword %d: unexpected %d", i, c)
		}
	}
	return b.String(), gs1, nil
}

func TestDataMatrixRoundTripSizes(t *testing.T) {
	for _, s := range dmSizes {
		data := strings.Repeat("ABCDEFGHIJ", s.DataCW/10+1)[:s.DataCW]
		m, err := DataMatrix(data, DataMatrixOptions{Rectangular: s.Rows != s.Cols})
		if err != nil {
			t.Fatalf("%dx%d: %v", s.Rows, s.Cols, err)
		}
		if m.Rows != s.Rows || m.Cols != s.Cols {
			t.Fatalf("%d codewords: got %dx%d, want %dx%d", s.DataCW, m.Rows, m.Cols, s.Rows, s.Cols)
		}
		got, gs1, err := dmRead(m)
		if err != nil || gs1 || got != data {
			t.Errorf("%dx%d: read %q, gs1=%v, err=%v", s.Rows, s.Cols, got, gs1, err)
		}
	}
}

func TestGS1DataMatrixRoundTrip(t *testing.T) {
	tests := []string{
		"(01)04150000021126",
		"(01)04150000021126(17)250630(10)ABC123(21)SN0001",
		// 00, 02 and 414 are predefined length, 37 is not: Encode puts
		// the predefined ones first and ends 37 with FNC1.
		"(00)106141411234567897(02)04150000021126(37)20(414)0614141123452(10)LOT42",
		// 402 is fixed length but not predefined, so FNC1 follows it.
		"(402)12345678901234560(10)LOT1",
		"(10)A-1/B.2%C&D(21)x'y*z,:;<=>?_",
	}
	for _, in := range tests {
		b, err := gs1.Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		want, err := gs1.Encode(b.Elements)
		if err != nil {
			t.Fatalf("Encode(%q): %v", in, err)
		}
		for _, rect := range []bool{false, true} {
			m, err := GS1DataMatrix(b.Elements, DataMatrixOptions{Rectangular: rect})
			if errors.Is(err, ErrDataTooLong) && rect {
				continue
			}
			if err != nil {
				t.Fatalf("GS1DataMatrix(%q): %v", in, err)
			}
			got, isGS1, err := dmRead(m)
			if err != nil || !isGS1 || "\x1D"+got != want {
				t.Errorf("%q rect=%v: read %q, gs1=%v, err=%v; want %q", in, rect, got, isGS1, err, want[1:])
			}
		}
	}
}

func FuzzDataMatrix(f *testing.F) {
	f.Add("123456", false)
	f.Add("A\xe9\x1d99", true)
	f.Add(strings.Repeat("ABCDEFGHIJ", 155), false)
	f.Fuzz(func(t *testing.T, data string, rect bool) {
		m, err := DataMatrix(data, DataMatrixOptions{Rectangular: rect})
		if data == "" || errors.Is(err, ErrDataTooLong) {
			if err == nil {
				t.Fatalf("DataMatrix(%q) succeeded", data)
			}
			return
		}
		if err != nil {
			t.Fatalf("DataMatrix(%q): %v", data, err)
		}
		got, gs1, err := dmRead(m)
		if err != nil || gs1 || got != data {
			t.Fatalf("read %q, gs1=%v, err=%v; want %q", got, gs1, err, data)
		}
	})
}

func FuzzGS1DataMatrix(f *testing.F) {
	f.Add("(01)04150000021126(17)250630(10)ABC123")
	f.Add("(402)12345678901234560(10)LOT1(21)S/N")
	f.Add("\x1d0104150000021126" + "91" + strings.Repeat("Z", 90))
	f.Fuzz(func(t *testing.T, input string) {
		b, err := gs1.Parse(input)
		if err != nil {
			return
		}
		want, err := gs1.Encode(b.Elements)
		m, merr := GS1DataMatrix(b.Elements, DataMatrixOptions{})
		if err != nil || errors.Is(merr, ErrDataTooLong) {
			if merr == nil {
				t.Fatalf("GS1DataMatrix accepted what Encode rejects: %v", err)
			}
			return
		}
		if merr != nil {
			t.Fatalf("GS1DataMatrix(%q): %v", input, merr)
		}
		got, isGS1, err := dmRead(m)
		if err != nil || !isGS1 || "\x1D"+got != want {
			t.Fatalf("read %q, gs1=%v, err=%v; want %q", got, isGS1, err, want[1:])
		}
	})
}
