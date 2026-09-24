package symbol

// dmPlace lays the final codeword stream (data + ECC) into a symbol of size
// s, including finder and alignment patterns.
func dmPlace(cw []byte, s dmSize) *Matrix {
	out := newMatrix(s.Rows, s.Cols)
	v, h := s.regions()
	for r := 0; r < v; r++ {
		for c := 0; c < h; c++ {
			dmFinder(out, r*(s.RegionRows+2), c*(s.RegionCols+2), s.RegionRows+2, s.RegionCols+2)
		}
	}
	m := newDMMapping(out, s)
	m.fill(cw)
	return out
}

// dmFinder draws one data region's finder and alignment pattern at row, col:
// a solid left column and bottom row, and clock tracks of every other module
// dark along the top row (starting dark) and the right column (starting
// light) (ISO/IEC 16022 figure 3).
func dmFinder(out *Matrix, row, col, rows, cols int) {
	base := row * out.Cols
	for i := 0; i < rows; i++ {
		out.mods[base+i*out.Cols+col] = true
	}
	for j := 0; j < cols; j++ {
		out.mods[base+(rows-1)*out.Cols+col+j] = true
	}
	for j := 0; j < cols; j += 2 {
		out.mods[base+col+j] = true
	}
	for i := 1; i < rows; i += 2 {
		out.mods[base+i*out.Cols+col+cols-1] = true
	}
}

// dmMapping walks the mapping matrix of ISO/IEC 16022 Annex F — all data
// regions joined, finder and alignment patterns removed, one module per
// codeword bit — and writes every module straight into the symbol. Mapping row
// r is symbol row r + 2*(r/RegionRows) + 1 and mapping column c is symbol
// column c + 2*(c/RegionCols) + 1; the walk visits mapping rows and columns in
// no particular order, so both translations are lookup tables.
type dmMapping struct {
	out        *Matrix
	rows, cols int      // mapping matrix size
	rowOff     []int    // mapping row -> first module of that row in out.mods
	colOff     []int    // mapping column -> its column in out.mods
	corner     []uint64 // bits of out.mods: modules the corner patterns placed
	used       int      // codewords consumed
}

// newDMMapping returns a writer for the data modules of out, the symbol with
// its finder patterns already drawn.
func newDMMapping(out *Matrix, s dmSize) *dmMapping {
	rows, cols := s.mappingSize()
	m := &dmMapping{
		out:    out,
		rows:   rows,
		cols:   cols,
		rowOff: make([]int, rows),
		colOff: make([]int, cols),
		corner: make([]uint64, (len(out.mods)+63)/64),
	}
	for r := range m.rowOff {
		m.rowOff[r] = (r + 2*(r/s.RegionRows) + 1) * out.Cols
	}
	for c := range m.colOff {
		m.colOff[c] = c + 2*(c/s.RegionCols) + 1
	}
	return m
}

// placed reports whether a corner pattern has already placed the mapping
// module at row, col. The corner patterns are the only modules a later sweep
// reaches again — every other sweep position is still empty — so they are the
// only ones the walk has to remember (see cornerModule).
func (m *dmMapping) placed(row, col int) bool {
	i := m.rowOff[row] + m.colOff[col]
	return m.corner[i/64]&(1<<(i%64)) != 0
}

func (m *dmMapping) set(row, col int, dark bool) {
	m.out.mods[m.rowOff[row]+m.colOff[col]] = dark
}

// next returns the next codeword and advances the codeword counter.
func (m *dmMapping) next(cw []byte) byte {
	c := cw[m.used]
	m.used++
	return c
}

// fill walks the mapping matrix filling it with cw as described in ISO/IEC
// 16022 Annex F.
func (m *dmMapping) fill(cw []byte) {
	row, col := 4, 0
	for {
		m.corners(row, col, cw)
		row, col = m.sweepUp(row, col, cw)
		row, col = m.sweepDown(row, col, cw)
		if row >= m.rows && col >= m.cols {
			break
		}
	}
	// A symbol whose module count is not a multiple of 8 leaves these four
	// modules of the last codeword group empty; they get a fixed pattern.
	if 8*m.used != m.rows*m.cols {
		m.set(m.rows-1, m.cols-1, true)
		m.set(m.rows-2, m.cols-2, true)
		m.set(m.rows-1, m.cols-2, false)
		m.set(m.rows-2, m.cols-1, false)
	}
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

// sweepDown places codewords walking down and to the left until it leaves the
// matrix, and returns the start of the next sweep.
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
	m.set(row, col, c>>(8-bit)&1 != 0)
}

// cornerModule places bit (1 = MSB, 8 = LSB) of codeword c at row, col of a
// corner pattern, and records the module: the sweeps tile the mapping matrix
// around the corner patterns and meet them again only there, so these
// placements are all the walk has to skip later.
func (m *dmMapping) cornerModule(row, col int, c byte, bit uint) {
	m.module(row, col, c, bit)
	i := m.rowOff[row] + m.colOff[col]
	m.corner[i/64] |= 1 << (i % 64)
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
	m.cornerModule(m.rows-1, 0, c, 1)
	m.cornerModule(m.rows-1, 1, c, 2)
	m.cornerModule(m.rows-1, 2, c, 3)
	m.cornerModule(0, m.cols-2, c, 4)
	m.cornerModule(0, m.cols-1, c, 5)
	m.cornerModule(1, m.cols-1, c, 6)
	m.cornerModule(2, m.cols-1, c, 7)
	m.cornerModule(3, m.cols-1, c, 8)
}

func (m *dmMapping) corner2(c byte) {
	m.cornerModule(m.rows-3, 0, c, 1)
	m.cornerModule(m.rows-2, 0, c, 2)
	m.cornerModule(m.rows-1, 0, c, 3)
	m.cornerModule(0, m.cols-4, c, 4)
	m.cornerModule(0, m.cols-3, c, 5)
	m.cornerModule(0, m.cols-2, c, 6)
	m.cornerModule(0, m.cols-1, c, 7)
	m.cornerModule(1, m.cols-1, c, 8)
}

func (m *dmMapping) corner3(c byte) {
	m.cornerModule(m.rows-3, 0, c, 1)
	m.cornerModule(m.rows-2, 0, c, 2)
	m.cornerModule(m.rows-1, 0, c, 3)
	m.cornerModule(0, m.cols-2, c, 4)
	m.cornerModule(0, m.cols-1, c, 5)
	m.cornerModule(1, m.cols-1, c, 6)
	m.cornerModule(2, m.cols-1, c, 7)
	m.cornerModule(3, m.cols-1, c, 8)
}

func (m *dmMapping) corner4(c byte) {
	m.cornerModule(m.rows-1, 0, c, 1)
	m.cornerModule(m.rows-1, m.cols-1, c, 2)
	m.cornerModule(0, m.cols-3, c, 3)
	m.cornerModule(0, m.cols-2, c, 4)
	m.cornerModule(0, m.cols-1, c, 5)
	m.cornerModule(1, m.cols-3, c, 6)
	m.cornerModule(1, m.cols-2, c, 7)
	m.cornerModule(1, m.cols-1, c, 8)
}
