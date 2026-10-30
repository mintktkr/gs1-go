# GS1 solution-provider certification matrix

This document maps every item of the GS1 LATAM solution-provider
certification checklist to the state of this library. It is the source of
truth for the [GS1 certification readiness milestone](https://github.com/galenzo17/gs1-go/milestone/1)
and the tracking issue [#22](https://github.com/galenzo17/gs1-go/issues/22).

The checklist audits a complete solution: an application plus the parsing
layer it is built on. This module is only the parsing and validation layer
(see [ADR 0002](adr/0002-parser-scope.md)), so each row states who owns the
requirement.

## Status legend

| Status | Meaning |
|---|---|
| **Covered** | Implemented and tested in this module. |
| **Gap** | Belongs to this module and is missing or partial. Linked issue tracks it. |
| **Application layer** | Belongs to the application built on top of the module. The library provides the data; the integration guide (#20) documents how to use it. |
| **Out of scope** | Not addressed by this module or by its documentation. Pointers to GS1 resources are given. |

Type follows the checklist: Required, Recommended, Optional.

## 1. GS1 keys and Application Identifiers

The checklist asks that every key is accepted with its length, data type and,
where applicable, check digit validated.

| # | Item | AI | Type | Status | Notes / issue |
|---|---|---|---|---|---|
| 1.1 | GTIN-13 for trade items | 01, 02 | Required | Covered | Length and numeric type validated on parse. Check digit via `ValidateGTIN`, or on parse with `ParseOptions.ValidateCheckDigits` (#6). |
| 1.2 | GTIN-14 at case level | 01, 02 | Required | Covered | Same as 1.1. |
| 1.3 | GTIN-12 (UPC-A) | 01, 02 | Required | Covered | Bare 12-digit input zero-padded to GTIN-14. |
| 1.4 | GTIN-8 (EAN-8) | 01, 02 | Required | Covered | As AI (01) data it parses. Under the `]E4` identifier it is an EAN-8 carrier; bare 8-digit input is ambiguous with AI data, so it needs `ParseOptions.AssumeBareGTIN8` (CLI `-gtin8`, WASM `assumeBareGTIN8`) (#15). |
| 1.5 | Batch/lot, alphanumeric up to 20 | 10 | Required | Covered | `Lot()`. |
| 1.6 | Expiration date YYMMDD | 17 | Required | Covered | `ExpirationDate()`. AI (12) parses with `DueDate()` and raises the `WarnDueDateAsExpiry` warning (#9). |
| 1.7 | Production date | 11 | Required | Covered | `ProductionDate()`. |
| 1.8 | Packaging date | 13 | Required | Covered | `PackagingDate()` (#12). |
| 1.9 | Best before date | 15 | Required | Covered | `BestBeforeDate()`. |
| 1.10 | SSCC, 17 digits + check | 00 | Required | Covered | `SSCC()`, `ValidateSSCC`, `ValidateCheckDigits` on parse (#7), `NewSSCC` builder (#17). |
| 1.11 | Net weight kg, 6 digits | 310n | Required | Covered | `NetWeightKg()` returns a `Measure` with the implied decimal point applied (#11). |
| 1.12 | Gross weight kg | 330n | Required | Covered | `GrossWeightKg()` (#11). |
| 1.13 | Net weight lb | 320n | Required | Covered | `NetWeightLb()` (#11). |
| 1.14 | Gross weight lb | 340n | Required | Covered | `GrossWeightLb()` (#11). |
| 1.15 | Count, up to 8 digits | 37 | Required | Covered | `CountOfTradeItems()` (#12). |
| 1.16 | GLN, 12 digits + check | 414 | Required | Covered | Whole GLN family parses: 410–417, 254 and 7040 landed in #27 (closes #10). `ValidateGLN` and `ValidateCheckDigits` cover 410–417 (#7); `GLN()` accessor (#12). |
| 1.17 | Customer part number | 241 | Required | Covered | |
| 1.18 | Serial number | 21 | Required | Covered | `SerialNumber()`. |
| 1.19 | GSIN, 16 digits + check | 402 | Required | Covered | `ValidateGSIN` and `ValidateCheckDigits` (#7); `GSIN()` accessor (#12). |
| 1.20 | Internal AIs 90–99 | 90–99 | Required | Covered | 90 is X..30, 91–99 are X..90. |
| 1.21 | GDTI | 253 | Recommended | Covered | Landed in #32 (closes #8) with `GDTI()`. `ValidateCheckDigits` checks the 13-digit key prefix (#7). |
| 1.22 | GRAI | 8003 | Recommended | Covered | Landed in #32 with `GRAI()`. `ValidateCheckDigits` checks the 13-digit key after the leading zero (#7). |
| 1.23 | GIAI | 8004 | Recommended | Covered | Landed in #32. No check digit by specification. |
| 1.24 | GSRN | 8018 | Recommended | Covered | 8017 and 8018 landed in #32. `ValidateGSRN` and `ValidateCheckDigits` (#7). |
| 1.25 | GINC | 401 | Recommended | Covered | Landed in #32. No check digit by specification. |

Cross-cutting: the AI table is generated from the GS1 Barcode Syntax
Dictionary release 2026-01-27 (541 AIs, #14), and the association rules are
the dictionary's `req`/`ex` attributes evaluated for every AI (#13, #14). A
newer dictionary release is a file replacement plus `make generate`.
Check-digit validation is opt-in by design, through
`ParseOptions.ValidateCheckDigits` or the `Validate*` helpers, so resilient
parsing of noisy scanner output stays the default (ADR 0008, ADR 0010).

## 2. Symbologies

Apart from the `symbol` sub-package (row 3.5), the module never touches
pixels or bars. It consumes what the scanner
decodes, so "supports symbology X" means: accepts the decoded string, with
or without its AIM symbology identifier, and splits it correctly.

| # | Item | Type | Status | Notes / issue |
|---|---|---|---|---|
| 2.1 | EAN-13 | Required | Covered | Bare 13 digits and the `]E0` identifier parse; `Barcode.Symbology` reports `SymEANUPC` (#15). |
| 2.2 | ITF-14 | Required | Covered | Bare 14 digits and the `]I0`, `]I1` and `]I2` identifiers parse; `Barcode.Symbology` reports `SymITF14` and a stripped check digit under `]I2` is recomputed (#15). |
| 2.3 | GS1-128 with its AIs | Required | Covered | `]C1`, FNC1 as ASCII 29, missing-FNC1 recovery. |
| 2.4 | GS1 DataMatrix with its AIs | Required | Covered | `]d1`, `]d2`. |
| 2.5 | GS1 DataBar family | Required | Covered | `]e0`. Decodes to the same element string as GS1-128. |
| 2.6 | EAN-8 | Required | Covered | `]E4` identifier, or bare input with `AssumeBareGTIN8`; see 1.4 (#15). |
| 2.7 | UPC-A | Required | Covered | See 2.1. |
| 2.8 | UPC-E | Required | Covered | `ExpandUPCE`; an eight-digit payload under `]E0` is expanded to UPC-A automatically (#15). |
| 2.9 | GS1 QR Code | Recommended | Covered | `]Q3`. |
| 2.10 | EPC / RFID tag | Recommended | Covered | `ParseEPC` and `ParseEPCURI` decode SGTIN-96/198, SSCC-96, SGLN-96/195, GRAI-96/170 and GIAI-96/202 into the equivalent element string (#19); CLI `gs1 epc`, WASM `parseEPC()`. Writing tags is out of scope. |

Printing symbols (checklist wording "impresión") is covered under 3.5 and tracked in #24.

## 3. Reading and processing Application Identifiers

| # | Item | Type | Status | Notes / issue |
|---|---|---|---|---|
| 3.1 | Compatible with Wi-Fi data capture devices (mobile computers, not only tethered scanners) | Required | Application layer | The parser is transport-agnostic: HID text, TCP streams and HTTP posts all feed `ParseInto`. Documented in the integration guide (#20). |
| 3.2 | Read and process AIs: split and store internally | Required | Covered | Elements in scan order, `Get`, typed accessors. Full AI table and association rules generated from the Syntax Dictionary (#13, #14). |
| 3.3 | Separate and store each AI as its own field, e.g. (02) GTIN, (10) lot, (17) expiry, (37) count | Required | Covered | `Barcode.Elements` plus accessors. Dates return `time.Time`. |
| 3.4 | Place, manage and interpret FNC1 after variable-length AIs | Required | Covered | Reading side complete. Writing side covered by `Encode` and `EncodeZPL`, which place FNC1 after every variable-length AI that is not last (#16). |
| 3.5 | Print, read and interpret 2D symbologies; print simple and logistic labels | Required | Gap, other symbologies and label layout | Reading is covered. Building the element string with correct AI order, FNC1 placement, association rules and check digits, plus HRI text and ZPL `^FD` data, is covered by `Encode`, `EncodeZPL`, `gs1 encode` and WASM `encode()` (#16). GS1 DataMatrix rendering is covered by the `symbol` package (`Image`, `PNG`, `SVG`) and `gs1 datamatrix` per ADR 0009 (#24); GS1-128, GS1 QR, EAN/UPC and ITF-14 symbols and the logistic label layout remain in #24. |
| 3.6 | Samples and logistic labels validated with the local GS1 verifier | Required | Covered (data), print verification pending | Data conformance: `TestConformance` runs the GS1 Syntax Engine 1.4.1 and Syntax Dictionary 2026-01-27 vectors (#18), with documented divergences; `testdata/labels/` holds the logistic label element strings per carrier. Once #24 ships, its renderers will provide the printed samples for the GS1 member organisation's verifier (ISO/IEC 15415/15416); the resulting reports go in `testdata/labels/`. |

## 4. Reports compatible with global standards

All rows belong to the application. The library's job is to hand over
normalised keys so the application can store, filter and print them.

| # | Item | Type | Status | Notes / issue |
|---|---|---|---|---|
| 4.1 | Reports show GS1 keys across the product hierarchy | Required | Application layer | Hierarchy SSCC → (02)+(37) → item GTIN. Builders in #17, guide in #20. |
| 4.2 | GS1 keys as primary identifiers on dispatch notes, purchase orders, invoices | Required | Application layer | Store GTIN-14 canonical form from `GTIN()`. Guide in #20. |
| 4.3 | Reports filtered by GTIN, lot, dates, serial, SSCC | Required | Application layer | Recommended column set in #20. |
| 4.4 | Generate, send or receive a packing list with SSCC, GTIN-14, lot, dates, GTIN-13, quantities | Required | Application layer | Mapping to GS1 XML DespatchAdvice / EANCOM DESADV in #20. |
| 4.5 | Ingest a received packing list without manual typing | Required | Application layer | Same mapping in reverse; parser handles the barcode side, EDI handles the document side. |

## 5. EDI-GS1 compatibility (second phase)

Out of scope for this module. The integration guide (#20) includes a
pointer table so auditors can see where each item lives in a complete
solution.

| # | Item | Type | Status |
|---|---|---|---|
| 5.1 | WebService transport | Optional | Out of scope |
| 5.2 | AS2 | Optional | Out of scope |
| 5.3 | AS4 | Optional | Out of scope |
| 5.4 | VAN | Optional | Out of scope |
| 5.5 | webEDI | Optional | Out of scope |
| 5.6 | GS1 XML 3.2 / EANCOM syntax 4 concepts | Optional | Out of scope |
| 5.7 | GDSN catalogue synchronisation | Optional | Out of scope |
| A–L | EDI documents: ORDERS, INVOIC, RECADV, DESADV, ORDCHG, RETANN, CONTRL, APERAK, REMADV, ORDRSP, RETINS, PRICAT | Optional (RECADV, DESADV Recommended) | Out of scope |

Reference implementations: GS1 XML 3.x schemas and EANCOM at
<https://www.gs1.org/standards/edi>, GDSN at <https://www.gs1.org/services/gdsn>.

## Summary

| Section | Required rows | Covered | Gap | Application layer / out of scope |
|---|---|---|---|---|
| 1. Keys | 20 | 20 | 0 | 0 |
| 2. Symbologies | 8 | 8 | 0 | 0 |
| 3. Processing | 6 | 4 (3.6 data only) | 1 | 1 |
| 4. Reports | 5 | 0 | 0 | 5 |
| 5. EDI | 0 | 0 | 0 | all |

Rows marked "Covered" are implemented and tested in this module. Row 3.6
counts as covered on the data side only: its print verification waits on
#24. The one Required gap left is rendering of the remaining symbologies
and the logistic label layout (3.5, #24), which will also supply the printed
samples for 3.6. The conformance job (#18) is green and the CLI and
WebAssembly bindings expose every covered feature except symbol rendering,
which reaches WebAssembly later in #24 (#21), so the module side of the checklist is
complete once #24 ships; the remaining rows are demonstrated by the
application during the audit using the integration guide.

## Keeping this file current

Last reviewed: 2026-10-27, after #51 merged. Update this file in the same pull request that closes an issue, so the matrix never lags the code.

## Working with the milestone

- Issues carry `gs1-certification` plus a `priority:` label mirroring the
  checklist type and an `area:` label for the code they touch.
- `application-layer` marks items the module documents but does not
  implement.
