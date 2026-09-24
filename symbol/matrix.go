package symbol

// Matrix is a two-dimensional grid of modules. Row 0 is the top of the
// symbol and column 0 the left edge. The quiet zone is not included.
type Matrix struct {
	Rows, Cols int
	mods       []bool
}

func newMatrix(rows, cols int) *Matrix {
	return &Matrix{Rows: rows, Cols: cols, mods: make([]bool, rows*cols)}
}

// Dark reports whether the module at row, col is dark. Coordinates outside
// the matrix are light, which matches the quiet zone around a symbol.
func (m *Matrix) Dark(row, col int) bool {
	if row < 0 || col < 0 || row >= m.Rows || col >= m.Cols {
		return false
	}
	return m.mods[row*m.Cols+col]
}

func (m *Matrix) set(row, col int, dark bool) {
	m.mods[row*m.Cols+col] = dark
}
