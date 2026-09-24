# gs1-go

[![CI](https://github.com/galenzo17/gs1-go/actions/workflows/ci.yml/badge.svg)](https://github.com/galenzo17/gs1-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/galenzo17/gs1-go.svg)](https://pkg.go.dev/github.com/galenzo17/gs1-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/galenzo17/gs1-go)](https://goreportcard.com/report/github.com/galenzo17/gs1-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A dependency-free Go library for parsing and validating **GS1 Application
Identifier element strings**: the data carried by GS1-128, GS1 DataMatrix,
GS1 QR Code and GS1 DataBar symbols.

It is built for the data-capture edge, where scanner output is noisy and
throughput matters: hospital dispensing, pharmacy point of sale, warehouse
receiving, and pharmaceutical serialization systems.

```go
import "github.com/galenzo17/gs1-go"

b, err := gs1.Parse("]d20104150000021126172506302112345ABC\x1D10LOT42X")
if err != nil {
    return err
}
b.GTIN()          // "04150000021126"
b.Lot()           // "LOT42X"
b.SerialNumber()  // "12345ABC"
b.ExpirationDate() // 2025-06-30 00:00:00 +0000 UTC
```

## Features

- **Element string parsing** for fixed- and variable-length AIs with FNC1
  (ASCII 29) separators, bracket notation `(01)…(17)…`, AIM symbology
  identifiers (`]C1`, `]d2`, `]Q3`, `]e0`, `]J1`) and bare EAN-13 / UPC-A / GTIN-14.
- **Scanner resilience by default.** UTF-8 BOM, CR/LF, NUL bytes and
  duplicated FNC1 from keyboard-wedge and USB HID scanners are normalized
  before parsing. See [ADR 0003](docs/adr/0003-scanner-resilience.md).
- **Missing-FNC1 recovery.** When a scanner drops the separator between two
  variable-length fields, a balanced-split heuristic restores the boundary
  instead of failing.
- **Zero-allocation hot path.** `ParseInto` reuses a caller-owned `Barcode`;
  values are substrings of the input. See [ADR 0004](docs/adr/0004-zero-allocation-parsing.md).
- **GTIN check digits** (GTIN-8/12/13/14), UPC-E expansion, YYMMDD date
  parsing with a configurable day-zero policy.
- **Regulatory profiles** for LATAM pharmaceutical traceability: ANVISA,
  ANMAT, SNFA, COFEPRIS.
- **WebAssembly build** with a JavaScript loader and TypeScript declarations.
- **CLI** for scripts and scanner loops, also dependency-free.
- **Fuzzed continuously** in CI and nightly.

## Installation

```bash
go get github.com/galenzo17/gs1-go@latest
```

Requires Go 1.23 or later. The module has no dependencies outside the
standard library.

CLI:

```bash
go install github.com/galenzo17/gs1-go/cmd/gs1@latest
```

## Usage

### Parse

`Parse` accepts whatever the scanner sends. The returned `Barcode` keeps the
elements in scan order and offers typed accessors for the common healthcare AIs.

```go
b, err := gs1.Parse("(01)04150000021126(17)250630(10)ABC123")
if err != nil {
    log.Fatal(err)
}

for _, e := range b.Elements {
    ai, _ := gs1.LookupAI(e.AI)
    fmt.Printf("(%s) %-16s %s\n", e.AI, ai.Name, e.Value)
}
// (01) GTIN             04150000021126
// (17) Expiration Date  250630
// (10) Batch/Lot        ABC123

v, ok := b.Get("17") // generic lookup, first occurrence
```

Errors wrap sentinel values so callers can branch with `errors.Is`:

| Sentinel | Meaning |
|---|---|
| `ErrEmptyInput` | Input is empty or whitespace |
| `ErrUnknownAI` | Unrecognized Application Identifier at a position |
| `ErrTruncatedData` | Fixed-length field cut short |
| `ErrInvalidData` | Data violates the AI's type or length |
| `ErrInvalidCheckDigit` | GTIN modulo-10 check failed |
| `ErrInvalidDate` | YYMMDD field is not a calendar date |
| `ErrMissingRequiredAI` | A regulator-mandated AI is absent |

### Zero-allocation loop

For continuous scanning or batch pipelines, reuse a `Barcode`. Each goroutine
must own its instance; there is no shared state.

```go
var b gs1.Barcode
for sc.Scan() {
    b.Reset()
    if err := gs1.ParseInto(sc.Text(), &b); err != nil {
        log.Println(err)
        continue
    }
    process(b.GTIN(), b.Lot(), b.SerialNumber())
}
```

| API | Allocs/op | Throughput, single core |
|---|---|---|
| `Parse` | 1 | ~3.9M/s |
| `ParseInto` | 0 | ~5.9M/s |

Run `make bench` to reproduce on your hardware.

### GTIN validation

Parsing checks structure only. Validate check digits explicitly:

```go
err := gs1.ValidateGTIN("04150000021126")            // nil
check, _ := gs1.ComputeGTINCheckDigit("0415000002112") // '6'
upca, _ := gs1.ExpandUPCE("012345")                    // 12-digit UPC-A
```

### Dates

GS1 dates are `YYMMDD` with years mapped to 2000–2099. A day of `00` denotes
the end of the month in the GS1 General Specifications; some national systems
interpret it as the first day instead.

```go
t, _ := gs1.ParseDate("250200")                                   // 2025-02-28
t, _ = gs1.ParseDateWithOptions("250200",
        gs1.DateOptions{DayZero: gs1.DayZeroFirstDay})            // 2025-02-01
raw, _ := gs1.ParseDateRaw("250630")                              // "250630", validated
```

### Regulatory profiles

Check that a barcode carries the AIs a national regulator mandates for
pharmaceutical traceability.

```go
b, _ := gs1.Parse("0104150000021126172506302112345\x1D10LOT1")

err := gs1.ANVISA.Validate(b)   // or b.ValidateANVISA()
```

| Regulator | Country | Required AIs |
|---|---|---|
| ANVISA | Brazil | 01, 17, 10, 21 |
| ANMAT | Argentina | 01, 17, 10, 21 |
| SNFA | Chile | 01, 17, 10 |
| COFEPRIS | Mexico | 01, 17, 10 |

Custom profiles are plain values: `gs1.Regulator{Name: "…", RequiredAIs: []string{"01", "10"}}`.

## Supported Application Identifiers

Formats use GS1 Syntax Dictionary notation: `N14` is exactly 14 digits,
`X..20` is up to 20 characters from the GS1 AI encodable character set 82.

| AI | Data title | Format |
|---|---|---|
| 00 | SSCC | N18 |
| 01 | GTIN | N14 |
| 02 | Content GTIN | N14 |
| 10 | Batch/Lot | X..20 |
| 11 | Production Date | N6 |
| 13 | Packaging Date | N6 |
| 15 | Best Before Date | N6 |
| 17 | Expiration Date | N6 |
| 21 | Serial Number | X..20 |
| 30 | Count | N..8 |
| 37 | Count of Trade Items | N..8 |
| 240 | Additional Product ID | X..30 |
| 241 | Customer Part Number | X..30 |
| 310n | Net Weight, kg | N6 |
| 320n | Net Weight, lb | N6 |
| 330n | Gross Weight, kg | N6 |
| 340n | Gross Weight, lb | N6 |
| 402 | GSIN | N17 |
| 410 | Ship to / Deliver to GLN | N13 |
| 411 | Bill to / Invoice to GLN | N13 |
| 412 | Purchased from GLN | N13 |
| 413 | Ship for / Deliver for GLN | N13 |
| 414 | GLN | N13 |
| 415 | Invoicing party GLN | N13 |
| 416 | Production / service location GLN | N13 |
| 417 | Party GLN | N13 |
| 254 | GLN extension component | X..20 |
| 7040 | GS1 UIC with extension | N1 + X3 |
| 710–714 | NHRN (DE, FR, ES, BR, PT) | X..20 |
| 90 | Internal | X..30 |
| 91–99 | Internal | X..90 |

`gs1 ai <code>` on the command line, or `gs1.LookupAI` in code, returns the
same information. Requests for additional AIs are welcome; see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Input formats

| Source | Example |
|---|---|
| GS1-128 keyboard wedge | `0104150000021126172506301012345` |
| FNC1 as ASCII 29 | `2112345\x1D10LOT1` |
| Human-readable brackets | `(01)04150000021126(17)250630(10)12345` |
| AIM prefix, DataMatrix | `]d2…` (also `]d1`) |
| AIM prefix, GS1-128 | `]C1…` |
| AIM prefix, QR / DotCode / composite | `]Q3…`, `]J1…`, `]e0…` |
| Bare GTIN (EAN-13, UPC-A, GTIN-14) | `7800038041425` → AI 01, zero-padded to 14 |

GS1 DataBar needs no special handling: scanners decode it to the same
element string as GS1-128.

## Command line

```bash
gs1 parse "0104150000021126172506302112345ABC"        # text table
gs1 parse -json -iso "(01)04150000021126(17)250200"   # JSON, ISO dates
gs1 parse -validate anvisa "$SCAN"                     # exit 1 if non-compliant
cat scans.txt | gs1 parse -json                        # one JSON object per line
gs1 gtin 04150000021126                                # check digit
gs1 ai 3102                                            # AI (3102)  Net Weight kg  N6
```

Exit codes: `0` success, `1` invalid input or failed validation, `2` usage error.

## WebAssembly

The same parser runs in browsers, Electron and edge runtimes.

```bash
make wasm   # produces wasm/gs1.wasm and a servable example under wasm/example/
```

```html
<script src="wasm_exec.js"></script>
<script type="module">
  import { loadGS1 } from './gs1-loader.js';
  const gs1 = await loadGS1('gs1.wasm');

  const r = gs1.parse("0104150000021126172502001012345", { dateFormat: "iso", dayZero: "first" });
  r.gtin;            // "04150000021126"
  r.expirationDate;  // "2025-02-01"

  gs1.validateGTIN("04150000021126");            // true
  gs1.validateRegulatory(scan, "anvisa");       // null when compliant, else message
</script>
```

TypeScript declarations live in [`wasm/gs1.d.ts`](wasm/gs1.d.ts). Design
notes are in [ADR 0005](docs/adr/0005-webassembly-target.md).

## Symbol generation (prototype)

The `symbol` package renders GS1 DataMatrix symbols with no dependencies.
It is a prototype for [#24](https://github.com/galenzo17/gs1-go/issues/24)
and does not change the scope below until an ADR decides it.

```go
m, err := symbol.GS1DataMatrix("(01)04150000021126(17)250630(10)ABC123", symbol.DataMatrixOptions{})
svg := m.SVG(10, 1) // or m.PNG(w, 10, 1), m.Image, m.Paletted
```

```bash
gs1 datamatrix -o dm.png "(01)04150000021126(17)250630(10)ABC123"
```

See [docs/datamatrix.md](docs/datamatrix.md) for sizes, outputs,
verification and limits, and [docs/performance.md](docs/performance.md) for
profiling results and the opt-in `Encoder`.

## Scope

This library is a parsing and validation layer. It does not generate
barcodes, verify print quality, resolve GS1 Digital Link URIs, or implement
business documents such as dispatch advices. See
[ADR 0002](docs/adr/0002-parser-scope.md) for the reasoning and the GS1
resources that cover those areas.

## Related GS1 resources

- [GS1 General Specifications](https://www.gs1.org/standards/barcodes-epcrfid-id-keys/gs1-general-specifications)
- [GS1 Syntax Dictionary](https://github.com/gs1/gs1-syntax-dictionary) and
  [GS1 Barcode Syntax Engine](https://github.com/gs1/gs1-syntax-engine)
- [GS1 Healthcare](https://www.gs1.org/industries/healthcare)

## Contributing

Issues and pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md)
for the workflow and [docs/adr](docs/adr) for design history. Security
reports go through [SECURITY.md](SECURITY.md).

## License

MIT. Copyright (c) 2026 Agustín Bereciartúa Castillo. See [LICENSE](LICENSE).

GS1 is a registered trademark of GS1 AISBL. This project is an independent
implementation of publicly available specifications and is not affiliated
with, sponsored by, or endorsed by GS1.
