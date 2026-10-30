// Package symbol encodes GS1 keys and element strings as barcode symbols.
//
// It currently implements ECC 200 Data Matrix (ISO/IEC 16022) in square and
// rectangular sizes, plain and in GS1 mode, returning the symbol as a Matrix
// of modules that renders to an image, PNG or SVG. Further symbologies
// follow per ADR 0009.
//
// Like the parent module, the package has no dependencies outside the
// standard library.
package symbol
