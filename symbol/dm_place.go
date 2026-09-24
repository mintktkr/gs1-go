package symbol

// dmPlace lays the final codeword stream (data + ECC) into a symbol of size
// s, including finder and alignment patterns.
func dmPlace(cw []byte, s dmSize) *Matrix {
	out := newMatrix(s.Rows, s.Cols)
	dmFinders(out, s)
	m := newDMMapping(out, s)
	m.fill(cw)
	return out
}

// dmPlaceLayout lays the final codeword stream cw into a symbol of size s
// according to tab, the module layout of s as built by buildDMLayout. It
// walks no mapping matrix, which only pays off when the layout is reused.
func dmPlaceLayout(tab []uint16, cw []byte, s dmSize) *Matrix {
	out := newMatrix(s.Rows, s.Cols)
	for i, e := range tab {
		if e < dmLayoutFixed {
			out.mods[i] = cw[e>>3]>>(7-(e&7))&1 != 0
		} else {
			out.mods[i] = e == dmLayoutDark
		}
	}
	return out
}

// A module layout entry says what its symbol module holds: bit e&7 (0 = most
// significant) of codeword e>>3, a fixed light module, or a fixed dark one
// (finder, alignment and corner). The largest symbol, 144x144, has 2178
// codewords, so no codeword bit reaches dmLayoutLight and the two fixed
// entries need no sentinel outside the range.
const (
	dmLayoutLight uint16 = 0xFFFE
	dmLayoutDark  uint16 = 0xFFFF
	dmLayoutFixed uint16 = dmLayoutLight // entries from here up carry no bit
)

// buildDMLayout returns the module layout of size s: one entry per module of
// the symbol. The Annex F walk places modules by position only, never by
// value, so one walk over zero codewords describes every symbol of that size.
func buildDMLayout(s dmSize) []uint16 {
	tab := make([]uint16, s.Rows*s.Cols)
	for i := range tab {
		tab[i] = dmLayoutLight
	}
	finders := newMatrix(s.Rows, s.Cols)
	dmFinders(finders, s)
	for i, dark := range finders.mods {
		if dark {
			tab[i] = dmLayoutDark
		}
	}
	newDMLayoutMapping(tab, s).fill(make([]byte, s.DataCW+s.ECCCW))
	return tab
}

// dmFinders draws the finder and alignment patterns of every data region of s.
func dmFinders(out *Matrix, s dmSize) {
	v, h := s.regions()
	for r := 0; r < v; r++ {
		for c := 0; c < h; c++ {
			dmFinder(out, r*(s.RegionRows+2), c*(s.RegionCols+2), s.RegionRows+2, s.RegionCols+2)
		}
	}
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
// codeword bit — and writes every module to a sink. Exactly one sink is set:
// out, the symbol being placed, or layout, the module layout being recorded.
// Mapping row r is symbol row r + 2*(r/RegionRows) + 1 and mapping column c is
// symbol column c + 2*(c/RegionCols) + 1; the walk visits mapping rows and
// columns in no particular order, so both translations are lookup tables.
type dmMapping struct {
	out    *Matrix  // sink: the symbol with its finder patterns drawn
	layout []uint16 // sink: the module layout entries, in symbol positions
	width  int      // symbol columns, which is also the row stride

	rows, cols int      // mapping matrix size
	rowOff     []int    // mapping row -> first module of that row in the sink
	colOff     []int    // mapping column -> its column in the sink
	corner     []uint64 // bits of the sink: modules the corner patterns placed
	used       int      // codewords consumed
}

// newDMMapping returns a writer for the data modules of out, the symbol with
// its finder patterns already drawn.
func newDMMapping(out *Matrix, s dmSize) *dmMapping {
	m := &dmMapping{out: out, width: out.Cols}
	m.init(s)
	return m
}

// newDMLayoutMapping returns a writer for the module layout tab of a symbol
// of size s; the finder and alignment modules of tab must already be marked.
func newDMLayoutMapping(tab []uint16, s dmSize) *dmMapping {
	m := &dmMapping{layout: tab, width: s.Cols}
	m.init(s)
	return m
}

// init sizes the coordinate translations and the corner bitset of s.
func (m *dmMapping) init(s dmSize) {
	m.rows, m.cols = s.mappingSize()
	m.rowOff = make([]int, m.rows)
	m.colOff = make([]int, m.cols)
	m.corner = make([]uint64, (s.Rows*s.Cols+63)/64)
	for r := range m.rowOff {
		m.rowOff[r] = (r + 2*(r/s.RegionRows) + 1) * m.width
	}
	for c := range m.colOff {
		m.colOff[c] = c + 2*(c/s.RegionCols) + 1
	}
}

// placed reports whether a corner pattern has already placed the mapping
// module at row, col. The corner patterns are the only modules a later sweep
// reaches again — every other sweep position is still empty — so they are the
// only ones the walk has to remember (see cornerModule).
func (m *dmMapping) placed(row, col int) bool {
	i := m.rowOff[row] + m.colOff[col]
	return m.corner[i/64]&(1<<(i%64)) != 0
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
		m.setFixed(m.rows-1, m.cols-1, true)
		m.setFixed(m.rows-2, m.cols-2, true)
		m.setFixed(m.rows-1, m.cols-2, false)
		m.setFixed(m.rows-2, m.cols-1, false)
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

// module places bit (1 = MSB, 8 = LSB) of codeword c at row, col of the
// symbol, wrapping a negative coordinate back into the matrix as Annex F
// specifies. Recording the module layout is a separate method, recordBit,
// because a second sink branch here would push module past the inlining budget
// and cost about a third of the placement time.
func (m *dmMapping) module(row, col int, c byte, bit uint) {
	row, col = m.fold(row, col)
	m.out.mods[m.rowOff[row]+m.colOff[col]] = c>>(8-bit)&1 != 0
}

// recordBit stores the layout entry of the module at row, col: bit bit-1 of
// the codeword m.next returned last, bit 0 of a codeword being its most
// significant bit.
func (m *dmMapping) recordBit(row, col int, bit uint) {
	row, col = m.fold(row, col)
	m.layout[m.rowOff[row]+m.colOff[col]] = uint16(((m.used-1)*8 + int(bit) - 1) & 0xFFFF)
}

// fold wraps a negative mapping coordinate back into the matrix as Annex F
// specifies.
func (m *dmMapping) fold(row, col int) (int, int) {
	if row < 0 {
		row += m.rows
		col += 4 - (m.rows+4)%8
	}
	if col < 0 {
		col += m.cols
		row += 4 - (m.cols+4)%8
	}
	return row, col
}

// setFixed writes module row, col of the mapping matrix with a fixed value: a
// module that no codeword bit reaches, so it takes no bit of the layout.
func (m *dmMapping) setFixed(row, col int, dark bool) {
	i := m.rowOff[row] + m.colOff[col]
	if m.layout != nil {
		m.layout[i] = dmLayoutLight
		if dark {
			m.layout[i] = dmLayoutDark
		}
		return
	}
	m.out.mods[i] = dark
}

// cornerModule places bit (1 = MSB, 8 = LSB) of codeword c at row, col of a
// corner pattern, and records the module: the sweeps tile the mapping matrix
// around the corner patterns and meet them again only there, so these
// placements are all the walk has to skip later. Corner pattern coordinates
// are never negative, so the module is marked where it is placed.
func (m *dmMapping) cornerModule(row, col int, c byte, bit uint) {
	i := m.rowOff[row] + m.colOff[col]
	m.corner[i/64] |= 1 << (i % 64)
	if m.layout == nil {
		m.module(row, col, c, bit)
		return
	}
	m.recordBit(row, col, bit)
}

// utah places one codeword in the 8 module "utah" shape ending at row, col
// (ISO/IEC 16022 Annex F).
func (m *dmMapping) utah(row, col int, c byte) {
	if m.layout != nil {
		m.layoutUtah(row, col)
		return
	}
	m.module(row-2, col-2, c, 1)
	m.module(row-2, col-1, c, 2)
	m.module(row-1, col-2, c, 3)
	m.module(row-1, col-1, c, 4)
	m.module(row-1, col, c, 5)
	m.module(row, col-2, c, 6)
	m.module(row, col-1, c, 7)
	m.module(row, col, c, 8)
}

// layoutUtah records the module layout entries of the utah shape ending at
// row, col, module by module as utah places them.
func (m *dmMapping) layoutUtah(row, col int) {
	m.recordBit(row-2, col-2, 1)
	m.recordBit(row-2, col-1, 2)
	m.recordBit(row-1, col-2, 3)
	m.recordBit(row-1, col-1, 4)
	m.recordBit(row-1, col, 5)
	m.recordBit(row, col-2, 6)
	m.recordBit(row, col-1, 7)
	m.recordBit(row, col, 8)
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
