package symbol

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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
