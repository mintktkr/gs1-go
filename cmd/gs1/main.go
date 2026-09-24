// Command gs1 parses GS1 Application Identifier element strings from the
// command line or from a stream of scanner output on stdin.
//
// Usage:
//
//	gs1 parse [-json] [-iso] [-validate REGULATOR] [BARCODE]
//	gs1 gtin GTIN
//	gs1 ai CODE
//	gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE
//	gs1 version
//
// When BARCODE is omitted, parse reads one barcode per line from stdin
// until EOF, which suits keyboard-wedge scanners and shell pipelines.
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
	"strings"

	"github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
)

// version is set by the linker for release builds; otherwise it is derived
// from module build info.
var version = ""

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
	case "gtin":
		return runGTIN(args[1:], stdout, stderr)
	case "ai":
		return runAI(args[1:], stdout, stderr)
	case "datamatrix":
		return runDataMatrix(args[1:], stdout, stderr)
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
  gs1 parse [-json] [-iso] [-validate REGULATOR] [BARCODE]
  gs1 gtin GTIN
  gs1 ai CODE
  gs1 datamatrix [-o FILE] [-scale N] [-quiet N] [-rect] BARCODE
  gs1 version

Commands:
  parse    Parse a GS1-128 / DataMatrix element string. Reads stdin line by
           line when BARCODE is omitted.
  gtin     Validate the check digit of a GTIN-8/12/13/14.
  ai       Show name and format of an Application Identifier.
  datamatrix
           Render a GS1 DataMatrix symbol as SVG (stdout, or -o FILE) or as
           PNG when FILE ends in .png.
  version  Print the build version.

Regulators for -validate: anvisa, anmat, snfa, cofepris
`)
}

// element is the JSON shape for one AI/value pair.
type element struct {
	AI    string `json:"ai"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value"`
	Date  string `json:"date,omitempty"`
}

type parseOutput struct {
	Raw      string    `json:"raw"`
	Elements []element `json:"elements"`
	Error    string    `json:"error,omitempty"`
}

type parseOptions struct {
	json      bool
	iso       bool
	validate  string
	regulator *gs1.Regulator
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
	fs.BoolVar(&opts.iso, "iso", false, "render date AIs (11, 13, 15, 17) as ISO 8601")
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
		if err := parseOne(fs.Arg(0), &b, stdout, opts); err != nil {
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
		if err := parseOne(line, &b, stdout, opts); err != nil {
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

func parseOne(input string, b *gs1.Barcode, stdout io.Writer, opts parseOptions) error {
	if err := gs1.ParseInto(input, b); err != nil {
		return err
	}
	if opts.regulator != nil {
		if err := opts.regulator.Validate(*b); err != nil {
			return err
		}
	}
	out := parseOutput{Raw: b.Raw, Elements: make([]element, 0, len(b.Elements))}
	for _, e := range b.Elements {
		el := element{AI: e.AI, Value: e.Value}
		if ai, ok := gs1.LookupAI(e.AI); ok {
			el.Name = ai.Name
		}
		if opts.iso && isDateAI(e.AI) {
			if t, err := gs1.ParseDate(e.Value); err == nil {
				el.Date = t.Format("2006-01-02")
			}
		}
		out.Elements = append(out.Elements, el)
	}
	if opts.json {
		return json.NewEncoder(stdout).Encode(out)
	}
	for _, el := range out.Elements {
		v := el.Value
		if el.Date != "" {
			v = el.Date
		}
		if _, err := fmt.Fprintf(stdout, "(%s) %-20s %s\n", el.AI, v, el.Name); err != nil {
			return err
		}
	}
	return nil
}

func isDateAI(ai string) bool {
	switch ai {
	case "11", "13", "15", "17":
		return true
	}
	return false
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
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
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
	m, err := symbol.GS1DataMatrix(fs.Arg(0), symbol.DataMatrixOptions{Rectangular: *rect})
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
