package symbol_test

import (
	"fmt"

	"github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
)

// ExampleGS1DataMatrix encodes an element string given in bracket notation:
// split it with ParseHRI, then encode the elements.
func ExampleGS1DataMatrix() {
	elements, err := gs1.ParseHRI("(01)04150000021126(17)250630(10)ABC123")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	m, err := symbol.GS1DataMatrix(elements, symbol.DataMatrixOptions{})
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
