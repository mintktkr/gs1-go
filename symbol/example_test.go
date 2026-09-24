package symbol_test

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"

	"github.com/galenzo17/gs1-go/symbol"
)

func ExampleGS1DataMatrix() {
	m, err := symbol.GS1DataMatrix("(01)04150000021126(17)250630(10)ABC123", symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d x %d modules\n", m.Rows, m.Cols)
	// Output: 20 x 20 modules
}

// ExampleEncoder encodes two sizes with one Encoder, which keeps the module
// layout of each size it has seen, and renders the first symbol.
func ExampleEncoder() {
	var e symbol.Encoder
	for _, in := range []string{
		"(01)04150000021126(17)250630(10)ABC123",
		"(01)04150000021126(17)250630(10)ABC123(21)SN0001",
	} {
		m, err := e.GS1DataMatrix(in, symbol.DataMatrixOptions{})
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		fmt.Printf("%d x %d\n", m.Rows, m.Cols)
	}
	// Output:
	// 20 x 20
	// 22 x 22
}

func ExampleMatrix_SVG() {
	m, err := symbol.GS1DataMatrix("(01)04150000021126", symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	svg := m.SVG(4, 2)
	header, _, _ := strings.Cut(svg, "\n")
	fmt.Println(header)
	// Output: <svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="80" height="80" viewBox="0 0 80 80" shape-rendering="crispEdges">
}

func ExampleMatrix_Paletted() {
	m, err := symbol.GS1DataMatrix("(01)04150000021126", symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var buf bytes.Buffer
	img := m.Paletted(4, 2)
	if err := png.Encode(&buf, img); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("PNG: %v, bounds: %v\n", bytes.HasPrefix(buf.Bytes(), []byte("\x89PNG\r\n\x1a\n")), img.Bounds())
	// Output: PNG: true, bounds: (0,0)-(80,80)
}
