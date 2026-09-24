package symbol

import (
	"image"
	"io"
)

// Image renders the matrix as a grayscale image with scale pixels per module
// and a quiet zone of quiet modules on every side.
func (m *Matrix) Image(scale, quiet int) *image.Gray {
	panic("Image: not implemented")
}

// PNG writes the matrix as a PNG image; see Image for the parameters.
func (m *Matrix) PNG(w io.Writer, scale, quiet int) error {
	panic("PNG: not implemented")
}

// SVG returns the matrix as a standalone SVG document; see Image for the
// parameters. One user unit equals one pixel at the given scale.
func (m *Matrix) SVG(scale, quiet int) string {
	panic("SVG: not implemented")
}
