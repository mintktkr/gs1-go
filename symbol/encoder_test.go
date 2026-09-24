package symbol

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestEncoderGolden runs every zint golden symbol through the cached-layout
// path as well as the direct one, so both are verified module for module
// against zint.
func TestEncoderGolden(t *testing.T) {
	var e Encoder
	for _, gc := range loadGolden(t) {
		name := gc.kind + "/" + strconv.Itoa(gc.version)
		if gc.kind == "gs1" {
			name = gc.kind + "/" + gc.data
		}
		t.Run(name, func(t *testing.T) {
			switch gc.kind {
			case "plain":
				// The case forces a symbol size, so encode where that size is
				// known instead of through the exported API, which picks one.
				s := dmSizes[gc.version-1]
				cw, err := dmEncodeASCII(gc.data, false)
				if err != nil {
					t.Fatal(err)
				}
				full := dmECC(dmPad(cw, s.DataCW), s)
				compareDump(t, dmPlace(full, s), gc.rows)
				compareDump(t, dmPlaceLayout(e.layout(s), full, s), gc.rows)
			case "gs1":
				in := strings.NewReplacer("[", "(", "]", ")").Replace(gc.data)
				want, err := GS1DataMatrix(in, DataMatrixOptions{})
				if err != nil {
					t.Fatal(err)
				}
				compareDump(t, want, gc.rows)
				got, err := e.GS1DataMatrix(in, DataMatrixOptions{})
				if err != nil {
					t.Fatal(err)
				}
				compareDump(t, got, gc.rows)
			default:
				t.Fatalf("unknown case kind %q", gc.kind)
			}
		})
	}
}

// TestEncoderAllSizes encodes one codeword stream per size, so every cached
// layout is looked up and used. dmSelectSize picks the requested size for a
// stream of exactly its data capacity, which the public API cannot express.
func TestEncoderAllSizes(t *testing.T) {
	var e Encoder
	for _, s := range dmSizes {
		t.Run(sizeName(s), func(t *testing.T) {
			opts := DataMatrixOptions{Rectangular: s.Rows != s.Cols}
			cw := testCW(s.DataCW)
			want, err := encodeDataMatrix(cw, opts, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want.Rows != s.Rows || want.Cols != s.Cols {
				t.Fatalf("direct path chose %dx%d", want.Rows, want.Cols)
			}
			got, err := encodeDataMatrix(cw, opts, &e)
			if err != nil {
				t.Fatal(err)
			}
			compareMatrix(t, sizeName(s), got, want)
		})
	}
}

// TestEncoderConcurrent encodes with one zero-value Encoder from many
// goroutines at once, so that the first build of every layout races with the
// others. Run with -race. Every result must match the package-level function.
func TestEncoderConcurrent(t *testing.T) {
	inputs := []struct {
		in   string
		opts DataMatrixOptions
	}{
		{"(01)04150000021126(17)250630(10)ABC123", DataMatrixOptions{}},
		{"(01)04150000021126(10)ABC", DataMatrixOptions{Rectangular: true}},
		{"(01)04150000021126" + strings.Repeat("(91)ABC123", 200), DataMatrixOptions{}},
	}
	want := make([]*Matrix, len(inputs))
	for i, in := range inputs {
		m, err := GS1DataMatrix(in.in, in.opts)
		if err != nil {
			t.Fatal(err)
		}
		want[i] = m
	}

	const rounds = 3
	var e Encoder
	results := make([][]*Matrix, 8)
	errs := make(chan error, len(results))
	var wg sync.WaitGroup
	for g := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out []*Matrix
			for range rounds {
				for _, in := range inputs {
					m, err := e.GS1DataMatrix(in.in, in.opts)
					if err != nil {
						errs <- err
						return
					}
					out = append(out, m)
				}
			}
			results[g] = out
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent encode: %v", err)
	}
	for g, out := range results {
		if len(out) != rounds*len(inputs) {
			t.Fatalf("goroutine %d returned %d symbols, want %d", g, len(out), rounds*len(inputs))
		}
		for i, m := range out {
			compareMatrix(t, fmt.Sprintf("goroutine %d symbol %d", g, i), m, want[i%len(inputs)])
		}
	}
}

// TestEncoderErrors checks that an Encoder fails exactly like the package-level
// functions.
func TestEncoderErrors(t *testing.T) {
	cases := []struct {
		name string
		gs1  bool
		in   string
		opts DataMatrixOptions
	}{
		{"unparsable gs1", true, "not an element string", DataMatrixOptions{}},
		{"plain too long", false, strings.Repeat("A", 4000), DataMatrixOptions{}},
		{"gs1 rectangular too long", true, "(91)" + strings.Repeat("A", 200), DataMatrixOptions{Rectangular: true}},
	}
	var e Encoder
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var want, got error
			if tc.gs1 {
				_, want = GS1DataMatrix(tc.in, tc.opts)
				_, got = e.GS1DataMatrix(tc.in, tc.opts)
			} else {
				_, want = DataMatrix(tc.in, tc.opts)
				_, got = e.DataMatrix(tc.in, tc.opts)
			}
			if want == nil {
				t.Fatalf("package function accepted %q", tc.in)
			}
			if got == nil || got.Error() != want.Error() {
				t.Fatalf("err = %v, want %v", got, want)
			}
		})
	}
	if _, err := e.DataMatrix(strings.Repeat("A", 4000), DataMatrixOptions{}); !errors.Is(err, ErrDataTooLong) {
		t.Errorf("err = %v, want ErrDataTooLong", err)
	}
}
