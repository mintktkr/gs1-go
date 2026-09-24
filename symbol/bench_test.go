package symbol

import (
	"image/png"
	"io"
	"strings"
	"testing"
)

// Benchmark inputs: a typical pharma unit pack, and a large symbol that
// exercises multi-block Reed-Solomon and the 144x144 layout.
var (
	benchSmall = "(01)04150000021126(17)250630(10)ABC123(21)SN0001"
	benchLarge = "(01)04150000021126" + strings.Repeat("(91)"+strings.Repeat("ABCDEFGHIJ", 9), 16)
)

func benchEncode(b *testing.B, in string) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := GS1DataMatrix(in, DataMatrixOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGS1DataMatrixSmall(b *testing.B) { benchEncode(b, benchSmall) }
func BenchmarkGS1DataMatrixLarge(b *testing.B) { benchEncode(b, benchLarge) }

func benchStages(b *testing.B, in string) (dmSize, []byte, []byte) {
	cw, err := dmEncodeASCII(in, false)
	if err != nil {
		b.Fatal(err)
	}
	s, err := dmSelectSize(len(cw), false)
	if err != nil {
		b.Fatal(err)
	}
	padded := dmPad(cw, s.DataCW)
	return s, padded, dmECC(padded, s)
}

func BenchmarkECCLarge(b *testing.B) {
	s, padded, _ := benchStages(b, benchLarge)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dmECC(padded, s)
	}
}

func BenchmarkPlaceLarge(b *testing.B) {
	s, _, full := benchStages(b, benchLarge)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dmPlace(full, s)
	}
}

func benchMatrix(b *testing.B, in string) *Matrix {
	m, err := GS1DataMatrix(in, DataMatrixOptions{})
	if err != nil {
		b.Fatal(err)
	}
	return m
}

func BenchmarkPNGSmall(b *testing.B) {
	m := benchMatrix(b, benchSmall)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := m.PNG(io.Discard, 10, 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPNGLarge(b *testing.B) {
	m := benchMatrix(b, benchLarge)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := m.PNG(io.Discard, 10, 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSVGSmall(b *testing.B) {
	m := benchMatrix(b, benchSmall)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.SVG(10, 1)
	}
}

func BenchmarkSVGLarge(b *testing.B) {
	m := benchMatrix(b, benchLarge)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.SVG(10, 1)
	}
}

func BenchmarkPalettedPNGSmall(b *testing.B) {
	m := benchMatrix(b, benchSmall)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := png.Encode(io.Discard, m.Paletted(10, 1)); err != nil {
			b.Fatal(err)
		}
	}
}
