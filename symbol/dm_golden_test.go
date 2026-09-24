package symbol

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// goldenCase is one symbol from testdata/datamatrix-golden.txt, produced by
// zint. See testdata/gen-golden.sh.
type goldenCase struct {
	kind    string // "plain" (forced size) or "gs1" (automatic size)
	version int    // zint --vers: index into dmSizes plus one
	data    string
	rows    []string // hex dump rows
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	data, err := os.ReadFile("testdata/datamatrix-golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "case "):
			fields := strings.SplitN(line, " ", 4)
			v, err := strconv.Atoi(fields[2])
			if err != nil {
				t.Fatal(err)
			}
			cases = append(cases, goldenCase{kind: fields[1], version: v, data: fields[3]})
		default:
			c := &cases[len(cases)-1]
			c.rows = append(c.rows, strings.ReplaceAll(line, " ", ""))
		}
	}
	return cases
}

// compareDump reports every module where m differs from a zint hex dump.
func compareDump(t *testing.T, m *Matrix, rows []string) {
	t.Helper()
	if m.Rows != len(rows) {
		t.Fatalf("rows = %d, want %d", m.Rows, len(rows))
	}
	bad := 0
	for r, hex := range rows {
		if m.Cols > len(hex)*4 {
			t.Fatalf("row %d: dump has %d bits, want %d", r, len(hex)*4, m.Cols)
		}
		for c := 0; c < m.Cols; c++ {
			nib, err := strconv.ParseUint(hex[c/4:c/4+1], 16, 8)
			if err != nil {
				t.Fatal(err)
			}
			want := nib&(8>>(c%4)) != 0
			if m.Dark(r, c) != want {
				if bad < 5 {
					t.Errorf("module (%d,%d) dark=%v, want %v", r, c, m.Dark(r, c), want)
				}
				bad++
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d modules differ", bad, m.Rows*m.Cols)
	}
}

func TestDataMatrixGolden(t *testing.T) {
	for _, gc := range loadGolden(t) {
		name := gc.kind + "/" + strconv.Itoa(gc.version)
		if gc.kind == "gs1" {
			name = gc.kind + "/" + gc.data
		}
		t.Run(name, func(t *testing.T) {
			var m *Matrix
			switch gc.kind {
			case "plain":
				s := dmSizes[gc.version-1]
				cw, err := dmEncodeASCII(gc.data, false)
				if err != nil {
					t.Fatal(err)
				}
				m = dmPlace(dmECC(dmPad(cw, s.DataCW), s), s)
			case "gs1":
				in := strings.NewReplacer("[", "(", "]", ")").Replace(gc.data)
				var err error
				m, err = GS1DataMatrix(in, DataMatrixOptions{})
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unknown case kind %q", gc.kind)
			}
			compareDump(t, m, gc.rows)
		})
	}
}
