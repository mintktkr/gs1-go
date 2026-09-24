package symbol

import "sync"

// dmPlace lays the final codeword stream (data + ECC) into a symbol of size
// s, including finder and alignment patterns. The layout depends only on the
// size, never on the data, so it comes from the size's placement table.
func dmPlace(cw []byte, s dmSize) *Matrix {
	tab := dmPlaceTable(s)
	out := newMatrix(s.Rows, s.Cols)
	for i, e := range tab {
		if e < dmPlaceFixed {
			out.mods[i] = cw[e>>3]>>(7-(e&7))&1 != 0
		} else {
			out.mods[i] = e == dmPlaceDark
		}
	}
	return out
}

// A placement table entry says what its module holds: bit e&7 (0 = most
// significant) of codeword e>>3, a fixed light module, or a fixed dark one
// (finder, alignment, corner). The largest symbol, 144x144, has 2178
// codewords, so no codeword bit reaches dmPlaceLight.
const (
	dmPlaceLight uint16 = 0xFFFE
	dmPlaceDark  uint16 = 0xFFFF
	dmPlaceFixed uint16 = dmPlaceLight // entries from here up carry no bit
)

var (
	dmPlaceTablesMu sync.RWMutex
	dmPlaceTables   = map[dmSize][]uint16{}
)

// dmPlaceTable returns the placement table of s, building it on first use.
// A table is never modified once built, so all encoders can share it.
func dmPlaceTable(s dmSize) []uint16 {
	dmPlaceTablesMu.RLock()
	tab := dmPlaceTables[s]
	dmPlaceTablesMu.RUnlock()
	if tab != nil {
		return tab
	}
	dmPlaceTablesMu.Lock()
	defer dmPlaceTablesMu.Unlock()
	if tab = dmPlaceTables[s]; tab == nil {
		tab = buildDMPlaceTable(s)
		dmPlaceTables[s] = tab
	}
	return tab
}

// buildDMPlaceTable walks the mapping matrix of ISO/IEC 16022 Annex F once,
// over zero data, and records for every symbol module which codeword bit it
// carries, or that it is a fixed finder, alignment or corner module. The walk
// places modules by position only, so the result holds for any data.
func buildDMPlaceTable(s dmSize) []uint16 {
	m := dmMappingOf(make([]byte, s.DataCW+s.ECCCW), s)
	tab := make([]uint16, s.Rows*s.Cols)
	for i := range tab {
		tab[i] = dmPlaceLight
	}
	v, h := s.regions()
	for r := 0; r < v; r++ {
		for c := 0; c < h; c++ {
			row := r * (s.RegionRows + 2)
			col := c * (s.RegionCols + 2)
			dmFinderDark(tab, s.Cols, row, col, s.RegionRows+2, s.RegionCols+2)
			for i := 0; i < s.RegionRows; i++ {
				for j := 0; j < s.RegionCols; j++ {
					tab[(row+1+i)*s.Cols+col+1+j] = m.pos[(r*s.RegionRows+i)*m.cols+c*s.RegionCols+j]
				}
			}
		}
	}
	return tab
}

// dmFinderDark marks the finder and alignment pattern of one data region at
// row, col in a placement table of the given width: a solid left column and
// bottom row, and clock tracks of every other module dark along the top row
// (starting dark) and the right column (starting light) (ISO/IEC 16022 figure
// 3). The modules inside the pattern stay light here and are overwritten by
// the mapping modules of the region.
func dmFinderDark(tab []uint16, width, row, col, rows, cols int) {
	for i := 0; i < rows; i++ {
		tab[(row+i)*width+col] = dmPlaceDark
	}
	for j := 0; j < cols; j++ {
		tab[(row+rows-1)*width+col+j] = dmPlaceDark
	}
	for j := 0; j < cols; j += 2 {
		tab[row*width+col+j] = dmPlaceDark
	}
	for i := 1; i < rows; i += 2 {
		tab[(row+i)*width+col+cols-1] = dmPlaceDark
	}
}

// dmMapping is the mapping matrix of ISO/IEC 16022 Annex F: all data regions
// joined, finder and alignment patterns removed, one 0/1 module per codeword
// bit.
type dmMapping struct {
	rows, cols int
	val        []int8   // -1 until placed, afterwards the module value
	pos        []uint16 // placement table entry of the module: codeword bit or fixed pattern
	used       int      // codewords consumed, must end up as len(cw)
	writes     int      // modules written, must end up as rows*cols
}

func newDMMapping(rows, cols int) *dmMapping {
	m := &dmMapping{rows: rows, cols: cols, val: make([]int8, rows*cols), pos: make([]uint16, rows*cols)}
	for i := range m.val {
		m.val[i] = -1
	}
	return m
}

func (m *dmMapping) get(row, col int) int8 { return m.val[row*m.cols+col] }

func (m *dmMapping) placed(row, col int) bool { return m.val[row*m.cols+col] >= 0 }

// set places module row, col with value v and placement table entry pos.
func (m *dmMapping) set(row, col int, v int8, pos uint16) {
	m.val[row*m.cols+col] = v
	m.pos[row*m.cols+col] = pos
	m.writes++
}

// next returns the next codeword and advances the codeword counter.
func (m *dmMapping) next(cw []byte) byte {
	c := cw[m.used]
	m.used++
	return c
}

// dmMappingOf fills the mapping matrix with cw as described in ISO/IEC 16022
// Annex F and returns it.
func dmMappingOf(cw []byte, s dmSize) *dmMapping {
	nrow, ncol := s.mappingSize()
	m := newDMMapping(nrow, ncol)
	row, col := 4, 0
	for {
		m.corners(row, col, cw)
		row, col = m.sweepUp(row, col, cw)
		row, col = m.sweepDown(row, col, cw)
		if row >= nrow && col >= ncol {
			break
		}
	}
	// A symbol whose module count is not a multiple of 8 leaves these four
	// modules of the last codeword group empty; they get a fixed pattern.
	if !m.placed(nrow-1, ncol-1) {
		m.set(nrow-1, ncol-1, 1, dmPlaceDark)
		m.set(nrow-2, ncol-2, 1, dmPlaceDark)
		m.set(nrow-1, ncol-2, 0, dmPlaceLight)
		m.set(nrow-2, ncol-1, 0, dmPlaceLight)
	}
	return m
}

// sweepUp places codewords walking up and to the right until it leaves the
// matrix, and returns the start of the next sweep (ISO/IEC 16022 Annex F).
func (m *dmMapping) sweepUp(row, col int, cw []byte) (int, int) {
	for {
		if row < m.rows && col >= 0 && !m.placed(row, col) {
			m.utah(row, col, m.next(cw))
		}
		row -= 2
		col += 2
		if row < 0 || col >= m.cols {
			break
		}
	}
	return row + 1, col + 3
}

// sweepDown places codewords walking down and to the left until it leaves
// the matrix, and returns the start of the next sweep.
func (m *dmMapping) sweepDown(row, col int, cw []byte) (int, int) {
	for {
		if row >= 0 && col < m.cols && !m.placed(row, col) {
			m.utah(row, col, m.next(cw))
		}
		row += 2
		col -= 2
		if row >= m.rows || col < 0 {
			break
		}
	}
	return row + 3, col + 1
}

// corners places the codewords of the special corner patterns the sweep
// reaches (ISO/IEC 16022 Annex F).
func (m *dmMapping) corners(row, col int, cw []byte) {
	if row == m.rows && col == 0 {
		m.corner1(m.next(cw))
	}
	if row == m.rows-2 && col == 0 && m.cols%4 != 0 {
		m.corner2(m.next(cw))
	}
	if row == m.rows-2 && col == 0 && m.cols%8 == 4 {
		m.corner3(m.next(cw))
	}
	if row == m.rows+4 && col == 2 && m.cols%8 == 0 {
		m.corner4(m.next(cw))
	}
}

// module places bit (1 = MSB, 8 = LSB) of codeword c at row, col, wrapping a
// negative coordinate back into the matrix as Annex F specifies. It records
// the module's placement table entry, bit bit-1 of codeword m.used-1, because
// c is always the codeword next returned last.
func (m *dmMapping) module(row, col int, c byte, bit uint) {
	if row < 0 {
		row += m.rows
		col += 4 - (m.rows+4)%8
	}
	if col < 0 {
		col += m.cols
		row += 4 - (m.cols+4)%8
	}
	var v int8
	if c>>(8-bit)&1 != 0 {
		v = 1
	}
	m.set(row, col, v, uint16(((m.used-1)*8+(int(bit)-1))&0xFFFF))
}

// utah places one codeword in the 8 module "utah" shape ending at row, col
// (ISO/IEC 16022 Annex F).
func (m *dmMapping) utah(row, col int, c byte) {
	m.module(row-2, col-2, c, 1)
	m.module(row-2, col-1, c, 2)
	m.module(row-1, col-2, c, 3)
	m.module(row-1, col-1, c, 4)
	m.module(row-1, col, c, 5)
	m.module(row, col-2, c, 6)
	m.module(row, col-1, c, 7)
	m.module(row, col, c, 8)
}

func (m *dmMapping) corner1(c byte) {
	m.module(m.rows-1, 0, c, 1)
	m.module(m.rows-1, 1, c, 2)
	m.module(m.rows-1, 2, c, 3)
	m.module(0, m.cols-2, c, 4)
	m.module(0, m.cols-1, c, 5)
	m.module(1, m.cols-1, c, 6)
	m.module(2, m.cols-1, c, 7)
	m.module(3, m.cols-1, c, 8)
}

func (m *dmMapping) corner2(c byte) {
	m.module(m.rows-3, 0, c, 1)
	m.module(m.rows-2, 0, c, 2)
	m.module(m.rows-1, 0, c, 3)
	m.module(0, m.cols-4, c, 4)
	m.module(0, m.cols-3, c, 5)
	m.module(0, m.cols-2, c, 6)
	m.module(0, m.cols-1, c, 7)
	m.module(1, m.cols-1, c, 8)
}

func (m *dmMapping) corner3(c byte) {
	m.module(m.rows-3, 0, c, 1)
	m.module(m.rows-2, 0, c, 2)
	m.module(m.rows-1, 0, c, 3)
	m.module(0, m.cols-2, c, 4)
	m.module(0, m.cols-1, c, 5)
	m.module(1, m.cols-1, c, 6)
	m.module(2, m.cols-1, c, 7)
	m.module(3, m.cols-1, c, 8)
}

func (m *dmMapping) corner4(c byte) {
	m.module(m.rows-1, 0, c, 1)
	m.module(m.rows-1, m.cols-1, c, 2)
	m.module(0, m.cols-3, c, 3)
	m.module(0, m.cols-2, c, 4)
	m.module(0, m.cols-1, c, 5)
	m.module(1, m.cols-3, c, 6)
	m.module(1, m.cols-2, c, 7)
	m.module(1, m.cols-1, c, 8)
}
