package symbol

import (
	"fmt"
	"testing"
)

// TestDMPlaceGolden10x10 checks a whole 10x10 symbol against zint's dump of
// "123456": its 3 data codewords (ASCII digit pairs) followed by the 5 ECC
// codewords of that size.
func TestDMPlaceGolden10x10(t *testing.T) {
	cw := []byte{142, 164, 186, 114, 25, 5, 88, 102}
	// zint -b DATAMATRIX --vers=1 --dump -d 123456
	dump := []string{"AA8", "CB4", "C10", "C74", "C20", "83C", "EC0", "F64", "9D0", "FFC"}
	compareDump(t, dmPlace(cw, dmSizes[0]), dump)
}

// TestDMPlaceCoverage checks that the placement fills every module of the
// mapping matrix and consumes exactly the codewords it was given. A module
// placed twice would leave a codeword bit without a module, which dmRead's
// position probe rejects.
func TestDMPlaceCoverage(t *testing.T) {
	for _, s := range dmSizes {
		t.Run(fmt.Sprintf("%dx%d", s.Rows, s.Cols), func(t *testing.T) {
			cw := dmTestCodewords(s.DataCW + s.ECCCW)
			m := dmMappingOf(cw, s)
			if m.used != len(cw) {
				t.Errorf("consumed %d codewords, want %d", m.used, len(cw))
			}
			rows, cols := s.mappingSize()
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					if !m.placed(r, c) {
						t.Fatalf("mapping module (%d,%d) was never placed", r, c)
					}
				}
			}
		})
	}
}
