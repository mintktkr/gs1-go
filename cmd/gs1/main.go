// Command gs1 parses GS1 Application Identifier element strings from the
// command line or from a stream of scanner output on stdin.
//
// Usage:
//
//	gs1 parse [-json] [-iso] [-strict] [-check-digits] [-gtin8] [-validate REGULATOR] [BARCODE]
//	gs1 encode [-hri] [-zpl] [ELEMENTS]
//	gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE
//	gs1 epc [-json] [-validate REGULATOR] [EPC]
//	gs1 gtin GTIN
//	gs1 key KEY
//	gs1 sscc -ext DIGIT -gcp PREFIX -serial REFERENCE
//	gs1 gtin13 -gcp PREFIX -item REFERENCE
//	gs1 gtin14 -indicator DIGIT GTIN13
//	gs1 ai CODE
//	gs1 version
//
// When BARCODE is omitted, parse reads one barcode per line from stdin
// until EOF, which suits keyboard-wedge scanners and shell pipelines. The
// encode command accepts element strings in bracket notation such as
// "(01)04150000021126(10)LOT42", validates them and prints the scanner
// equivalent string, its human-readable interpretation or a ZPL ^FD payload.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
)

// version, commit and date are set by the linker for release builds
// (GoReleaser ldflags); otherwise the version is derived from module build
// info.
var (
	version = ""
	commit  = ""
	date    = ""
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "parse":
		return runParse(args[1:], stdin, stdout, stderr)
	case "encode":
		return runEncode(args[1:], stdin, stdout, stderr)
	case "datamatrix":
		return runDataMatrix(args[1:], stdout, stderr)
	case "epc":
		return runEPC(args[1:], stdin, stdout, stderr)
	case "gtin":
		return runGTIN(args[1:], stdout, stderr)
	case "key":
		return runKey(args[1:], stdout, stderr)
	case "sscc":
		return runSSCC(args[1:], stdout, stderr)
	case "gtin13":
		return runGTIN13(args[1:], stdout, stderr)
	case "gtin14":
		return runGTIN14(args[1:], stdout, stderr)
	case "ai":
		return runAI(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "gs1", resolveVersion())
		return 0
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "gs1: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `gs1 - parse and validate GS1 barcode element strings

Usage:
  gs1 parse [-json] [-iso] [-strict] [-check-digits] [-gtin8] [-validate REGULATOR] [BARCODE]
  gs1 encode [-hri] [-zpl] [ELEMENTS]
  gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE
  gs1 epc [-json] [-validate REGULATOR] [EPC]
  gs1 gtin GTIN
  gs1 key KEY
  gs1 sscc -ext DIGIT -gcp PREFIX -serial REFERENCE
  gs1 gtin13 -gcp PREFIX -item REFERENCE
  gs1 gtin14 -indicator DIGIT GTIN13
  gs1 ai CODE
  gs1 version

Commands:
  parse    Parse a GS1-128 / DataMatrix element string. Reads stdin line by
           line when BARCODE is omitted.
  encode   Build the element string for a barcode generator from bracket
           notation such as "(01)04150000021126(10)LOT42": fixed-length AIs
           first, FNC1 (ASCII GS) where required, association rules and
           check digits enforced. -hri prints the human-readable
           interpretation, -zpl a ^FD payload with >8 FNC1 escapes. Reads
           stdin line by line when ELEMENTS is omitted.
  datamatrix
           Render a GS1 DataMatrix symbol as SVG (stdout, or -o FILE) or as
           PNG when FILE ends in .png.
  epc      Decode an RFID tag (EPC Tag Data Standard hexadecimal memory
           content, tag URI or pure-identity URI) into the equivalent
           element string: SGTIN, SSCC, SGLN, GRAI and GIAI. Reads stdin
           line by line when EPC is omitted.
  gtin     Validate the check digit of a GTIN-8/12/13/14.
  key      Validate the check digit of any numeric GS1 key (GTIN, SSCC, GLN,
           GSIN, GSRN) and report the key types its length admits.
  sscc     Build an 18-digit Serial Shipping Container Code.
  gtin13   Build a GTIN-13 from a company prefix and an item reference.
  gtin14   Build a case-level GTIN-14 from an indicator digit and a GTIN-13.
  ai       Show name and format of an Application Identifier.
  version  Print the build version.

Regulators for -validate: anvisa, anmat, snfa, cofepris
`)
}

// element is the JSON shape for one AI/value pair.
type element struct {
	AI      string         `json:"ai"`
	Name    string         `json:"name,omitempty"`
	Value   string         `json:"value"`
	Date    string         `json:"date,omitempty"`
	Measure *measureOutput `json:"measure,omitempty"`
}

type measureOutput struct {
	Raw      string  `json:"raw"`
	Value    float64 `json:"value"`
	Scaled   int64   `json:"scaled"`
	Decimals int     `json:"decimals"`
	Unit     string  `json:"unit"`
}

type parseOutput struct {
	Raw               string        `json:"raw"`
	Elements          []element     `json:"elements"`
	EPC               *epcOutput    `json:"epc,omitempty"`
	ContentGTIN       string        `json:"contentGtin,omitempty"`
	CountOfTradeItems string        `json:"countOfTradeItems,omitempty"`
	GLN               string        `json:"gln,omitempty"`
	GSIN              string        `json:"gsin,omitempty"`
	PackagingDate     string        `json:"packagingDate,omitempty"`
	Error             string        `json:"error,omitempty"`
	Symbology         string        `json:"symbology"`
	Warnings          []gs1.Warning `json:"warnings,omitempty"`
}

// epcOutput is the JSON shape of a decoded EPC identifier.
type epcOutput struct {
	Scheme        string `json:"scheme"`
	Bits          int    `json:"bits,omitempty"`
	Filter        *int   `json:"filter,omitempty"`
	CompanyPrefix string `json:"companyPrefix"`
	Reference     string `json:"reference"`
	Serial        string `json:"serial,omitempty"`
	URI           string `json:"uri"`
}

type parseOptions struct {
	json        bool
	iso         bool
	strict      bool
	checkDigits bool
	gtin8       bool
	validate    string
	regulator   *gs1.Regulator
}

var regulators = map[string]gs1.Regulator{
	"anvisa":   gs1.ANVISA,
	"anmat":    gs1.ANMAT,
	"snfa":     gs1.SNFA,
	"cofepris": gs1.COFEPRIS,
}

func runParse(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts parseOptions
	fs.BoolVar(&opts.json, "json", false, "emit one JSON object per barcode")
	fs.BoolVar(&opts.iso, "iso", false, "render date AIs (11, 12, 13, 15, 17) as ISO 8601")
	fs.BoolVar(&opts.strict, "strict", false, "validate AI association rules")
	fs.BoolVar(&opts.checkDigits, "check-digits", false, "validate GS1 key check digits (GTIN, SSCC, GLN, GSIN, GSRN, GDTI, GRAI)")
	fs.BoolVar(&opts.gtin8, "gtin8", false, "treat unprefixed eight-digit numeric input as a bare GTIN-8")
	fs.StringVar(&opts.validate, "validate", "", "check required AIs for a regulator: anvisa, anmat, snfa, cofepris")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if opts.validate != "" {
		reg, ok := regulators[strings.ToLower(opts.validate)]
		if !ok {
			fmt.Fprintf(stderr, "gs1: unknown regulator %q (valid: anvisa, anmat, snfa, cofepris)\n", opts.validate)
			return 2
		}
		opts.regulator = &reg
	}

	switch fs.NArg() {
	case 0:
		return parseStream(stdin, stdout, stderr, opts)
	case 1:
		var b gs1.Barcode
		if err := parseOne(fs.Arg(0), &b, stdout, stderr, opts); err != nil {
			fmt.Fprintln(stderr, "gs1:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "gs1: parse accepts at most one barcode; pipe multiple barcodes on stdin")
		return 2
	}
}

// parseStream reads one barcode per line, reusing a single Barcode.
// Per-line failures are reported and do not stop the stream; the exit code
// is non-zero if any line failed.
func parseStream(stdin io.Reader, stdout, stderr io.Writer, opts parseOptions) int {
	sc := bufio.NewScanner(stdin)
	var b gs1.Barcode
	failed := false
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !first && !opts.json {
			fmt.Fprintln(stdout, "---")
		}
		first = false
		b.Reset()
		if err := parseOne(line, &b, stdout, stderr, opts); err != nil {
			failed = true
			if opts.json {
				_ = json.NewEncoder(stdout).Encode(parseOutput{Raw: line, Error: err.Error()})
			} else {
				fmt.Fprintln(stderr, "gs1:", err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(stderr, "gs1: reading stdin:", err)
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

func parseOne(input string, b *gs1.Barcode, stdout, stderr io.Writer, opts parseOptions) error {
	if err := gs1.ParseIntoWithOptions(input, b, gs1.ParseOptions{
		AssumeBareGTIN8:      opts.gtin8,
		ValidateAssociations: opts.strict,
		ValidateCheckDigits:  opts.checkDigits,
	}); err != nil {
		return err
	}
	return renderBarcode(b, nil, stdout, stderr, opts)
}

// renderBarcode applies the regulator check and writes the barcode in the
// text or JSON format shared by parse and epc.
func renderBarcode(b *gs1.Barcode, epc *epcOutput, stdout, stderr io.Writer, opts parseOptions) error {
	if opts.regulator != nil {
		if err := opts.regulator.Validate(*b); err != nil {
			return err
		}
	}
	packagingDate, _ := b.Get("13")
	warnings := b.Warnings()
	out := parseOutput{
		Raw:               b.Raw,
		Elements:          make([]element, 0, len(b.Elements)),
		EPC:               epc,
		Symbology:         b.Symbology.String(),
		ContentGTIN:       b.ContentGTIN(),
		CountOfTradeItems: b.CountOfTradeItems(),
		GLN:               b.GLN(),
		GSIN:              b.GSIN(),
		PackagingDate:     packagingDate,
		Warnings:          warnings,
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "gs1: warning: %s\n", warning.Message)
	}
	if opts.iso {
		if t, err := b.PackagingDate(); err == nil {
			out.PackagingDate = t.Format("2006-01-02")
		} else {
			out.PackagingDate = ""
		}
	}
	out.Elements = buildElements(b, opts)
	if opts.json {
		return json.NewEncoder(stdout).Encode(out)
	}
	if epc != nil {
		if _, err := fmt.Fprintf(stdout, "%s %s\n", epcSchemeLabel(*epc), epc.URI); err != nil {
			return err
		}
	}
	return writeTextOutput(stdout, out.Elements)
}

func epcSchemeLabel(epc epcOutput) string {
	label := strings.ToUpper(epc.Scheme)
	if epc.Bits > 0 {
		label += "-" + strconv.Itoa(epc.Bits)
	}
	return label
}

func runEPC(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 epc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts parseOptions
	fs.BoolVar(&opts.json, "json", false, "emit one JSON object per tag")
	fs.StringVar(&opts.validate, "validate", "", "check required AIs for a regulator: anvisa, anmat, snfa, cofepris")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if opts.validate != "" {
		reg, ok := regulators[strings.ToLower(opts.validate)]
		if !ok {
			fmt.Fprintf(stderr, "gs1: unknown regulator %q (valid: anvisa, anmat, snfa, cofepris)\n", opts.validate)
			return 2
		}
		opts.regulator = &reg
	}
	switch fs.NArg() {
	case 0:
		return epcStream(stdin, stdout, stderr, opts)
	case 1:
		if err := epcOne(fs.Arg(0), stdout, stderr, opts); err != nil {
			fmt.Fprintln(stderr, "gs1:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "gs1: epc accepts at most one tag; pipe multiple tags on stdin")
		return 2
	}
}

func epcStream(stdin io.Reader, stdout, stderr io.Writer, opts parseOptions) int {
	sc := bufio.NewScanner(stdin)
	failed := false
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !first && !opts.json {
			fmt.Fprintln(stdout, "---")
		}
		first = false
		if err := epcOne(line, stdout, stderr, opts); err != nil {
			failed = true
			if opts.json {
				_ = json.NewEncoder(stdout).Encode(parseOutput{Raw: line, Error: err.Error()})
			} else {
				fmt.Fprintln(stderr, "gs1:", err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(stderr, "gs1: reading stdin:", err)
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

// epcOne decodes a hexadecimal tag or an EPC URI and renders it like parse.
func epcOne(input string, stdout, stderr io.Writer, opts parseOptions) error {
	var epc gs1.EPC
	var err error
	if strings.HasPrefix(input, "urn:") {
		epc, err = gs1.DecodeEPCURI(input)
	} else {
		epc, err = gs1.DecodeEPC(input)
	}
	if err != nil {
		return err
	}
	b := gs1.Barcode{Raw: input, Symbology: gs1.SymEPC, Elements: epc.Elements()}
	out := epcOutput{Scheme: epc.Scheme.String(), Bits: epc.Bits, CompanyPrefix: epc.CompanyPrefix, Reference: epc.Reference, Serial: epc.Serial, URI: epc.URI()}
	if epc.Filter >= 0 {
		filter := epc.Filter
		out.Filter = &filter
	}
	return renderBarcode(&b, &out, stdout, stderr, opts)
}

// buildElements converts parsed elements into their output representation.
func buildElements(b *gs1.Barcode, opts parseOptions) []element {
	elements := make([]element, 0, len(b.Elements))
	for _, e := range b.Elements {
		el := element{AI: e.AI, Value: e.Value}
		if ai, ok := gs1.LookupAI(e.AI); ok {
			el.Name = ai.Name
		}
		if measure, ok := measureForAI(e.AI, e.Value); ok {
			el.Measure = &measureOutput{
				Raw: measure.Raw, Value: measure.Value, Scaled: measure.Scaled,
				Decimals: measure.Decimals, Unit: measure.Unit,
			}
		}
		if opts.iso && isDateAI(e.AI) {
			if t, err := gs1.ParseDate(e.Value); err == nil {
				el.Date = t.Format("2006-01-02")
			}
		}
		elements = append(elements, el)
	}
	return elements
}

// writeTextOutput renders elements in the human-readable text format.
func writeTextOutput(stdout io.Writer, elements []element) error {
	for _, el := range elements {
		v := el.Value
		if el.Date != "" {
			v = el.Date
		} else if el.Measure != nil {
			v = formatMeasure(*el.Measure)
		}
		if _, err := fmt.Fprintf(stdout, "(%s) %-20s %s\n", el.AI, v, el.Name); err != nil {
			return err
		}
	}
	return nil
}

func measureForAI(ai, value string) (gs1.Measure, bool) {
	if len(ai) != 4 {
		return gs1.Measure{}, false
	}
	b := gs1.Barcode{Elements: []gs1.Element{{AI: ai, Value: value}}}
	return b.Measure(ai[:3])
}

func formatMeasure(measure measureOutput) string {
	return fmt.Sprintf("%.*f %s", measure.Decimals, measure.Value, measure.Unit)
}

func isDateAI(ai string) bool {
	switch ai {
	case "11", "12", "13", "15", "17":
		return true
	}
	return false
}

type encodeOptions struct {
	hri bool
	zpl bool
}

func runEncode(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 encode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts encodeOptions
	fs.BoolVar(&opts.hri, "hri", false, "print the human-readable interpretation, e.g. (01)04150000021126(10)LOT42")
	fs.BoolVar(&opts.zpl, "zpl", false, "print ZPL ^FD field data for ^BC with >8 FNC1 escapes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if opts.hri && opts.zpl {
		fmt.Fprintln(stderr, "gs1: -hri and -zpl are mutually exclusive")
		return 2
	}

	switch fs.NArg() {
	case 0:
		return encodeStream(stdin, stdout, stderr, opts)
	case 1:
		if err := encodeOne(fs.Arg(0), stdout, opts); err != nil {
			fmt.Fprintln(stderr, "gs1:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "gs1: encode accepts at most one element string; pipe multiple element strings on stdin")
		return 2
	}
}

// encodeStream encodes one element string per line. Failures are reported
// and do not stop the stream; the exit code is non-zero if any line failed.
func encodeStream(stdin io.Reader, stdout, stderr io.Writer, opts encodeOptions) int {
	sc := bufio.NewScanner(stdin)
	failed := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := encodeOne(line, stdout, opts); err != nil {
			failed = true
			fmt.Fprintln(stderr, "gs1:", err)
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(stderr, "gs1: reading stdin:", err)
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

// encodeOne reads bracket notation or a raw scan string, re-encodes it
// through the strict encoder and prints the requested representation.
func encodeOne(input string, stdout io.Writer, opts encodeOptions) error {
	elements, err := encodeInput(input)
	if err != nil {
		return err
	}
	switch {
	case opts.zpl:
		zpl, err := gs1.EncodeZPL(elements)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, zpl)
		return err
	case opts.hri:
		raw, err := gs1.Encode(elements)
		if err != nil {
			return err
		}
		ordered, err := gs1.Parse(raw)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, ordered.HRI())
		return err
	default:
		raw, err := gs1.Encode(elements)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, raw)
		return err
	}
}

// encodeInput accepts bracket notation, split literally by ParseHRI so that
// the AIs are exactly what the operator typed, or a raw scan string.
func encodeInput(input string) ([]gs1.Element, error) {
	if strings.HasPrefix(input, "(") {
		return gs1.ParseHRI(input)
	}
	parsed, err := gs1.Parse(input)
	if err != nil {
		return nil, err
	}
	return parsed.Elements, nil
}

func runDataMatrix(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 datamatrix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "write to `FILE`; PNG when it ends in .png, SVG otherwise")
	scale := fs.Int("scale", 10, "pixels per module")
	quiet := fs.Int("quiet", 1, "quiet zone in modules")
	rect := fs.Bool("rect", false, "use a rectangular symbol")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE")
		return 2
	}
	elements, err := encodeInput(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	m, err := symbol.GS1DataMatrix(elements, symbol.DataMatrixOptions{Rectangular: *rect})
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	if *out == "" {
		fmt.Fprint(stdout, m.SVG(*scale, *quiet))
		return 0
	}
	if err := writeSymbol(*out, m, *scale, *quiet); err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	return 0
}

// writeSymbol writes m to path as PNG when the extension is .png and as SVG
// otherwise.
func writeSymbol(path string, m *symbol.Matrix, scale, quiet int) error {
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(path), ".png") {
		err = m.PNG(f, scale, quiet)
	} else {
		_, err = io.WriteString(f, m.SVG(scale, quiet))
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func runGTIN(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: gs1 gtin GTIN")
		return 2
	}
	if err := gs1.ValidateGTIN(args[0]); err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		if errors.Is(err, gs1.ErrInvalidCheckDigit) {
			return 1
		}
		return 2
	}
	fmt.Fprintln(stdout, "valid")
	return 0
}

// runKey validates a numeric key whose family is unknown and names every
// family its length admits, since GTIN-13 and GLN, or SSCC and GSRN, share
// the same digits and check-digit algorithm.
func runKey(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: gs1 key KEY")
		return 2
	}
	types, err := gs1.ValidateKey(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		if errors.Is(err, gs1.ErrInvalidCheckDigit) {
			return 1
		}
		return 2
	}
	fmt.Fprintln(stdout, "valid", formatKeyTypes(types))
	return 0
}

func formatKeyTypes(types []gs1.KeyType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.String()
	}
	return strings.Join(names, " or ")
}

func runSSCC(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 sscc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	extension := fs.String("ext", "", "SSCC extension digit")
	companyPrefix := fs.String("gcp", "", "GS1 company prefix")
	serialReference := fs.String("serial", "", "serial reference")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || len(*extension) != 1 {
		fmt.Fprintln(stderr, "usage: gs1 sscc -ext DIGIT -gcp PREFIX -serial REFERENCE")
		return 2
	}
	sscc, err := gs1.NewSSCC((*extension)[0], *companyPrefix, *serialReference)
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	fmt.Fprintln(stdout, sscc)
	return 0
}

func runGTIN13(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 gtin13", flag.ContinueOnError)
	fs.SetOutput(stderr)
	companyPrefix := fs.String("gcp", "", "GS1 company prefix")
	itemReference := fs.String("item", "", "item reference")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *companyPrefix == "" || *itemReference == "" {
		fmt.Fprintln(stderr, "usage: gs1 gtin13 -gcp PREFIX -item REFERENCE")
		return 2
	}
	gtin, err := gs1.NewGTIN13(*companyPrefix, *itemReference)
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	fmt.Fprintln(stdout, gtin)
	return 0
}

func runGTIN14(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gs1 gtin14", flag.ContinueOnError)
	fs.SetOutput(stderr)
	indicator := fs.String("indicator", "", "packaging level indicator digit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || len(*indicator) != 1 {
		fmt.Fprintln(stderr, "usage: gs1 gtin14 -indicator DIGIT GTIN13")
		return 2
	}
	gtin, err := gs1.NewGTIN14((*indicator)[0], fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "gs1:", err)
		return 1
	}
	fmt.Fprintln(stdout, gtin)
	return 0
}

func runAI(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: gs1 ai CODE")
		return 2
	}
	ai, ok := gs1.LookupAI(args[0])
	if !ok {
		fmt.Fprintf(stderr, "gs1: unknown application identifier %q\n", args[0])
		return 1
	}
	fmt.Fprintf(stdout, "AI (%s)\t%s\t%s\n", ai.Code, ai.Name, ai.Format)
	return 0
}

func resolveVersion() string {
	if version != "" {
		if commit != "" {
			return fmt.Sprintf("%s (%s, %s)", version, commit, date)
		}
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}
