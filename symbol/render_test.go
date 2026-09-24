package symbol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
)

// renderMatrix builds a matrix from rows of '#' (dark) and '.' (light).
func renderMatrix(rows ...string) *Matrix {
	m := newMatrix(len(rows), len(rows[0]))
	for r, row := range rows {
		for c, ch := range row {
			m.set(r, c, ch == '#')
		}
	}
	return m
}

func TestRenderImage(t *testing.T) {
	tests := []struct {
		name  string
		rows  []string
		scale int
		quiet int
		want  []string
	}{
		{
			name:  "runs and single module",
			rows:  []string{"##.", "..#"},
			scale: 2,
			quiet: 1,
			want: []string{
				"..........",
				"..........",
				"..####....",
				"..####....",
				"......##..",
				"......##..",
				"..........",
				"..........",
			},
		},
		{
			name:  "single module scaled on both axes",
			rows:  []string{"#"},
			scale: 3,
			quiet: 1,
			want: []string{
				".........",
				".........",
				".........",
				"...###...",
				"...###...",
				"...###...",
				".........",
				".........",
				".........",
			},
		},
		{
			name:  "no dark modules and no quiet zone",
			rows:  []string{"...", "..."},
			scale: 1,
			quiet: 0,
			want:  []string{"...", "..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := renderMatrix(tt.rows...).Image(tt.scale, tt.quiet)
			if got, want := img.Bounds().Dx(), len(tt.want[0]); got != want {
				t.Errorf("width = %d, want %d", got, want)
			}
			if got, want := img.Bounds().Dy(), len(tt.want); got != want {
				t.Errorf("height = %d, want %d", got, want)
			}
			for y, row := range tt.want {
				for x, ch := range row {
					want := uint8(255)
					if ch == '#' {
						want = 0
					}
					if got := img.GrayAt(x, y).Y; got != want {
						t.Errorf("pixel (%d, %d) = %d, want %d", x, y, got, want)
					}
				}
			}
		})
	}
}

func TestRenderImageQuietZone(t *testing.T) {
	const scale, quiet = 2, 2
	img := renderMatrix("###", "###").Image(scale, quiet)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			inSymbol := x >= quiet*scale && x < (3+quiet)*scale && y >= quiet*scale && y < (2+quiet)*scale
			want := uint8(255)
			if inSymbol {
				want = 0
			}
			if got := img.GrayAt(x, y).Y; got != want {
				t.Errorf("pixel (%d, %d) = %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestRenderImageClampsParameters(t *testing.T) {
	m := renderMatrix("##.", "..#")
	got := m.Image(0, -1)
	want := m.Image(1, 0)
	if want.Bounds() != image.Rect(0, 0, 3, 2) {
		t.Fatalf("Image(1, 0) bounds = %v, want 3x2", want.Bounds())
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("Image(0, -1) bounds = %v, want %v", got.Bounds(), want.Bounds())
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Errorf("Image(0, -1) pixels differ from Image(1, 0)")
	}
}

func TestRenderPNG(t *testing.T) {
	m := renderMatrix("##.", "..#", ".#.")
	var buf bytes.Buffer
	if err := m.PNG(&buf, 3, 2); err != nil {
		t.Fatalf("PNG() error = %v", err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	want := m.Image(3, 2)
	if img.Bounds() != want.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", img.Bounds(), want.Bounds())
	}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			got := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y
			if wantY := want.GrayAt(x, y).Y; got != wantY {
				t.Errorf("pixel (%d, %d) = %d, want %d", x, y, got, wantY)
			}
		}
	}
}

// TestRenderSVGLarge checks the SVG of a real 144x144 symbol against hashes
// recorded from the pre-optimization writer: the output of a full-size symbol
// must stay byte for byte the same, multi-digit coordinates included.
func TestRenderSVGLarge(t *testing.T) {
	m, err := GS1DataMatrix(benchLarge, DataMatrixOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Rows != 144 || m.Cols != 144 {
		t.Fatalf("symbol is %dx%d, want 144x144", m.Rows, m.Cols)
	}
	tests := []struct {
		scale, quiet int
		want         string
	}{
		{10, 1, "acc1ed9b37e23465a9d16f21a120831dce2e5f5ece3b258bb028a25b53161101"},
		{1, 0, "7e9fa4b788b6a1e32b1f5a3d600110be4e78ecaba8b222a656d24abbd18c44f1"},
	}
	for _, tt := range tests {
		sum := sha256.Sum256([]byte(m.SVG(tt.scale, tt.quiet)))
		if got := hex.EncodeToString(sum[:]); got != tt.want {
			t.Errorf("SVG(%d, %d) sha256 = %s, want %s", tt.scale, tt.quiet, got, tt.want)
		}
	}
	// One allocation for the document buffer and one for the string: no
	// intermediate string per number, and the buffer never grows.
	if allocs := testing.AllocsPerRun(3, func() { _ = m.SVG(10, 1) }); allocs > 2 {
		t.Errorf("SVG(10, 1) allocates %.0f times per run, want at most 2", allocs)
	}
}

func TestRenderSVG(t *testing.T) {
	tests := []struct {
		name  string
		rows  []string
		scale int
		quiet int
		want  string
	}{
		{
			name:  "run of two and a single module",
			rows:  []string{"##.", "..#"},
			scale: 3,
			quiet: 1,
			want: `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="15" height="12" viewBox="0 0 15 12" shape-rendering="crispEdges">
<rect width="100%" height="100%" fill="#fff"/>
<path fill="#000" d="M3 3h6v3h-6zM9 6h3v3h-3z"/>
</svg>
`,
		},
		{
			name:  "run reaches the right edge",
			rows:  []string{"###"},
			scale: 1,
			quiet: 0,
			want: `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="3" height="1" viewBox="0 0 3 1" shape-rendering="crispEdges">
<rect width="100%" height="100%" fill="#fff"/>
<path fill="#000" d="M0 0h3v1h-3z"/>
</svg>
`,
		},
		{
			name:  "scale 0 and quiet -1 are clamped",
			rows:  []string{"#"},
			scale: 0,
			quiet: -1,
			want: `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="1" height="1" viewBox="0 0 1 1" shape-rendering="crispEdges">
<rect width="100%" height="100%" fill="#fff"/>
<path fill="#000" d="M0 0h1v1h-1z"/>
</svg>
`,
		},
		{
			name:  "no dark modules leaves the path data empty",
			rows:  []string{"...", "..."},
			scale: 2,
			quiet: 1,
			want: `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="10" height="8" viewBox="0 0 10 8" shape-rendering="crispEdges">
<rect width="100%" height="100%" fill="#fff"/>
<path fill="#000" d=""/>
</svg>
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderMatrix(tt.rows...).SVG(tt.scale, tt.quiet); got != tt.want {
				t.Errorf("SVG() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestRenderPNGConcurrent(t *testing.T) {
	// PNG shares one encoder and its buffer pool across goroutines; every
	// call must still return the same bytes.
	m := renderMatrix("##.", "..#", ".#.")
	var want bytes.Buffer
	if err := m.PNG(&want, 3, 2); err != nil {
		t.Fatalf("PNG() error = %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				var got bytes.Buffer
				if err := m.PNG(&got, 3, 2); err != nil {
					t.Errorf("PNG() error = %v", err)
					return
				}
				if !bytes.Equal(got.Bytes(), want.Bytes()) {
					t.Errorf("concurrent PNG() returned %d bytes, want %d", got.Len(), want.Len())
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestRenderPaletted checks that Paletted draws the same picture as Image and
// that it encodes as a 1-bit PNG that decodes to the same pixels.
func TestRenderPaletted(t *testing.T) {
	m, err := GS1DataMatrix(benchSmall, DataMatrixOptions{})
	if err != nil {
		t.Fatal(err)
	}
	gray := m.Image(3, 2)
	pal := m.Paletted(3, 2)
	if pal.Bounds() != gray.Bounds() {
		t.Fatalf("bounds %v, want %v", pal.Bounds(), gray.Bounds())
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, pal); err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.ColorModel.(color.Palette); !ok {
		t.Errorf("PNG color model %T, want a palette", cfg.ColorModel)
	}
	dec, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	b := gray.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			want := gray.GrayAt(x, y).Y
			if got := color.GrayModel.Convert(pal.At(x, y)).(color.Gray).Y; got != want {
				t.Fatalf("Paletted pixel (%d,%d) = %d, want %d", x, y, got, want)
			}
			if got := color.GrayModel.Convert(dec.At(x, y)).(color.Gray).Y; got != want {
				t.Fatalf("decoded pixel (%d,%d) = %d, want %d", x, y, got, want)
			}
		}
	}
}
