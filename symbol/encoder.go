package symbol

import "sync"

// Encoder encodes Data Matrix symbols like the package-level functions
// GS1DataMatrix and DataMatrix, and caches the module layout of each symbol
// size it has encoded, so repeated encoding skips the ISO/IEC 16022 Annex F
// placement walk. The zero value is ready to use. An Encoder is safe for
// concurrent use and must not be copied after first use. A layout costs two
// bytes per module: about 41 KiB for 144x144, 220 KiB for all 30 sizes.
type Encoder struct {
	layouts [len(dmSizes)]dmLayoutSlot
}

// GS1DataMatrix encodes a GS1 element string as a GS1 DataMatrix symbol,
// accepting exactly what the package-level GS1DataMatrix accepts.
func (e *Encoder) GS1DataMatrix(input string, opts DataMatrixOptions) (*Matrix, error) {
	cw, err := gs1Codewords(input)
	if err != nil {
		return nil, err
	}
	return encodeDataMatrix(cw, opts, e)
}

// DataMatrix encodes arbitrary data as a plain (non-GS1) ECC 200 Data Matrix
// symbol, accepting exactly what the package-level DataMatrix accepts.
func (e *Encoder) DataMatrix(data string, opts DataMatrixOptions) (*Matrix, error) {
	cw, err := dmEncodeASCII(data, false)
	if err != nil {
		return nil, err
	}
	return encodeDataMatrix(cw, opts, e)
}

// layout returns the module layout of s, building and caching it on first use.
func (e *Encoder) layout(s dmSize) []uint16 {
	return e.layouts[dmSizeIndex(s)].get(s)
}

// dmLayoutSlot is the cached module layout of one symbol size; the zero value
// is ready to use. sync.Once publishes the layout to every encoder sharing the
// slot, and a built layout is never written again.
type dmLayoutSlot struct {
	once sync.Once
	tab  []uint16
}

// get returns the slot's layout of size s, building it on first use.
func (slot *dmLayoutSlot) get(s dmSize) []uint16 {
	slot.once.Do(func() { slot.tab = buildDMLayout(s) })
	return slot.tab
}
