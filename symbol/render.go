package symbol

import (
	"image"
	"image/png"
	"io"
	"strconv"
	"strings"
)

// Image renders the matrix as a grayscale image, scale pixels per module with
// a quiet zone of quiet modules on every side. The image is
// (Cols+2*quiet)*scale pixels wide and (Rows+2*quiet)*scale pixels high. Dark
// modules are black (0) and the light modules and quiet zone are white (255).
// A scale below 1 is treated as 1 and a negative quiet zone as 0.
func (m *Matrix) Image(scale, quiet int) *image.Gray {
	scale, quiet = renderParams(scale, quiet)
	img := image.NewGray(image.Rect(0, 0, (m.Cols+2*quiet)*scale, (m.Rows+2*quiet)*scale))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	for row := 0; row < m.Rows; row++ {
		top := (row + quiet) * scale
		for col := 0; col < m.Cols; col++ {
			if !m.Dark(row, col) {
				continue
			}
			left := (col + quiet) * scale
			for y := top; y < top+scale; y++ {
				clear(img.Pix[y*img.Stride+left : y*img.Stride+left+scale])
			}
		}
	}
	return img
}

// PNG writes the matrix as a PNG image; see Image for the parameters.
func (m *Matrix) PNG(w io.Writer, scale, quiet int) error {
	return png.Encode(w, m.Image(scale, quiet))
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

	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="`)
	b.WriteString(strconv.Itoa(width))
	b.WriteString(`" height="`)
	b.WriteString(strconv.Itoa(height))
	b.WriteString(`" viewBox="0 0 `)
	b.WriteString(strconv.Itoa(width))
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(height))
	b.WriteString(`" shape-rendering="crispEdges">`)
	b.WriteByte('\n')
	b.WriteString(`<rect width="100%" height="100%" fill="#fff"/>`)
	b.WriteByte('\n')
	b.WriteString(`<path fill="#000" d="`)
	m.writePath(&b, scale, quiet)
	b.WriteString("\"/>\n</svg>\n")
	return b.String()
}

// writePath appends the path data of the dark modules: one subpath per
// horizontal run, in pixel units.
func (m *Matrix) writePath(b *strings.Builder, scale, quiet int) {
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
			b.WriteByte('M')
			b.WriteString(strconv.Itoa(x))
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(y))
			b.WriteByte('h')
			b.WriteString(strconv.Itoa(runWidth))
			b.WriteByte('v')
			b.WriteString(strconv.Itoa(scale))
			b.WriteString("h-")
			b.WriteString(strconv.Itoa(runWidth))
			b.WriteByte('z')
			col += run - 1
		}
	}
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
