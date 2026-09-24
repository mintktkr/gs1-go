package symbol

import (
	"errors"
	"strings"

	"github.com/galenzo17/gs1-go"
)

// ErrDataTooLong is returned when the data does not fit the largest symbol
// of the requested shape.
var ErrDataTooLong = errors.New("symbol: data too long for Data Matrix")

// DataMatrixOptions configures Data Matrix symbol selection.
type DataMatrixOptions struct {
	// Rectangular selects the smallest rectangular size instead of the
	// smallest square size. Rectangular symbols hold at most 49 data
	// codewords.
	Rectangular bool
}

// GS1DataMatrix encodes a GS1 element string as a GS1 DataMatrix symbol.
// The input accepts anything gs1.Parse accepts, including bracket notation
// such as "(01)04150000021126(17)250630(10)ABC123". The element string is
// validated by parsing before encoding, and FNC1 is placed after every
// element whose AI is not of predefined length, except the last.
func GS1DataMatrix(input string, opts DataMatrixOptions) (*Matrix, error) {
	b, err := gs1.Parse(input)
	if err != nil {
		return nil, err
	}
	cw, err := dmEncodeASCII(elementString(b.Elements), true)
	if err != nil {
		return nil, err
	}
	return encodeDataMatrix(cw, opts)
}

// DataMatrix encodes arbitrary data as a plain (non-GS1) ECC 200 Data Matrix
// symbol. Bytes above 127 are encoded with the upper shift codeword.
func DataMatrix(data string, opts DataMatrixOptions) (*Matrix, error) {
	cw, err := dmEncodeASCII(data, false)
	if err != nil {
		return nil, err
	}
	return encodeDataMatrix(cw, opts)
}

func encodeDataMatrix(cw []byte, opts DataMatrixOptions) (*Matrix, error) {
	s, err := dmSelectSize(len(cw), opts.Rectangular)
	if err != nil {
		return nil, err
	}
	return dmPlace(dmECC(dmPad(cw, s.DataCW), s), s), nil
}

// elementString joins elements with FNC1 (ASCII 29) after each
// variable-length field except the last. Element order is kept as given.
//
// This is a stand-in for gs1.Encode (#16, PR #34), which also reorders
// predefined-length AIs first and validates the character set.
func elementString(elems []gs1.Element) string {
	var b strings.Builder
	for i, e := range elems {
		b.WriteString(e.AI)
		b.WriteString(e.Value)
		if i < len(elems)-1 && !predefinedLength(e.AI) {
			b.WriteByte(0x1D)
		}
	}
	return b.String()
}

// predefinedLength reports whether an AI is in GS1 General Specifications
// table 7.8.5-2, the AIs that never need a trailing FNC1.
func predefinedLength(ai string) bool {
	if len(ai) < 2 {
		return false
	}
	switch ai[:2] {
	case "00", "01", "02", "03", "04", "11", "12", "13", "14", "15", "16",
		"17", "18", "19", "20", "31", "32", "33", "34", "35", "36", "41":
		return true
	}
	return false
}

// dmSize describes one ECC 200 symbol size (ISO/IEC 16022 table 7).
type dmSize struct {
	Rows, Cols             int // symbol size including finder patterns
	RegionRows, RegionCols int // data region size excluding finder patterns
	DataCW                 int // data codewords
	ECCCW                  int // error correction codewords, all blocks
	Blocks                 int // interleaved Reed-Solomon blocks
}

// regions returns the number of data regions vertically and horizontally.
func (s dmSize) regions() (v, h int) {
	return s.Rows / (s.RegionRows + 2), s.Cols / (s.RegionCols + 2)
}

// mappingSize returns the size of the mapping matrix: all data regions
// joined, finder patterns and alignment patterns removed.
func (s dmSize) mappingSize() (rows, cols int) {
	v, h := s.regions()
	return v * s.RegionRows, h * s.RegionCols
}

// dmSizes lists the 24 square sizes followed by the 6 rectangular sizes,
// each group in increasing capacity.
var dmSizes = []dmSize{
	{10, 10, 8, 8, 3, 5, 1},
	{12, 12, 10, 10, 5, 7, 1},
	{14, 14, 12, 12, 8, 10, 1},
	{16, 16, 14, 14, 12, 12, 1},
	{18, 18, 16, 16, 18, 14, 1},
	{20, 20, 18, 18, 22, 18, 1},
	{22, 22, 20, 20, 30, 20, 1},
	{24, 24, 22, 22, 36, 24, 1},
	{26, 26, 24, 24, 44, 28, 1},
	{32, 32, 14, 14, 62, 36, 1},
	{36, 36, 16, 16, 86, 42, 1},
	{40, 40, 18, 18, 114, 48, 1},
	{44, 44, 20, 20, 144, 56, 1},
	{48, 48, 22, 22, 174, 68, 1},
	{52, 52, 24, 24, 204, 84, 2},
	{64, 64, 14, 14, 280, 112, 2},
	{72, 72, 16, 16, 368, 144, 4},
	{80, 80, 18, 18, 456, 192, 4},
	{88, 88, 20, 20, 576, 224, 4},
	{96, 96, 22, 22, 696, 272, 4},
	{104, 104, 24, 24, 816, 336, 6},
	{120, 120, 18, 18, 1050, 408, 6},
	{132, 132, 20, 20, 1304, 496, 8},
	{144, 144, 22, 22, 1558, 620, 10},

	{8, 18, 6, 16, 5, 7, 1},
	{8, 32, 6, 14, 10, 11, 1},
	{12, 26, 10, 24, 16, 14, 1},
	{12, 36, 10, 16, 22, 18, 1},
	{16, 36, 14, 16, 32, 24, 1},
	{16, 48, 14, 22, 49, 28, 1},
}
