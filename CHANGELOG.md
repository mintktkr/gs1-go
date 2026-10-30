# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Preserve literal parentheses in raw element-string values. Bracket
  notation is detected only at the first AI after scanner cleaning and
  prefixes, so `10LOT(90)X` retains the lot value `LOT(90)X`.

### Changed

- Strict association validation applies the complete dictionary rules, so
  inputs such as (12) without (8020) or (10) without an item key now fail
  under `ValidateAssociations`. Lenient parsing is unchanged.
- Twelve-digit input whose leading AI cannot fit its fixed data, such as
  `010415000002`, parses as a bare UPC-A instead of a truncated AI (01).
- Association error messages name the AI and alternatives from the
  dictionary, e.g. `AI (3102) excludes AI (3103)`.

### Added

- GoReleaser release pipeline (`.goreleaser.yaml`): the same six binaries
  as before with their bare names plus tar.gz/zip archives, SHA-256
  checksums, SPDX SBOMs, deb/rpm/apk packages, a Homebrew cask in
  `galenzo17/homebrew-tap`, the `gs1-bin` AUR package, a `FROM scratch`
  multi-arch image at `ghcr.io/galenzo17/gs1`, a changelog grouped by
  conventional-commit type and the WebAssembly artifacts attached to the
  release. `gs1 version` reports commit and date on release builds;
  `make snapshot` runs the pipeline locally.
- npm package `@galenzo17/gs1-wasm`: `wasm/package.json` with ESM exports
  and types, a loader that resolves `gs1.wasm` and `wasm_exec.js` next to
  itself (bundlers, CDNs and Node 20+ work without a path argument), a
  package README, publication with npm provenance from the release
  workflow with the version taken from the git tag, and a CI smoke test
  that loads the package through its own loader and checks the published
  file list.
- `RELEASING.md` with the module proxy rules (immutable tags, `retract`,
  `v0.x` versus `v1.0.0` versus `/v2`), the pre-tag checklist, what a tag
  publishes and the criteria for `v1.0.0`. A `hygiene` CI job and
  `make tidy-check` enforce the zero-dependency promise (`go mod tidy`
  produces no diff, no `require` block, no `go.sum`) and gofmt. Runnable
  examples for strict parsing, association and umbrella validation, HRI,
  the identifier builders, UPC-E expansion, check-digit computation,
  measures, warnings and raw dates, so pkg.go.dev shows every entry point.
- EPC Tag Data Standard decoding: `ParseEPC` and `DecodeEPC` read the
  hexadecimal memory content of SGTIN-96/198, SSCC-96, SGLN-96/195,
  GRAI-96/170 and GIAI-96/202 tags, `ParseEPCURI` and `DecodeEPCURI` read
  tag and pure-identity URIs, and both yield the equivalent element string
  ((01)+(21), (00), (414)+(254), (8003), (8004)) with `Symbology` `SymEPC`.
  CLI `gs1 epc` and WASM `parseEPC()` expose it with an `epc` object in the
  JSON result.
- Conformance suite against the GS1 Barcode Syntax Engine 1.4.1 and Syntax
  Dictionary 2026-01-27 test vectors, vendored under `testdata/gs1` with
  their licences: 1281 vectors run through the parser, 92% agree, and the
  100 documented divergences in `known-divergences.txt` must change
  deliberately. `make conformance` and a CI job report the rate.
  `ValidateAssociations` rejects an AI repeated with different values, the
  encoder enforces character set 82 exactly, and base64url components
  accept standard padding. `testdata/labels` holds logistic label element
  strings per carrier.
- The AI table is generated from the GS1 Barcode Syntax Dictionary
  (release 2026-01-27, Apache 2.0) by `go generate`, growing from 63 to 541
  Application Identifiers with typed components, predefined-length flags,
  Digital Link attributes and the dictionary's `req`/`ex` association rules.
  `ValidateAssociations` evaluates those rules for every AI; `LookupAI`
  reports `Title` and `Predefined` and renders `Format` in dictionary
  notation; `SyntaxDictionaryRelease()` exposes the release. CI fails when
  the committed table differs from the dictionary.
- CLI and WASM parity: WASM elements carry `name`, `measure` and the ISO
  `date` like the CLI JSON output, `assumeBareGTIN8` in WASM and `-gtin8` in the CLI expose
  `ParseOptions.AssumeBareGTIN8`, and the identifier builders reach both
  entry points as `gs1 gtin13`, `gs1 gtin14`, `newSSCC`, `newGTIN13` and
  `newGTIN14`.
- `Encode` enforces AI association rules, so an element string it accepts
  also parses with `ValidateAssociations`. `EncodeZPL` emits ZPL `^FD` field
  data with `>8` FNC1 escapes and `ParseHRI` splits bracket notation
  literally for encoder input. CLI `gs1 encode [-hri] [-zpl]` and WASM
  `encode()` expose both, and golden tests cover the GS1 Logistics Label
  Guideline and Syntax Engine example strings.
- Check-digit validation for SSCC, GLN, GSIN and GSRN through
  `ValidateSSCC`, `ValidateGLN`, `ValidateGSIN`, `ValidateGSRN`, the generic
  `ComputeCheckDigit`, and `ValidateKey` with `KeyType` detection by length.
  `ParseOptions.ValidateCheckDigits`, `Barcode.Validate()` and `Encode` now
  cover AIs 00, 402, 410–417, 8017, 8018 and the numeric prefix of 253 and
  8003 in addition to 01 and 02. CLI `gs1 key KEY` and WASM `validateKey`.
- Opt-in GTIN check-digit validation for AI 01 and AI 02 through
  `ParseOptions.ValidateCheckDigits`, CLI `-check-digits`, and WASM
  `validateCheckDigits`, independently of association validation.
- Typed `Barcode` accessors and matching CLI JSON, WebAssembly and TypeScript fields.
- SSCC/GTIN identifier builders and check-digit validation helpers.
- Identification AIs 253, 401, 8003, 8004, 8017 and 8018, with `GDTI()` and
  `GRAI()` accessors.
- Missing-FNC1 recovery coverage for mixed-format identification AIs.
- Add dependency-free element string encoding and HRI formatting.
- Element encoding validates 01/02 GTIN check digits.
- Made unsupported carrier variants fail explicitly and exposed detected
  symbology names in CLI and WASM results.
- Added AIM symbology carrier detection and support for EAN/UPC, ITF-14, and
  opt-in bare GTIN-8 parsing.
- `Measure` and typed barcode measurement accessors for net and gross weight.
- Measurement data in CLI JSON output.
- `DueDate()` and `Warnings()` support, including due-date advisories in CLI
  and WASM JSON output.
- `symbol` sub-package with the ECC 200 Data Matrix encoder (ISO/IEC 16022)
  of ADR 0009. `GS1DataMatrix` takes validated elements and builds the
  element string with `Encode`, so a symbol can only carry what the encoder
  accepts; `DataMatrix` encodes arbitrary data. Both return a `Matrix` of
  modules in the smallest square or rectangular size that fits, or
  `ErrDataTooLong`. ASCII encodation can express every GS1 element string. Golden data from zint checks
  every module of all 30 symbol sizes, and a test-only reader round-trips
  symbols in `go test` and under fuzzing.
- `Matrix` renderers: `Image` returns an `*image.Gray`, `PNG` writes
  a PNG and `SVG` a standalone document, each at a caller-chosen number of
  pixels per module and quiet zone in modules, standard library only. CLI
  `gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE` renders
  from the same input as `gs1 encode`, SVG on stdout or PNG when FILE ends
  in .png. `make verify-symbol` decodes every rendered symbol with zxing-cpp
  through uv, outside `go.mod`.

### Changed

- `Barcode.Validate()` now checks GTIN check digits for AI 01 and AI 02
  after AI associations, using the current elements without reparsing raw
  input. Association errors retain priority; existing strict parsing options
  remain association-only and default parsing remains lenient.
- Corrected strict AI association rules to match the GS1 Syntax Dictionary.

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
