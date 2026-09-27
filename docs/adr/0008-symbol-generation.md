# ADR 0008: Symbol generation in a `symbol` sub-package

## Status
Proposed

## Context
[ADR 0002](0002-parser-scope.md) puts barcode generation out of scope. That
decision was taken while the parser lived inside a larger application that
already had a generator. As a standalone module aimed at the GS1 LATAM
solution-provider certification, section 3.5 of the checklist asks the
software to print labels with GS1 symbologies (#24), and integrators
currently combine a parser, an encoder and an unrelated renderer. FNC1
placement is where those combinations break.

`Encode` (#16) now produces validated element strings with correct FNC1
placement. A dependency-free GS1 DataMatrix encoder built on it takes about
730 lines of non-test code, including the renderers and the CLI command.

## Decision

### Supersede the "no generation" clause of ADR 0002
Symbol generation is in scope. Print-quality verification (ISO/IEC 15415,
15416) and GS1 Digital Link stay out of scope.

### Sub-package `github.com/galenzo17/gs1-go/symbol`
| Option | Cost for parser-only users | Discoverability |
|---|---|---|
| **Sub-package in this module** | **none: not imported, not linked** | **same repository, same release** |
| Separate module | none | second repository, versions to keep in sync |
| Root package | `image` and `image/png` linked into every binary | high |

The sub-package keeps the root package, the CLI's parse path and the
WebAssembly build free of rendering code.

### Elements in, `Encode` for the element string
GS1 symbol functions take `[]gs1.Element` and build the element string with
`Encode`, so a symbol can only carry what `Encode` accepts: set 82
characters, valid GTIN check digits, no repeated AIs, predefined-length AIs
first. They do not take a scanned string, because `Parse` is deliberately
lenient and recovers a missing FNC1 heuristically (ADR 0006); a generator
must not print a guess. Callers holding bracket notation parse it first.

### Matrix out, standard library renderers
Encoders return a `Matrix` of modules without quiet zone. It renders to
`image.Image`, PNG and SVG with the standard library, with module size and
quiet zone chosen by the caller for the printer's X-dimension. ZPL, a text
format, can follow without a library.

### Specification sources
Encoders are written from ISO/IEC 16022 and the GS1 General Specifications,
not ported from other encoders. For 144x144 Data Matrix the ECC codewords of
the two shorter Reed-Solomon blocks come first; every deployed reader and
encoder uses that layout, which a literal reading of ISO/IEC 16022 does not
give (zxing-cpp issue 259).

### Verification without test dependencies
- Golden data produced by an external encoder (zint) is committed, and
  `go test` compares every symbol size module for module.
- A test-only reader decodes symbols back through the Reed-Solomon syndromes
  and ASCII decoding, so round trips run in `go test` and under fuzzing.
- `make verify-symbol` decodes rendered PNGs with an independent reader
  (zxing-cpp) from a script outside `go.mod`.

## Consequences
- ADR 0002 remains accepted for everything except the generation clause.
- GS1-128, GS1 QR Code, EAN/UPC and ITF-14 can land as further files in
  `symbol`, each with golden data from a reference encoder.
- Only ASCII encodation is implemented for Data Matrix. It encodes every
  GS1 element string; long alphanumeric data may get a larger symbol than an
  optimizing encoder would pick.
- The CLI gains rendering commands; the WebAssembly bindings can expose them
  later (#21).
