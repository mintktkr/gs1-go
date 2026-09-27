package symbol

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/galenzo17/gs1-go"
)

// Benchmark inputs: a typical pharma unit pack, a large GS1 element string
// (nine distinct 90-character internal AIs, 120x120 with six Reed-Solomon
// blocks), and plain data filling 144x144, the largest size and the one with
// the rotated ECC layout, for the pipeline stage benchmarks.
var (
	benchSmall      = "(01)04150000021126(17)250630(10)ABC123(21)SN0001"
	benchLarge      = "(01)04150000021126" + benchInternal()
	benchLargePlain = strings.Repeat("ABCDEFGHIJ", 155)
)

func benchInternal() string {
	var b strings.Builder
	for ai := 91; ai <= 99; ai++ {
		fmt.Fprintf(&b, "(%d)%s", ai, strings.Repeat("ABCDEFGHIJ", 9))
	}
	return b.String()
}

func benchElements(b *testing.B, in string) []gs1.Element {
	bc, err := gs1.Parse(in)
	if err != nil {
		b.Fatal(err)
	}
	return bc.Elements
}

func benchEncode(b *testing.B, in string) {
	elems := benchElements(b, in)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := GS1DataMatrix(elems, DataMatrixOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGS1DataMatrixSmall(b *testing.B) { benchEncode(b, benchSmall) }
func BenchmarkGS1DataMatrixLarge(b *testing.B) { benchEncode(b, benchLarge) }

func benchStages(b *testing.B, in string) (dmSize, []byte, []byte) {
	cw := dmEncodeASCII(in, false)
	s, err := dmSelectSize(len(cw), false)
	if err != nil {
		b.Fatal(err)
	}
	padded := dmPad(cw, s.DataCW)
	return s, padded, dmECC(padded, s)
}

func BenchmarkECCLarge(b *testing.B) {
	s, padded, _ := benchStages(b, benchLargePlain)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dmECC(padded, s)
	}
}

func BenchmarkPlaceLarge(b *testing.B) {
	s, _, full := benchStages(b, benchLargePlain)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dmPlace(full, s)
	}
}

func benchMatrix(b *testing.B, in string) *Matrix {
	m, err := GS1DataMatrix(benchElements(b, in), DataMatrixOptions{})
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
