package symbol

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
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
// mapping matrix exactly once, and that it consumes exactly the codewords it
// was given.
func TestDMPlaceCoverage(t *testing.T) {
	for i, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			cw := testCW(s.DataCW + s.ECCCW)
			m := dmMappingOf(cw, s)
			if m.used != len(cw) {
				t.Errorf("vers %d: consumed %d codewords, want %d", i+1, m.used, len(cw))
			}
			rows, cols := s.mappingSize()
			if m.writes != rows*cols {
				t.Errorf("vers %d: %d module writes for %d modules", i+1, m.writes, rows*cols)
			}
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					if !m.placed(r, c) {
						t.Fatalf("vers %d: mapping module (%d,%d) was never placed", i+1, r, c)
					}
				}
			}
		})
	}
}

// TestDMPlaceTableAllSizes checks the placement table of all 30 sizes against
// the Annex F walk it is built from: every mapping module decodes to the value
// the walk gave it, every codeword bit is placed exactly once, and every
// module outside the mapping matrix follows the finder pattern.
func TestDMPlaceTableAllSizes(t *testing.T) {
	for i, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			tab := dmPlaceTable(s)
			if len(tab) != s.Rows*s.Cols {
				t.Fatalf("vers %d: table has %d entries, want %d", i+1, len(tab), s.Rows*s.Cols)
			}
			cw := testCW(s.DataCW + s.ECCCW)
			m := dmMappingOf(cw, s)
			mrows, mcols := s.mappingSize()

			seen := make([]bool, len(cw)*8)
			for mr := 0; mr < mrows; mr++ {
				for mc := 0; mc < mcols; mc++ {
					r := mr/s.RegionRows*(s.RegionRows+2) + 1 + mr%s.RegionRows
					c := mc/s.RegionCols*(s.RegionCols+2) + 1 + mc%s.RegionCols
					e := tab[r*s.Cols+c]
					var got int
					switch e {
					case dmPlaceDark:
						got = 1
					case dmPlaceLight:
						got = 0
					default:
						got = int(cw[e>>3] >> (7 - (e & 7)) & 1)
						if seen[e] {
							t.Errorf("vers %d: codeword %d bit %d placed twice", i+1, e>>3, e&7)
						}
						seen[e] = true
					}
					if got != int(m.get(mr, mc)) {
						t.Errorf("vers %d: mapping module (%d,%d) is %d, the table says %d",
							i+1, mr, mc, m.get(mr, mc), got)
					}
				}
			}
			for e, used := range seen {
				if !used {
					t.Errorf("vers %d: codeword %d bit %d was never placed", i+1, e>>3, e&7)
				}
			}

			for r := 0; r < s.Rows; r++ {
				for c := 0; c < s.Cols; c++ {
					if dataModule(s, r, c) {
						continue
					}
					rr, cc := r%(s.RegionRows+2), c%(s.RegionCols+2)
					want := finderModule(rr, cc, s.RegionRows+2, s.RegionCols+2)
					if got := tab[r*s.Cols+c] == dmPlaceDark; got != want {
						t.Errorf("vers %d: module (%d,%d) = %v, want finder %v", i+1, r, c, got, want)
					}
				}
			}
		})
	}
}

// TestDMPlaceTableConcurrent checks concurrent first use of the tables: after
// clearing the cache, several encoders build and read all of them at once, and
// must still see the same modules as a single-threaded run does.
func TestDMPlaceTableConcurrent(t *testing.T) {
	want := make([]*Matrix, len(dmSizes))
	for i, s := range dmSizes {
		want[i] = dmPlace(testCW(s.DataCW+s.ECCCW), s)
	}
	dmPlaceTablesMu.Lock()
	dmPlaceTables = map[dmSize][]uint16{}
	dmPlaceTablesMu.Unlock()

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, s := range dmSizes {
				got := dmPlace(testCW(s.DataCW+s.ECCCW), s)
				for r := 0; r < s.Rows; r++ {
					for c := 0; c < s.Cols; c++ {
						if got.Dark(r, c) != want[i].Dark(r, c) {
							t.Errorf("vers %d: module (%d,%d) = %v, want %v", i+1, r, c, got.Dark(r, c), want[i].Dark(r, c))
							return
						}
					}
				}
			}
		}()
	}
	wg.Wait()
}

// BenchmarkPlaceTableBuild measures the first-use cost of a placement table,
// paid once per size: the largest symbol, 144x144.
func BenchmarkPlaceTableBuild(b *testing.B) {
	s := dmSizes[0]
	for _, c := range dmSizes {
		if c.Rows*c.Cols > s.Rows*s.Cols {
			s = c
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buildDMPlaceTable(s)
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
