package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func exec(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestParseText(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "0104150000021126172506301012345")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"(01) 04150000021126", "GTIN", "(17) 250630", "(10) 12345", "Batch/Lot"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestParseJSONWithISODates(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "-json", "-iso", "0104150000021126172502001012345")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got.Elements) != 3 {
		t.Fatalf("elements = %d, want 3", len(got.Elements))
	}
	if got.Elements[1].Date != "2025-02-28" {
		t.Errorf("date = %q, want 2025-02-28", got.Elements[1].Date)
	}
}

func TestParseInvalid(t *testing.T) {
	code, _, errOut := exec(t, "", "parse", "8812345")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown application identifier") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestParseValidateRegulator(t *testing.T) {
	full := "0104150000021126172506302112345ABC\x1D10LOT42X"
	if code, _, _ := exec(t, "", "parse", "-validate", "anvisa", full); code != 0 {
		t.Errorf("ANVISA pass: exit %d", code)
	}
	code, _, errOut := exec(t, "", "parse", "-validate", "anvisa", "01041500000211261725063010LOT42X")
	if code != 1 || !strings.Contains(errOut, "requires AI (21)") {
		t.Errorf("ANVISA fail: exit %d stderr %q", code, errOut)
	}
	if code, _, _ := exec(t, "", "parse", "-validate", "nope", full); code != 2 {
		t.Errorf("unknown regulator: exit %d, want 2", code)
	}
}

func TestParseStream(t *testing.T) {
	stdin := "0104150000021126172506301012345\n\n8812345\n01041500000211261725063010LOT2\n"
	code, out, errOut := exec(t, stdin, "parse")
	if code != 1 {
		t.Errorf("exit %d, want 1 (one bad line)", code)
	}
	if strings.Count(out, "(01) 04150000021126") != 2 {
		t.Errorf("expected two parsed barcodes:\n%s", out)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("expected separator between records")
	}
	if !strings.Contains(errOut, "unknown application identifier") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestParseStreamJSONReportsErrorsInline(t *testing.T) {
	code, out, _ := exec(t, "8812345\n", "parse", "-json")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.Error == "" || got.Raw != "8812345" {
		t.Errorf("got %+v", got)
	}
}

func TestGTIN(t *testing.T) {
	if code, out, _ := exec(t, "", "gtin", "04150000021126"); code != 0 || !strings.Contains(out, "valid") {
		t.Errorf("valid GTIN: exit %d out %q", code, out)
	}
	if code, _, _ := exec(t, "", "gtin", "04150000021127"); code != 1 {
		t.Errorf("bad check digit: exit %d, want 1", code)
	}
	if code, _, _ := exec(t, "", "gtin", "12"); code != 2 {
		t.Errorf("bad length: exit %d, want 2", code)
	}
}

func TestAI(t *testing.T) {
	code, out, _ := exec(t, "", "ai", "3102")
	if code != 0 || !strings.Contains(out, "Net Weight kg") || !strings.Contains(out, "N6") {
		t.Errorf("exit %d out %q", code, out)
	}
	if code, _, _ := exec(t, "", "ai", "88"); code != 1 {
		t.Errorf("unknown AI: exit %d, want 1", code)
	}
}

func TestUsageAndVersion(t *testing.T) {
	if code, _, errOut := exec(t, ""); code != 2 || !strings.Contains(errOut, "Usage") {
		t.Errorf("no args: exit %d stderr %q", code, errOut)
	}
	if code, out, _ := exec(t, "", "help"); code != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help: exit %d", code)
	}
	if code, _, _ := exec(t, "", "bogus"); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
	if code, out, _ := exec(t, "", "version"); code != 0 || !strings.HasPrefix(out, "gs1 ") {
		t.Errorf("version: exit %d out %q", code, out)
	}
}

func TestDataMatrixSVG(t *testing.T) {
	code, out, errOut := exec(t, "", "datamatrix", "(01)04150000021126(17)250630(10)ABC123")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.HasPrefix(out, "<svg ") || !strings.Contains(out, "</svg>") {
		t.Errorf("not an SVG document:\n%s", out)
	}
}

func TestDataMatrixPNG(t *testing.T) {
	path := t.TempDir() + "/dm.png"
	code, _, errOut := exec(t, "", "datamatrix", "-o", path, "-scale", "4", "0104150000021126")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("\x89PNG")) {
		t.Errorf("not a PNG file")
	}
}

func TestDataMatrixErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"no barcode", []string{"datamatrix"}, 2},
		{"bad flag", []string{"datamatrix", "-nope", "01"}, 2},
		{"invalid element string", []string{"datamatrix", "(01)123"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := exec(t, "", tt.args...); code != tt.code {
				t.Errorf("exit %d, want %d", code, tt.code)
			}
		})
	}
}
