package symbol_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
)

// ExampleGS1DataMatrix encodes an element string given in bracket notation:
// parse it, then encode the elements.
func ExampleGS1DataMatrix() {
	b, err := gs1.Parse("(01)04150000021126(17)250630(10)ABC123")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	m, err := symbol.GS1DataMatrix(b.Elements, symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d x %d modules\n", m.Rows, m.Cols)
	// Output: 20 x 20 modules
}

// ExampleGS1DataMatrix_rectangular builds the elements directly and asks for
// a rectangular symbol, which fits narrow labels.
func ExampleGS1DataMatrix_rectangular() {
	m, err := symbol.GS1DataMatrix([]gs1.Element{
		{AI: "01", Value: "04150000021126"},
		{AI: "10", Value: "ABC123"},
	}, symbol.DataMatrixOptions{Rectangular: true})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d x %d modules\n", m.Rows, m.Cols)
	// Output: 12 x 26 modules
}

func ExampleMatrix_SVG() {
	m, err := symbol.DataMatrix("Hello", symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	header, _, _ := strings.Cut(m.SVG(4, 1), "\n")
	fmt.Println(header)
	// Output: <svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="56" height="56" viewBox="0 0 56 56" shape-rendering="crispEdges">
}

// ExampleMatrix_PNG writes a symbol at 10 pixels per module with the one
// module quiet zone GS1 DataMatrix requires.
func ExampleMatrix_PNG() {
	m, err := symbol.DataMatrix("Hello", symbol.DataMatrixOptions{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var buf bytes.Buffer
	if err := m.PNG(&buf, 10, 1); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(bytes.HasPrefix(buf.Bytes(), []byte("\x89PNG")))
	// Output: true
}
