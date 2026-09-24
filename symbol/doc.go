// Package symbol renders GS1 element strings as barcode symbols.
//
// It currently implements ECC 200 Data Matrix (ISO/IEC 16022) in square and
// rectangular sizes, with GS1 mode signaled by FNC1 in the first position.
// Data is compacted with ASCII encodation, which is valid for every GS1
// element string; the denser C40, Text, X12, EDIFACT and Base 256 modes are
// not implemented, so some inputs pick a larger symbol than an optimizing
// encoder would.
//
// A Matrix is a grid of dark and light modules without quiet zone. Render it
// with Image, PNG or SVG, choosing the module size to match the X-dimension
// of the target printer.
//
// Like the parent module, the package has no dependencies outside the
// standard library.
package symbol
