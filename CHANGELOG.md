# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `symbol` package (prototype for #24): ECC 200 Data Matrix in all 24
  square and 6 rectangular sizes, GS1 mode with FNC1 in first position.
  `GS1DataMatrix` and `DataMatrix` return a `Matrix` rendered with `Image`,
  `Paletted`, `PNG` or `SVG`.
- `symbol.Encoder`, which caches module layouts per symbol size for
  repeated encoding. The package functions keep no state.
- `gs1 datamatrix` command writing SVG or PNG.
- zint golden tests for every symbol size and `make verify-symbol`, which
  decodes rendered symbols with zxing-cpp outside the module.
- `docs/datamatrix.md` and `docs/performance.md`.

## [0.1.0] - 2026-09-22

First release as a standalone module. The code was extracted from the `gs1`
sub-module of [health-interop](https://github.com/galenzo17/health-interop)
with its full history.

### Added

- `Parse` and `ParseInto` for GS1-128 / GS1 DataMatrix element strings with
  FNC1 (ASCII 29) separators, bracket notation, AIM symbology identifiers and
  bare EAN-13 / UPC-A / GTIN-14.
- Scanner resilience by default: BOM, CR/LF, NUL and duplicated FNC1 are
  normalized before parsing.
- Recovery from missing FNC1 between variable-length fields using a
  balanced-split heuristic.
- `Barcode` accessors: `GTIN`, `Lot`, `SerialNumber`, `SSCC`, `Count`,
  `ExpirationDate`, `ProductionDate`, `BestBeforeDate`, `Get`.
- `LookupAI` exposing AI name and GS1 Syntax Dictionary format (`N14`, `X..20`).
- `ValidateGTIN`, `ComputeGTINCheckDigit` and `ExpandUPCE`.
- `ParseDate`, `ParseDateWithOptions` (configurable day-zero policy) and
  `ParseDateRaw`.
- Regulatory profiles for ANVISA (BR), ANMAT (AR), SNFA (CL) and COFEPRIS (MX).
- `cmd/gs1`: dependency-free CLI with text/JSON output, stdin streaming for
  scanner loops, GTIN and AI lookup subcommands.
- `cmd/gs1-wasm`: WebAssembly bindings with a JavaScript loader and
  TypeScript declarations.
- Fuzz target `FuzzParse` run on every CI build and nightly.

[Unreleased]: https://github.com/galenzo17/gs1-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/galenzo17/gs1-go/releases/tag/v0.1.0
