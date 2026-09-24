package symbol

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"sync"
)

// Image renders the matrix as a grayscale image, scale pixels per module with
// a quiet zone of quiet modules on every side. The image is
// (Cols+2*quiet)*scale pixels wide and (Rows+2*quiet)*scale pixels high. Dark
// modules are black (0) and the light modules and quiet zone are white (255).
// A scale below 1 is treated as 1 and a negative quiet zone as 0.
func (m *Matrix) Image(scale, quiet int) *image.Gray {
	scale, quiet = renderParams(scale, quiet)
	img := image.NewGray(image.Rect(0, 0, (m.Cols+2*quiet)*scale, (m.Rows+2*quiet)*scale))
	m.draw(img.Pix, img.Stride, scale, quiet, 0xFF, 0)
	return img
}

// PNG writes the matrix as a PNG image; see Image for the parameters.
func (m *Matrix) PNG(w io.Writer, scale, quiet int) error {
	return pngEncoder.Encode(w, m.pngImage(scale, quiet))
}

// pngEncoder writes every PNG and reuses its buffers through pngBufferPool.
// Encoding allocates far more scratch space than it produces: deflate alone
// keeps about 600 KiB of hash tables, so paying for them once per process
// instead of once per image is most of the cost of a small PNG.
var pngEncoder = png.Encoder{BufferPool: &pngBufferPool{
	pool: sync.Pool{New: func() any { return new(png.EncoderBuffer) }},
}}

// pngBufferPool is a png.EncoderBufferPool over a sync.Pool. A sync.Pool is
// safe for concurrent use and a buffer is only held by the pool or by one
// Encode call at a time, so the package-level pngEncoder is too. Encoder
// state that is not reset per call (the last image and writer) stays
// reachable from the pool until the next encode reuses the buffer.
type pngBufferPool struct{ pool sync.Pool }

// Get returns a buffer for one Encode call.
func (p *pngBufferPool) Get() *png.EncoderBuffer { return p.pool.Get().(*png.EncoderBuffer) }

// Put returns a buffer from a finished Encode call to the pool.
func (p *pngBufferPool) Put(b *png.EncoderBuffer) { p.pool.Put(b) }

// pngPalette holds the only two colors a rendered symbol uses: index pngWhite
// is the light module and the quiet zone, index pngBlack the dark module. Two
// entries are what make the PNG encoder write a bit depth 1 image, so filter
// and deflate see eight times fewer bytes than on the grayscale Image.
var pngPalette = color.Palette{color.Gray{Y: 0}, color.Gray{Y: 0xFF}}

const (
	pngBlack = 0
	pngWhite = 1
)

// pngImage renders the same picture as Image as a two-color paletted image.
func (m *Matrix) pngImage(scale, quiet int) *image.Paletted {
	scale, quiet = renderParams(scale, quiet)
	img := image.NewPaletted(image.Rect(0, 0, (m.Cols+2*quiet)*scale, (m.Rows+2*quiet)*scale), pngPalette)
	m.draw(img.Pix, img.Stride, scale, quiet, pngWhite, pngBlack)
	return img
}

// draw fills the pixmap pix, whose rows are stride bytes apart, with light and
// writes every dark module as a scale x scale rectangle of dark.
func (m *Matrix) draw(pix []byte, stride, scale, quiet int, light, dark byte) {
	for i := range pix {
		pix[i] = light
	}
	for row := 0; row < m.Rows; row++ {
		top := (row + quiet) * scale
		for col := 0; col < m.Cols; col++ {
			if !m.Dark(row, col) {
				continue
			}
			left := (col + quiet) * scale
			for y := top; y < top+scale; y++ {
				line := pix[y*stride+left : y*stride+left+scale]
				for i := range line {
					line[i] = dark
				}
			}
		}
	}
}

// SVG returns the matrix as a standalone SVG document; see Image for the
// parameters. One user unit equals one pixel at the given scale. The document
// is a white background rectangle and a single black path, one subpath per
// horizontal run of dark modules. A matrix with no dark modules yields an
// empty path data attribute, which draws nothing.
func (m *Matrix) SVG(scale, quiet int) string {
	scale, quiet = renderParams(scale, quiet)
	width := (m.Cols + 2*quiet) * scale
	height := (m.Rows + 2*quiet) * scale

	// One buffer for the whole document, sized up front so that it never
	// grows, then converted to a string in a single copy.
	b := make([]byte, 0, svgFixedCap+m.pathCap(width, height, scale))
	b = append(b, `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="`...)
	b = strconv.AppendInt(b, int64(width), 10)
	b = append(b, `" height="`...)
	b = strconv.AppendInt(b, int64(height), 10)
	b = append(b, `" viewBox="0 0 `...)
	b = strconv.AppendInt(b, int64(width), 10)
	b = append(b, ' ')
	b = strconv.AppendInt(b, int64(height), 10)
	b = append(b, `" shape-rendering="crispEdges">`...)
	b = append(b, '\n')
	b = append(b, `<rect width="100%" height="100%" fill="#fff"/>`...)
	b = append(b, '\n')
	b = append(b, `<path fill="#000" d="`...)
	b = m.appendPath(b, scale, quiet)
	b = append(b, "\"/>\n</svg>\n"...)
	return string(b)
}

// svgFixedCap is an upper bound on the bytes of the document that are not
// path data: the fixed text and the four numbers of the header. Reserving a
// few bytes too many costs nothing; a too small number only makes the buffer
// grow once more.
const svgFixedCap = 256

// pathCap returns an upper bound on the length of the path data of a symbol
// rendered at the given pixel width, height and scale: one subpath per dark
// run, whose numbers are all at most as wide as the symbol.
func (m *Matrix) pathCap(width, height, scale int) int {
	const syntax = 8 // 'M', ' ', 'h', 'v', "h-", 'z'
	return m.darkRuns() * (syntax + 3*intDigits(width) + intDigits(height) + intDigits(scale))
}

// darkRuns returns the number of horizontal dark runs, which is the number of
// subpaths appendPath emits.
func (m *Matrix) darkRuns() int {
	runs := 0
	for row := 0; row < m.Rows; row++ {
		base := row * m.Cols
		for col := 0; col < m.Cols; {
			if !m.mods[base+col] {
				col++
				continue
			}
			runs++
			for col < m.Cols && m.mods[base+col] {
				col++
			}
		}
	}
	return runs
}

// intDigits returns the number of decimal digits of n, which must not be
// negative.
func intDigits(n int) int {
	digits := 1
	for n >= 10 {
		n /= 10
		digits++
	}
	return digits
}

// appendPath appends the path data of the dark modules: one subpath per
// horizontal run, in pixel units.
func (m *Matrix) appendPath(b []byte, scale, quiet int) []byte {
	for row := 0; row < m.Rows; row++ {
		y := (row + quiet) * scale
		for col := 0; col < m.Cols; col++ {
			if !m.Dark(row, col) {
				continue
			}
			// Dark reports light past the last column, so runs end at the edge.
			run := 1
			for m.Dark(row, col+run) {
				run++
			}
			x := (col + quiet) * scale
			runWidth := run * scale
			b = append(b, 'M')
			b = strconv.AppendInt(b, int64(x), 10)
			b = append(b, ' ')
			b = strconv.AppendInt(b, int64(y), 10)
			b = append(b, 'h')
			b = strconv.AppendInt(b, int64(runWidth), 10)
			b = append(b, 'v')
			b = strconv.AppendInt(b, int64(scale), 10)
			b = append(b, 'h', '-')
			b = strconv.AppendInt(b, int64(runWidth), 10)
			b = append(b, 'z')
			col += run - 1
		}
	}
	return b
}

// renderParams clamps scale and quiet to the values Image, PNG and SVG accept.
func renderParams(scale, quiet int) (int, int) {
	if scale < 1 {
		scale = 1
	}
	if quiet < 0 {
		quiet = 0
	}
	return scale, quiet
}
