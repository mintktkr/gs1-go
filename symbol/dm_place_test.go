package symbol

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// TestDMPlaceGolden10x10 checks a whole 10x10 symbol against zint's dump of
// "123456": its 3 data codewords (ASCII digit pairs) followed by the 5 ECC
// codewords of that size.
func TestDMPlaceGolden10x10(t *testing.T) {
	cw := []byte{142, 164, 186, 114, 25, 5, 88, 102}
	// zint -b DATAMATRIX --vers=1 --dump -d 123456
	dump := `AA 8
CB 4
C1 0
C7 4
C2 0
83 C
EC 0
F6 4
9D 0
FF C`
	s := dmSizes[0]
	want := parseDump(t, dump, s.Rows, s.Cols)
	got := dmPlace(cw, s)
	for r := 0; r < s.Rows; r++ {
		for c := 0; c < s.Cols; c++ {
			if got.Dark(r, c) != want[r][c] {
				t.Fatalf("module (%d,%d) = %v, zint has %v", r, c, got.Dark(r, c), want[r][c])
			}
		}
	}
}

// TestDMPlaceFinderAllSizes checks the finder and alignment modules of all 30
// sizes. A region's modules do not depend on the data, so the expected
// pattern follows from the region geometry alone; the zint dumps in
// TestDMPlaceFinderLiterals and TestDMPlaceGolden10x10 anchor that geometry.
func TestDMPlaceFinderAllSizes(t *testing.T) {
	for i, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			got := dmPlace(testCW(s.DataCW+s.ECCCW), s)
			rows, cols := s.RegionRows+2, s.RegionCols+2
			for r := 0; r < s.Rows; r++ {
				for c := 0; c < s.Cols; c++ {
					if dataModule(s, r, c) {
						continue
					}
					rr, cc := r%rows, c%cols
					if got.Dark(r, c) != finderModule(rr, cc, rows, cols) {
						t.Errorf("vers %d: module (%d,%d) = %v, want %v",
							i+1, r, c, got.Dark(r, c), finderModule(rr, cc, rows, cols))
					}
				}
			}
		})
	}
}

// TestDMPlaceFinderLiterals checks the finder and alignment modules of five
// sizes against module values taken from zint, hex encoded (four modules per
// digit, most significant first) in row-major order over the border modules
// only.
func TestDMPlaceFinderLiterals(t *testing.T) {
	cases := []struct {
		vers       int
		rows, cols int
		borderHex  string
	}{
		// zint -b DATAMATRIX --vers=10 --dump -d 1
		{10, 32, 32, "aaaaaaaafafafafafafafaffffffffaaaaaaaafafafafafafafaffffffff"},
		// zint -b DATAMATRIX --vers=16 --dump -d 1
		{16, 64, 64,
			"aaaaaaaaaaaaaaaaffaaffaaffaaffaaffaaffaaffaaffffffffffffffffaaaa" +
				"aaaaaaaaaaaaffaaffaaffaaffaaffaaffaaffaaffffffffffffffffaaaaaaaa" +
				"aaaaaaaaffaaffaaffaaffaaffaaffaaffaaffffffffffffffffaaaaaaaaaaaa" +
				"aaaaffaaffaaffaaffaaffaaffaaffaaffffffffffffffff"},
		// zint -b DATAMATRIX --vers=24 --dump -d 1
		{24, 144, 144,
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaafffaaafffaaafffaaafffaaafffa" +
				"aafffaaafffaaafffaaafffaaafffaaafffaaaffffffffffffffffffffffffff" +
				"ffffffffffaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaafffaaafffaaafffaaa" +
				"fffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaaffffffffffffffff" +
				"ffffffffffffffffffffaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaafffaaaff" +
				"faaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaaffffff" +
				"ffffffffffffffffffffffffffffffaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
				"aafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaaff" +
				"faaaffffffffffffffffffffffffffffffffffffaaaaaaaaaaaaaaaaaaaaaaaa" +
				"aaaaaaaaaaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaafffa" +
				"aafffaaafffaaaffffffffffffffffffffffffffffffffffffaaaaaaaaaaaaaa" +
				"aaaaaaaaaaaaaaaaaaaaaafffaaafffaaafffaaafffaaafffaaafffaaafffaaa" +
				"fffaaafffaaafffaaafffaaaffffffffffffffffffffffffffffffffffff"},
		// zint -b DATAMATRIX --vers=26 --dump -d 1
		{26, 8, 32, "aaaaaaaafafafaffffffff"},
		// zint -b DATAMATRIX --vers=30 --dump -d 1
		{30, 16, 48, "aaaaaaaaaaaafafafafafafafaffffffffffff"},
	}
	for _, tc := range cases {
		t.Run(sizeName(dmSizes[tc.vers-1]), func(t *testing.T) {
			s := dmSizes[tc.vers-1]
			if s.Rows != tc.rows || s.Cols != tc.cols {
				t.Fatalf("vers %d is %dx%d, not %dx%d", tc.vers, s.Rows, s.Cols, tc.rows, tc.cols)
			}
			got := dmPlace(testCW(s.DataCW+s.ECCCW), s)
			want := decodeModules(t, tc.borderHex)
			i := 0
			for r := 0; r < s.Rows; r++ {
				for c := 0; c < s.Cols; c++ {
					if dataModule(s, r, c) {
						continue
					}
					if i == len(want) {
						t.Fatalf("vers %d: more border modules than the %d in the literal", tc.vers, len(want))
					}
					if got.Dark(r, c) != want[i] {
						t.Errorf("vers %d: border module (%d,%d) = %v, zint has %v", tc.vers, r, c, got.Dark(r, c), want[i])
					}
					i++
				}
			}
			if i != len(want) {
				t.Errorf("vers %d: %d border modules, literal has %d", tc.vers, i, len(want))
			}
		})
	}
}

// TestDMPlaceCoverage checks that the placement fills every module of the
// mapping matrix, and that it consumes exactly the codewords it was given.
// With an all-ones codeword stream every codeword bit is dark, so a module the
// walk never reaches stays light; together with the write budget — 8 modules
// per codeword, plus the four fixed modules of a trailing group — filling the
// whole matrix leaves no room for a module written twice.
func TestDMPlaceCoverage(t *testing.T) {
	for i, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			cw := make([]byte, s.DataCW+s.ECCCW)
			for j := range cw {
				cw[j] = 0xFF
			}
			m := newDMMapping(newMatrix(s.Rows, s.Cols), s)
			m.fill(cw)
			if m.used != len(cw) {
				t.Errorf("vers %d: consumed %d codewords, want %d", i+1, m.used, len(cw))
			}
			rows, cols := s.mappingSize()
			if n := 8 * len(cw); n != rows*cols && n+4 != rows*cols {
				t.Errorf("vers %d: %d codewords hold %d module writes, matrix has %d modules",
					i+1, len(cw), n, rows*cols)
			}
			// A matrix that is not a whole number of codewords ends with the
			// four fixed modules, two of which are light; skip them, as they
			// carry no codeword bit.
			fixed := 8*len(cw) != rows*cols
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					if fixed && r >= rows-2 && c >= cols-2 {
						continue
					}
					got := m.out.Dark(r+2*(r/s.RegionRows)+1, c+2*(c/s.RegionCols)+1)
					if !got {
						t.Fatalf("vers %d: mapping module (%d,%d) was never placed", i+1, r, c)
					}
				}
			}
		})
	}
}

// TestDMPlaceLayout checks that the cached layout of every size, applied to a
// codeword stream, reproduces the direct placement module for module. The
// all-ones stream repeats the coverage argument of TestDMPlaceCoverage for the
// layout: a data module without a layout entry stays light, and the write
// budget leaves no room for a module carrying two entries.
func TestDMPlaceLayout(t *testing.T) {
	for _, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			tab := buildDMLayout(s)
			n := s.DataCW + s.ECCCW
			ones := make([]byte, n)
			for i := range ones {
				ones[i] = 0xFF
			}
			rng := rand.New(rand.NewSource(int64(s.Rows)))
			rngCW := make([]byte, n)
			for i := range rngCW {
				rngCW[i] = byte(rng.Intn(256))
			}
			for i, cw := range [][]byte{testCW(n), ones, make([]byte, n), rngCW} {
				compareMatrix(t, sizeName(s)+" stream "+strconv.Itoa(i), dmPlaceLayout(tab, cw, s), dmPlace(cw, s))
			}
		})
	}
}

// compareMatrix fails the test at the first module where got and want differ.
func compareMatrix(t *testing.T, name string, got, want *Matrix) {
	t.Helper()
	if got.Rows != want.Rows || got.Cols != want.Cols {
		t.Fatalf("%s: got %dx%d, want %dx%d", name, got.Rows, got.Cols, want.Rows, want.Cols)
	}
	for r := 0; r < want.Rows; r++ {
		for c := 0; c < want.Cols; c++ {
			if got.Dark(r, c) != want.Dark(r, c) {
				t.Fatalf("%s: module (%d,%d) = %v, want %v", name, r, c, got.Dark(r, c), want.Dark(r, c))
			}
		}
	}
}

// dataModule reports whether symbol module (r, c) of s carries a mapping
// matrix module rather than finder or alignment pattern.
func dataModule(s dmSize, r, c int) bool {
	rr := r % (s.RegionRows + 2)
	cc := c % (s.RegionCols + 2)
	return rr > 0 && rr <= s.RegionRows && cc > 0 && cc <= s.RegionCols
}

// finderModule is the expected finder or alignment module at offset rr, cc of
// a region of rows x cols modules: solid left column and bottom row, and
// clock tracks of every other module dark on the top row (starting dark) and
// the right column (starting light).
func finderModule(rr, cc, rows, cols int) bool {
	if cc == 0 || rr == rows-1 {
		return true
	}
	if rr == 0 {
		return cc%2 == 0
	}
	if cc == cols-1 {
		return rr%2 == 1
	}
	return false
}

// parseDump parses zint --dump output: one symbol row per line, four modules
// per hex digit, most significant first; the last digit of a row may carry
// fewer than four modules.
func parseDump(t *testing.T, dump string, rows, cols int) [][]bool {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) != rows {
		t.Fatalf("dump has %d rows, want %d", len(lines), rows)
	}
	out := make([][]bool, rows)
	for i, line := range lines {
		bits := decodeModules(t, strings.ReplaceAll(line, " ", ""))
		if len(bits) < cols {
			t.Fatalf("dump row %d has %d modules, want %d", i, len(bits), cols)
		}
		out[i] = bits[:cols]
	}
	return out
}

// decodeModules decodes module values written as hex, four modules per digit,
// most significant first.
func decodeModules(t *testing.T, h string) []bool {
	t.Helper()
	bits := make([]bool, 0, 4*len(h))
	for i := 0; i < len(h); i++ {
		v, err := strconv.ParseUint(h[i:i+1], 16, 8)
		if err != nil {
			t.Fatalf("bad hex digit %q", h[i])
		}
		for b := 3; b >= 0; b-- {
			bits = append(bits, v>>uint(b)&1 == 1)
		}
	}
	return bits
}

// testCW returns a codeword stream of n bytes that is not all equal, so that
// transposed or shifted module writes show up.
func testCW(n int) []byte {
	cw := make([]byte, n)
	for i := range cw {
		cw[i] = byte(i*7 + 1)
	}
	return cw
}

func sizeName(s dmSize) string {
	return fmt.Sprintf("%dx%d", s.Rows, s.Cols)
}
