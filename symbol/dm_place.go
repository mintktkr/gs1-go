package symbol

// dmPlace lays the final codeword stream (data + ECC) into a symbol of size
// s, including finder and alignment patterns.
func dmPlace(cw []byte, s dmSize) *Matrix {
	m := dmMappingOf(cw, s)
	out := newMatrix(s.Rows, s.Cols)
	v, h := s.regions()
	for r := 0; r < v; r++ {
		for c := 0; c < h; c++ {
			dmRegion(out, m, s, r, c)
		}
	}
	return out
}

// dmRegion writes the data region at region coordinates r, c: its finder and
// alignment pattern, then the mapping modules it carries. Region (r, c)
// starts at symbol row r*(RegionRows+2), column c*(RegionCols+2).
func dmRegion(out *Matrix, m *dmMapping, s dmSize, r, c int) {
	row := r * (s.RegionRows + 2)
	col := c * (s.RegionCols + 2)
	dmFinder(out, row, col, s.RegionRows+2, s.RegionCols+2)
	for i := 0; i < s.RegionRows; i++ {
		for j := 0; j < s.RegionCols; j++ {
			out.set(row+1+i, col+1+j, m.get(r*s.RegionRows+i, c*s.RegionCols+j) == 1)
		}
	}
}

// dmFinder draws one data region's finder and alignment pattern at row, col:
// a solid left column and bottom row, and clock tracks of every other module
// dark along the top row (starting dark) and the right column (starting
// light) (ISO/IEC 16022 figure 3).
func dmFinder(out *Matrix, row, col, rows, cols int) {
	for i := 0; i < rows; i++ {
		out.set(row+i, col, true)
	}
	for j := 0; j < cols; j++ {
		out.set(row+rows-1, col+j, true)
	}
	for j := 0; j < cols; j += 2 {
		out.set(row, col+j, true)
	}
	for i := 1; i < rows; i += 2 {
		out.set(row+i, col+cols-1, true)
	}
}

// dmMapping is the mapping matrix of ISO/IEC 16022 Annex F: all data regions
// joined, finder and alignment patterns removed, one 0/1 module per codeword
// bit.
type dmMapping struct {
	rows, cols int
	val        []int8 // -1 until placed, afterwards the module value
	used       int    // codewords consumed, must end up as len(cw)
	writes     int    // modules written, must end up as rows*cols
}

func newDMMapping(rows, cols int) *dmMapping {
	m := &dmMapping{rows: rows, cols: cols, val: make([]int8, rows*cols)}
	for i := range m.val {
		m.val[i] = -1
	}
	return m
}

func (m *dmMapping) get(row, col int) int8 { return m.val[row*m.cols+col] }

func (m *dmMapping) placed(row, col int) bool { return m.val[row*m.cols+col] >= 0 }

func (m *dmMapping) set(row, col int, v int8) {
	m.val[row*m.cols+col] = v
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
		m.set(nrow-1, ncol-1, 1)
		m.set(nrow-2, ncol-2, 1)
		m.set(nrow-1, ncol-2, 0)
		m.set(nrow-2, ncol-1, 0)
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
// negative coordinate back into the matrix as Annex F specifies.
func (m *dmMapping) module(row, col int, c byte, bit uint) {
	if row < 0 {
		row += m.rows
		col += 4 - (m.rows+4)%8
	}
	if col < 0 {
		col += m.cols
		row += 4 - (m.cols+4)%8
	}
	m.set(row, col, int8(c>>(8-bit))&1)
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
