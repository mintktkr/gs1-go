# gs1-go

[![CI](https://github.com/galenzo17/gs1-go/actions/workflows/ci.yml/badge.svg)](https://github.com/galenzo17/gs1-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/galenzo17/gs1-go.svg)](https://pkg.go.dev/github.com/galenzo17/gs1-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/galenzo17/gs1-go)](https://goreportcard.com/report/github.com/galenzo17/gs1-go)
[![Go version](https://img.shields.io/github/go-mod/go-version/galenzo17/gs1-go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A dependency-free Go library for parsing and validating **GS1 Application
Identifier element strings**: the data carried by GS1-128, GS1 DataMatrix,
GS1 QR Code and GS1 DataBar symbols.

It is built for the data-capture edge, where scanner output is noisy and
throughput matters: hospital dispensing, pharmacy point of sale, warehouse
receiving, and pharmaceutical serialization systems.

```go
import "github.com/galenzo17/gs1-go"

b, err := gs1.Parse("(01)04150000021126(02)04150000021126(13)250601(17)250630(37)42(402)12345678901234567(414)1234567890123(21)12345ABC(10)LOT42X")
if err != nil {
    return err
}
b.GTIN()          // "04150000021126"
b.Lot()           // "LOT42X"
b.SerialNumber()  // "12345ABC"
b.ExpirationDate() // 2025-06-30 00:00:00 +0000 UTC
b.PackagingDate() // 2025-06-01 00:00:00 +0000 UTC
b.ContentGTIN() // "04150000021126"
b.CountOfTradeItems() // "42"
b.GLN() // "1234567890123"
b.GSIN() // "12345678901234567"
```

## Features

- **Every Application Identifier** of the GS1 Barcode Syntax Dictionary,
  generated from the vendored release by `go generate`.
- **Element string parsing** for fixed- and variable-length AIs with FNC1
  (ASCII 29) separators, bracket notation `(01)…(17)…`, AIM symbology
  identifiers (`]C1`, `]d2`, `]Q3`, `]e0`, `]J1`, `]E`, `]I`) and bare EAN-13 / UPC-A / GTIN-14.
- **Element string encoding** with canonical fixed-AI ordering, FNC1 placement,
  association and check-digit enforcement, ZPL `^FD` output, and
  human-readable interpretation through `Barcode.HRI()`.
- **GS1 DataMatrix generation** in the `symbol` sub-package: ECC 200 in every
  square and rectangular size, rendered as `image.Image`, PNG or SVG, from the
  elements `Encode` accepts. See [ADR 0009](docs/adr/0009-symbol-generation.md).
- **Carrier detection** through `Barcode.Symbology`, including EAN/UPC and ITF-14.
- **Scanner resilience by default.** UTF-8 BOM, CR/LF, NUL bytes and
  duplicated FNC1 from keyboard-wedge and USB HID scanners are normalized
  before parsing. See [ADR 0003](docs/adr/0003-scanner-resilience.md).
- **Missing-FNC1 recovery.** When a scanner drops the separator between two
  variable-length fields, a balanced-split heuristic restores the boundary
  instead of failing.
- **Zero-allocation hot path.** `ParseInto` reuses a caller-owned `Barcode`;
  values are substrings of the input. See [ADR 0004](docs/adr/0004-zero-allocation-parsing.md).
- **Check digits** for GTIN-8/12/13/14, SSCC, GLN, GSIN and GSRN, key-type
  detection for operator input, UPC-E expansion, YYMMDD date parsing with a
  configurable day-zero policy.
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

CLI, from the package manager of your platform or from source:

```bash
brew install galenzo17/tap/gs1                       # macOS and Linux (Homebrew)
yay -S gs1-bin                                       # Arch Linux (AUR)
docker run --rm ghcr.io/galenzo17/gs1 parse "(01)04150000021126(10)LOT42"
go install github.com/galenzo17/gs1-go/cmd/gs1@latest
```

Every [GitHub release](https://github.com/galenzo17/gs1-go/releases) also
carries bare binaries and archives for linux, macOS and Windows on amd64 and
arm64, deb/rpm/apk packages, SHA-256 checksums and SPDX SBOMs, all produced
by [GoReleaser](.goreleaser.yaml). `make snapshot` runs the same pipeline
locally without a tag.

## Usage

### Parse

`Parse` accepts whatever the scanner sends. The returned `Barcode` keeps the
elements in scan order and offers typed accessors for the common healthcare AIs.

```go
b, err := gs1.Parse("(01)04150000021126(3102)001234(17)250630(10)ABC123")
if err != nil {
    log.Fatal(err)
}

for _, e := range b.Elements {
    ai, _ := gs1.LookupAI(e.AI)
    fmt.Printf("(%s) %-16s %s\n", e.AI, ai.Name, e.Value)
}
// (01) GTIN             04150000021126
// (3102) Net Weight kg    001234
// (17) Expiration Date  250630
// (10) Batch/Lot        ABC123

v, ok := b.Get("17") // generic lookup, first occurrence
```

Weight AIs expose their implied decimal point through typed measurements. The
raw and scaled forms remain available when an exact decimal representation is
needed:

```go
weight, ok := b.NetWeightKg() // AI 310n
if ok {
    fmt.Printf("%.2f %s (raw %s)\n", weight.Value, weight.Unit, weight.Raw)
}
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

### Check-digit validation

Default parsing does not enforce check digits. Validate a key explicitly or
enable check-digit validation while parsing:

```go
err := gs1.ValidateGTIN("04150000021126")            // nil
err = gs1.ValidateSSCC("106141411234567897")          // nil
err = gs1.ValidateGLN("0614141000012")                // nil
err = gs1.ValidateGSIN("06141410000000012")           // nil
check, _ := gs1.ComputeGTINCheckDigit("0415000002112") // '6'
check, _ = gs1.ComputeCheckDigit("10614141123456789")  // '7', any 1–30 digit key
upca, _ := gs1.ExpandUPCE("012345")                    // 12-digit UPC-A

types, err := gs1.ValidateKey("0614141000012")         // [GTIN GLN], nil

_, err = gs1.ParseWithOptions("0104150000021127",
    gs1.ParseOptions{ValidateCheckDigits: true})
// errors.Is(err, gs1.ErrInvalidCheckDigit) == true
```

`ValidateKey` is for keys of unknown family, such as values typed by an
operator: it validates the check digit and reports every family the length
admits, because GTIN-13 and GLN share 13 digits and SSCC and GSRN share 18.

`ValidateCheckDigits` checks every numeric GS1 key in the element string:
GTIN (AI 01 and 02, including bare GTINs and EAN/UPC and ITF-14 payloads
normalized to AI 01), SSCC (00), GSIN (402), the GLN family (410–417), GSRN
(8017, 8018) and the 13-digit key prefix of GDTI (253) and GRAI (8003). It
is independent of `ValidateAssociations`:

| ValidateAssociations | ValidateCheckDigits | Semantic checks |
|---|---|---|
| false | false | None (default) |
| true | false | AI associations |
| false | true | Check digits of every numeric GS1 key |
| true | true | AI associations, then check digits |

Syntax, length, and data-type errors take precedence over semantic errors.
Check-digit errors wrap `ErrInvalidCheckDigit` and identify the AI and its
start position in the normalized input, after scanner cleaning and bracket
conversion. Carrier errors identify the normalized payload start.

Existing bare-GTIN/AI-prefix disambiguation is unchanged; bare GTIN-8 still
requires `AssumeBareGTIN8: true`. For eight-digit UPC-E (`]E0`), the option
checks the received digit before expansion replaces it. A 13-digit `]I2`
payload has no received check digit: the parser synthesizes it, so validation
cannot detect an incorrect digit that the scanner has already removed.

Applications that allocate their own logistics identifiers can build the
numeric keys without implementing the check-digit algorithm themselves:

```go
sscc, _ := gs1.NewSSCC('3', "7801234", "000000009")
gtin13, _ := gs1.NewGTIN13("7801234", "00005")
caseGTIN, _ := gs1.NewGTIN14('1', gtin13)
// sscc:     378012340000000095
// caseGTIN: 17801234000053
```

`SplitSSCC` accepts the known company-prefix length when an application needs
to recover the SSCC components. Allocation counters and persistence remain
the application's responsibility.

### Encoding for label printing

`Encode` builds the element string a barcode generator needs from AI/value
pairs: predefined-length AIs first so they need no separator, FNC1 (ASCII GS)
before every other AI, and a leading FNC1 marking the GS1 payload. It
enforces the same rules as strict parsing, so a string it accepts parses with
`ValidateAssociations` and `ValidateCheckDigits`: values are checked against
their AI format and character set 82, duplicates and wrong check digits are
rejected, and association rules such as `(02)` requiring `(00)` and `(37)`
apply.

```go
elements := []gs1.Element{
    {AI: "10", Value: "ABC123"},
    {AI: "00", Value: "790123451234500116"},
    {AI: "02", Value: "49012345008794"},
    {AI: "37", Value: "12"},
}
raw, err := gs1.Encode(elements)
// "\x1D00790123451234500116" + "0249012345008794" + "10ABC123" + "\x1D3712"

zpl, err := gs1.EncodeZPL(elements)
// ">800790123451234500116" + "0249012345008794" + "10ABC123" + ">83712"
// ^BCN,100,Y,N,N,A^FD<zpl>^FS

b, _ := gs1.Parse(raw)
b.HRI() // "(00)790123451234500116(02)49012345008794(10)ABC123(37)12"
```

`ParseHRI` is the inverse of `Barcode.HRI()` for encoder input typed by
people or stored in label templates: it splits bracket notation literally,
without the AI table or scanner heuristics of `Parse`, so a mistyped AI
reaches `Encode` and fails as `ErrUnknownAI`.

`EncodeZPL` writes every FNC1 as the Code 128 invocation code `>8` and a
literal `>` as `><`, for the `^BC` command in automatic mode. GS1 DataMatrix
(`^BX`) uses a different FNC1 escape and is not covered.

### Data Matrix

`symbol.GS1DataMatrix` builds the element string with `Encode`, so its input
is validated the same way, and returns a `Matrix` of modules without quiet
zone. Render it at the module size and quiet zone the label needs; GS1 Data
Matrix requires at least one module (General Specifications 5.12.3).
`GS1DataMatrix` takes elements rather than a string, because `Parse` is
deliberately lenient and recovers a missing FNC1 heuristically, and a printer
must not print that guess: callers holding bracket notation split it with
`ParseHRI` first. `gs1 datamatrix` accepts the same input as `gs1 encode`,
which does that split for bracket notation.

```go
elements, err := gs1.ParseHRI("(01)04150000021126(17)250630(10)ABC123")
if err != nil {
	return err
}
m, err := symbol.GS1DataMatrix(elements, symbol.DataMatrixOptions{})
if err != nil {
	return err
}
svg := m.SVG(10, 1)   // SVG document at 10 pixels per module, 1 module quiet zone
img := m.Image(10, 1) // *image.Gray with the quiet zone included
```

`PNG` writes that image to an `io.Writer`.

`DataMatrixOptions{Rectangular: true}` picks the smallest rectangular size.
Only ASCII encodation is implemented, which every GS1 element string can use;
an optimizing encoder may choose a smaller symbol for long alphanumeric data.
Symbols are checked against zint module for module in `go test`, decoded back
by a round-trip test, and decoded by zxing-cpp with `make verify-symbol`.

### RFID tags (EPC)

RFID reads land in the same `Barcode` shape as barcode scans. `ParseEPC`
decodes the hexadecimal EPC memory content of a tag and `ParseEPCURI` a tag
or pure-identity URI; `DecodeEPC` and `DecodeEPCURI` expose the identifier
fields. Supported coding schemes are SGTIN-96 and SGTIN-198 ((01) with
(21)), SSCC-96 ((00)), SGLN-96 and SGLN-195 ((414) with (254)), GRAI-96 and
GRAI-170 ((8003)) and GIAI-96 and GIAI-202 ((8004)), with check digits
computed and partition tables per the EPC Tag Data Standard.

```go
b, _ := gs1.ParseEPC("3074257BF7194E4000001A85")
b.Symbology      // gs1.SymEPC
b.GTIN()         // "80614141123458"
b.SerialNumber() // "6789"

epc, _ := gs1.DecodeEPCURI("urn:epc:tag:sscc-96:3.0614141.1234567890")
epc.Scheme, epc.Bits, epc.Filter // sscc 96 3
epc.URI()      // "urn:epc:id:sscc:0614141.1234567890"
epc.Elements() // [{00 106141412345678908}]
```

Writing tags is out of scope.

### Association rules

`Parse` remains lenient for scanner workflows. Use `Barcode.ValidateAssociations()`
or `ParseWithOptions` with `ValidateAssociations: true` to enforce required
pairs and exclusions such as `(02)` with `(37)`, weights with a GTIN, and
mutually exclusive identifier variants.

`Barcode.Validate()` runs association checks followed by the check-digit
checks of every numeric GS1 key. It uses the current `Elements` without reparsing
`Raw`; its errors provide AI context without an input position. Lenient
UPC-E expansion has already replaced the original digit, so post-parse
validation cannot recover a mismatched digit from that scan. Enable
`ValidateCheckDigits` while parsing to check it.

The association rules are the `req=` and `ex=` attributes of the GS1
Barcode Syntax Dictionary, evaluated for every AI present: each `req` group
needs one of its alternatives (`01+21` means both), and an `ex` pattern must
not match another AI, with `n` as a digit wildcard so `3102` with `3103`
fails on `310n`. Errors read `AI (37) requires AI (02) or AI (8026)` or
`AI (3102) excludes AI (3103)`. Some dictionary rules are stricter than the
scanner habits this parser tolerates leniently: (12) requires (8020), and
(10) requires an item key such as (01).

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

`Barcode.DueDate()` parses AI 12 and adds a `WarnDueDateAsExpiry` warning when
AI 17 is absent. Inspect `Barcode.Warnings()` to surface that advisory; the
CLI and WebAssembly JSON output expose the same warnings.

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

The AI table is generated from the
[GS1 Barcode Syntax Dictionary](https://github.com/gs1/gs1-syntax-dictionary)
(release 2026-01-27, Apache License 2.0, vendored under
`internal/gen/aitable`), so every Application Identifier in that release
parses: 541 AIs including the measures (310n–369n), prices (39nn),
shipping addresses (43nn), food and healthcare attributes (70nn, 71n, 72nn),
identification keys (253, 255, 401, 402, 8003–8018) and the Digital Link
and coupon AIs. `gs1.SyntaxDictionaryRelease()` returns the release in use.

Formats use dictionary notation: `N14` is exactly 14 digits, `X..20` is up
to 20 characters from the GS1 AI encodable character set 82, components are
separated by spaces and optional components are bracketed, as in `N13
[X..17]` for a GDTI. Numeric components, character set 39 (`Y`) and
base64url (`Z`) components are checked while parsing. Value linters such as
`yymmdd` or `iso3166` are recorded per component and exposed through the
existing options (`ParseDate`, `ValidateCheckDigits`).

`gs1 ai <code>` on the command line, or `gs1.LookupAI` in code, returns the
data title, the dictionary title, the format and whether the AI has a
predefined length. To move to a newer dictionary release, replace the
vendored file and run `make generate`; CI fails when the committed table
differs from the dictionary.

## Input formats

| Source | Example |
|---|---|
| GS1-128 keyboard wedge | `0104150000021126172506301012345` |
| FNC1 as ASCII 29 | `2112345\x1D10LOT1` |
| Human-readable brackets | `(01)04150000021126(17)250630(10)12345` |
| AIM prefix, DataMatrix | `]d2…` (also `]d1`) |
| AIM prefix, GS1-128 | `]C1…` |
| AIM prefix, QR / DotCode / composite | `]Q3…`, `]J1…`, `]e0…` |
| AIM prefix, EAN/UPC or ITF-14 | `]E0`–`]E4`, `]I0`–`]I2` |
| Bare GTIN (EAN-13, UPC-A, GTIN-14) | `7800038041425` → AI 01, zero-padded to 14 |
| Bare GTIN-8 | Requires `ParseOptions{AssumeBareGTIN8: true}` |

After scanner cleaning and recognized prefixes, a leading `(` selects
bracket notation for the whole input. Raw element strings preserve literal
parentheses in values: `10LOT(90)X` is one AI 10 with value `LOT(90)X`.
Use FNC1 to separate variable-length raw fields, rather than switching to
bracket notation partway through the input.

GS1 DataBar needs no special handling: scanners decode it to the same
element string as GS1-128.

## Command line

```bash
gs1 parse "0104150000021126172506302112345ABC"        # text table
gs1 parse -json -iso "(01)04150000021126(17)250200"   # JSON, ISO dates
gs1 parse -check-digits "0104150000021126"              # key check digits
gs1 parse -strict -check-digits "0104150000021126"      # both semantic checks
gs1 parse -gtin8 96385074                              # bare GTIN-8 instead of AI data
gs1 parse -validate anvisa "$SCAN"                     # exit 1 if non-compliant
cat scans.txt | gs1 parse -json                        # one JSON object per line
gs1 encode "(10)LOT42(01)04150000021126"               # raw string, FNC1 as ASCII GS
gs1 encode -hri "(10)LOT42(01)04150000021126"          # (01)04150000021126(10)LOT42
gs1 encode -zpl "(10)LOT42(01)04150000021126"          # >8010415000002112610LOT42
gs1 datamatrix -o dm.png "(01)04150000021126(10)ABC"   # GS1 DataMatrix, SVG without -o
gs1 epc 3074257BF7194E4000001A85                       # RFID tag to element string
gs1 epc -json "urn:epc:tag:sscc-96:3.0614141.1234567890"
gs1 gtin 04150000021126                                # GTIN check digit
gs1 key 106141411234567897                             # valid SSCC or GSRN
gs1 sscc -ext 3 -gcp 7801234 -serial 000000009         # 378012340000000095
gs1 gtin13 -gcp 7801234 -item 00005                    # 7801234000056
gs1 gtin14 -indicator 1 7801234000056                  # 17801234000053
gs1 ai 3102                                            # AI (3102)  Net Weight kg  N6
```

Exit codes: `0` success, `1` invalid input or failed validation, `2` usage error.

`-strict` validates AI associations only. `-check-digits` independently
enables check-digit validation for every numeric GS1 key (GTIN, SSCC, GLN,
GSIN, GSRN, GDTI, GRAI). `gs1 key` validates a key of unknown family and
names every family its length admits.

## WebAssembly

The same parser runs in browsers, Electron and Node, published to npm as
[`@galenzo17/gs1-wasm`](https://www.npmjs.com/package/@galenzo17/gs1-wasm)
with the version of the git tag and npm provenance:

```bash
npm install @galenzo17/gs1-wasm
```

```js
import { loadGS1 } from "@galenzo17/gs1-wasm";
const gs1 = await loadGS1(); // gs1.wasm and wasm_exec.js resolve next to the loader
```

The same files are attached to every GitHub release and served by jsDelivr
and unpkg for `<script type="module">` use without a build step; see
[`wasm/README.md`](wasm/README.md). To build locally:

```bash
make wasm   # produces wasm/gs1.wasm and a servable example under wasm/example/
make npm-check                   # loads the package through its own loader, lists what npm would ship
node wasm/check-digits.test.mjs  # optional Node runtime validation of that build
node wasm/encode.test.mjs        # same, for encode()
node wasm/epc.test.mjs           # same, for parseEPC()
node wasm/keys.test.mjs          # same, for validateKey and key check digits
node wasm/parity.test.mjs        # same, for measures, names, bare GTIN-8 and builders
```

```html
<script type="module">
  import { loadGS1 } from './gs1-loader.js';
  const gs1 = await loadGS1();

  const r = gs1.parse("0104150000021126172502001012345", {
    dateFormat: "iso", dayZero: "first", validateCheckDigits: true
  });
  r.gtin;            // "04150000021126"
  r.expirationDate;  // "2025-02-01"
  r.elements[0];     // { ai: "01", name: "GTIN", value: "04150000021126" }
  r.elements[1];     // { ai: "17", name: "Expiration Date", value: "250200", date: "2025-02-01" }
  r.warnings;         // parser advisories such as due-date-as-expiry

  gs1.encode([{ ai: "10", value: "LOT42" }, { ai: "01", value: "04150000021126" }]);
  // { raw: "\x1D010415000002112610LOT42", hri: "(01)04150000021126(10)LOT42", zpl: ">8010415000002112610LOT42" }

  gs1.parse("3102001500").elements[0].measure;   // { raw: "001500", value: 15, scaled: 1500, decimals: 2, unit: "kg" }
  gs1.parse("96385074", { assumeBareGTIN8: true }).gtin; // "00000096385074"

  gs1.newSSCC("3", "7801234", "000000009");      // "378012340000000095"
  const gtin13 = gs1.newGTIN13("7801234", "00005");   // "7801234000056"
  if (typeof gtin13 === "string") gs1.newGTIN14("1", gtin13); // "17801234000053"
  gs1.parseEPC("3074257BF7194E4000001A85");      // { gtin: "80614141123458", serial: "6789", epc: { scheme: "sgtin", bits: 96, filter: 3, ... } }

  gs1.validateGTIN("04150000021126");            // true
  gs1.validateKey("106141411234567897");         // { valid: true, keyTypes: ["SSCC", "GSRN"] }
  gs1.validateRegulatory(scan, "anvisa");       // null when compliant, else message
</script>
```

TypeScript declarations live in [`wasm/gs1.d.ts`](wasm/gs1.d.ts). Design
notes are in [ADR 0005](docs/adr/0005-webassembly-target.md).
The WASM `strict` option validates AI associations only; `validateCheckDigits`
independently checks the check digits of every numeric GS1 key and returns
the usual `{ error: string }` object on failure. `assumeBareGTIN8` mirrors
`ParseOptions.AssumeBareGTIN8` and the CLI `-gtin8` flag. Each element carries
the AI `name`, for measure AIs a `measure` object, and under `dateFormat: "iso"`
a `date` field, all with the same shape as the CLI JSON output. The builders `newSSCC`, `newGTIN13` and `newGTIN14` return
the key or an `{ error: string }` object. The optional Node test uses the matching `wasm_exec.js`
copied by `make wasm`; Node is not required by `make all`.

## Conformance

`make conformance` runs the test vectors that GS1 publishes with the
[Barcode Syntax Engine](https://github.com/gs1/gs1-syntax-engine) (1.4.1)
and the [Barcode Syntax Dictionary](https://github.com/gs1/gs1-syntax-dictionary)
(2026-01-27) through this parser, vendored under
[`testdata/gs1`](testdata/gs1/README.md) with their Apache 2.0 licences:
bracket-notation parsing, AI data validation, association rules, scan data
with symbology identifiers, and the dictionary linters this package
implements. Each divergence from the reference is listed with its reason in
[`known-divergences.txt`](testdata/gs1/known-divergences.txt): scanner
tolerance kept on purpose, value linters that run only through the explicit
options, and features such as unknown-AI passthrough or Digital Link URIs.
The test fails on any new divergence and on any listed one that starts to
agree, so the rate reported in CI moves only deliberately.

## Scope

This library is a parsing and validation layer whose `symbol` sub-package
also renders GS1 DataMatrix symbols
([ADR 0009](docs/adr/0009-symbol-generation.md)). It does not verify print
quality, resolve GS1 Digital Link URIs, or implement business documents such
as dispatch advices. See [ADR 0002](docs/adr/0002-parser-scope.md) for the
reasoning and the GS1 resources that cover those areas.

For guidance on connecting scanner transports, storing parsed fields, and
mapping records into dispatch advice workflows, see the
[integration guide](docs/integration-guide.md).

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
