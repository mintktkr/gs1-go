package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"strings"
	"testing"

	gs1 "github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
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

func TestParseWeightText(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "3102001250")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "12.50 kg") {
		t.Errorf("output = %q, want formatted weight", out)
	}
}

func TestParseWeightJSON(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "-json", "3405001250")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got.Elements) != 1 || got.Elements[0].Measure == nil {
		t.Fatalf("elements = %+v, want one measured element", got.Elements)
	}
	measure := got.Elements[0].Measure
	if measure.Raw != "001250" || measure.Value != 0.0125 || measure.Unit != "lb" {
		t.Errorf("measure = %+v, want raw 001250, value 0.0125, unit lb", measure)
	}
}

func TestParseJSONWithISODates(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "-json", "-iso", "010415000002112617250200020415000002112613250601")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got.Elements) != 4 {
		t.Fatalf("elements = %d, want 4", len(got.Elements))
	}
	if got.Elements[1].Date != "2025-02-28" {
		t.Errorf("date = %q, want 2025-02-28", got.Elements[1].Date)
	}
	if got.PackagingDate != "2025-06-01" {
		t.Errorf("packagingDate = %q, want 2025-06-01", got.PackagingDate)
	}
}

func TestParseJSONIncludesTypedAccessors(t *testing.T) {
	input := "0104150000021126" + "0204150000021126" + "13250601" + "3742\x1D" + "40212345678901234567" + "4141234567890123"
	code, out, _ := exec(t, "", "parse", "-json", input)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.ContentGTIN != "04150000021126" || got.CountOfTradeItems != "42" || got.GLN != "1234567890123" || got.GSIN != "12345678901234567" || got.PackagingDate != "250601" {
		t.Errorf("typed accessors = %+v", got)
	}
}

func TestParseDueDateWarning(t *testing.T) {
	code, out, errOut := exec(t, "", "parse", "-json", "12250630")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].Code != gs1.WarnDueDateAsExpiry {
		t.Errorf("warnings = %+v", got.Warnings)
	}
	if !strings.Contains(errOut, "AI (12) is a due date") {
		t.Errorf("stderr = %q", errOut)
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

func TestParseStrictAssociations(t *testing.T) {
	code, _, errOut := exec(t, "", "parse", "-strict", "0204150000021126")
	if code != 1 || !strings.Contains(errOut, "requires AI (37)") {
		t.Errorf("strict association exit %d stderr %q", code, errOut)
	}
	if code, _, _ := exec(t, "", "parse", "-strict", "0001234567890123456702041500000211263720"); code != 0 {
		t.Errorf("valid strict association exit %d, want 0", code)
	}
}

func TestParseIndependentValidationFlags(t *testing.T) {
	options := []struct {
		name        string
		flags       []string
		strict      bool
		checkDigits bool
	}{
		{name: "lenient"},
		{name: "strict", flags: []string{"-strict"}, strict: true},
		{name: "check digits", flags: []string{"-check-digits"}, checkDigits: true},
		{name: "both", flags: []string{"-strict", "-check-digits"}, strict: true, checkDigits: true},
	}
	inputs := []struct {
		name           string
		input          string
		badAssociation bool
		badCheckDigit  bool
	}{
		{name: "valid", input: "0104150000021126"},
		{name: "checksum only", input: "0104150000021127", badCheckDigit: true},
		{name: "content GTIN checksum only", input: "0001234567890123456002041500000211273720", badCheckDigit: true},
		{name: "association only", input: "0204150000021126", badAssociation: true},
		{name: "both errors", input: "0204150000021127", badAssociation: true, badCheckDigit: true},
	}
	for _, option := range options {
		for _, input := range inputs {
			t.Run(option.name+"/"+input.name, func(t *testing.T) {
				args := append([]string{"parse"}, option.flags...)
				args = append(args, input.input)
				code, out, errOut := exec(t, "", args...)
				wantError := ""
				if option.strict && input.badAssociation {
					wantError = "invalid AI association"
				} else if option.checkDigits && input.badCheckDigit {
					wantError = "invalid check digit"
				}
				if wantError == "" {
					if code != 0 || out == "" || errOut != "" {
						t.Errorf("exit %d, stdout %q, stderr %q, want parsed output", code, out, errOut)
					}
				} else if code != 1 || out != "" || !strings.Contains(errOut, wantError) {
					t.Errorf("exit %d, stdout %q, stderr %q, want %q", code, out, errOut, wantError)
				}
			})
		}
	}
}

func TestParseCheckDigitStreamJSONContinues(t *testing.T) {
	inputs := []string{"0104150000021126", "0104150000021127", "]E496385074"}
	code, out, errOut := exec(t, strings.Join(inputs, "\n")+"\n", "parse", "-json", "-strict", "-check-digits")
	if code != 1 || errOut != "" {
		t.Errorf("exit %d, stderr %q, want exit 1 and inline JSON errors", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != len(inputs) {
		t.Fatalf("JSON records = %d, want %d: %s", len(lines), len(inputs), out)
	}
	for i, line := range lines {
		var got parseOutput
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("record %d: invalid JSON: %v", i, err)
		}
		if got.Raw != inputs[i] {
			t.Errorf("record %d: Raw = %q, want %q", i, got.Raw, inputs[i])
		}
		if i == 1 {
			if !strings.Contains(got.Error, "invalid check digit") || len(got.Elements) != 0 {
				t.Errorf("error record = %+v, want checksum error without elements", got)
			}
		} else if got.Error != "" || len(got.Elements) != 1 || got.Elements[0].AI != "01" {
			t.Errorf("record %d = %+v, want one successful GTIN", i, got)
		}
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

func checkEPCTagJSON(t *testing.T, out string) {
	t.Helper()
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.Symbology != "EPC" || got.EPC == nil || got.EPC.Scheme != "sscc" || got.EPC.Bits != 96 || got.EPC.Filter == nil || *got.EPC.Filter != 3 ||
		got.EPC.CompanyPrefix != "0614141" || got.EPC.Reference != "1234567890" || got.EPC.URI != "urn:epc:id:sscc:0614141.1234567890" ||
		len(got.Elements) != 1 || got.Elements[0].AI != "00" || got.Elements[0].Value != "106141412345678908" {
		t.Errorf("JSON = %+v epc %+v", got, got.EPC)
	}
}

func checkEPCIdentityJSON(t *testing.T, out string) {
	t.Helper()
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.EPC == nil || got.EPC.Filter != nil || got.EPC.Bits != 0 || got.GLN != "0614141123452" || len(got.Elements) != 1 {
		t.Errorf("pure identity JSON = %+v epc %+v", got, got.EPC)
	}
}

func TestEPC(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		wantOut  []string
		wantErr  string
		check    func(t *testing.T, out string)
	}{
		{name: "text", args: []string{"epc", "3074257BF7194E4000001A85"},
			wantOut: []string{"SGTIN-96 urn:epc:id:sgtin:0614141.812345.6789", "(01) 80614141123458", "GTIN", "(21) 6789", "Serial Number"}},
		{name: "json tag URI", args: []string{"epc", "-json", "urn:epc:tag:sscc-96:3.0614141.1234567890"}, check: checkEPCTagJSON},
		{name: "json pure identity", args: []string{"epc", "-json", "urn:epc:id:sgln:0614141.12345.0"}, check: checkEPCIdentityJSON},
		{name: "unsupported scheme", args: []string{"epc", "350000000000000000000000"}, wantCode: 1, wantErr: "unsupported EPC scheme"},
		{name: "regulator", args: []string{"epc", "-validate", "anvisa", "3074257BF7194E4000001A85"}, wantCode: 1, wantErr: "missing required AI"},
		{name: "bad regulator", args: []string{"epc", "-validate", "nope", "3074257BF7194E4000001A85"}, wantCode: 2, wantErr: "unknown regulator"},
		{name: "two arguments", args: []string{"epc", "a", "b"}, wantCode: 2, wantErr: "at most one"},
		{name: "stream", args: []string{"epc"}, stdin: "3074257BF7194E4000001A85\n\nxyz\n3174257BF4499602D2000000\n", wantCode: 1,
			wantOut: []string{"---\n---", "(00) 106141412345678908"}, wantErr: "odd length"},
		{name: "stream json reports errors inline", args: []string{"epc", "-json"}, stdin: "xyz\n", wantCode: 1, wantOut: []string{`"error":"gs1: invalid data`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := exec(t, tt.stdin, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit %d, want %d (stdout %q stderr %q)", code, tt.wantCode, out, errOut)
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(out, want) {
					t.Errorf("stdout %q lacks %q", out, want)
				}
			}
			if tt.wantErr != "" && !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("stderr %q lacks %q", errOut, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, out)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{name: "raw", args: []string{"encode", "(10)LOT42(01)04150000021126(21)SERIAL"}, wantOut: "\x1D0104150000021126" + "10LOT42\x1D21SERIAL\n"},
		{name: "hri reorders", args: []string{"encode", "-hri", "(10)LOT42(01)04150000021126(21)SERIAL"}, wantOut: "(01)04150000021126(10)LOT42(21)SERIAL\n"},
		{name: "zpl", args: []string{"encode", "-zpl", "(10)LOT42(01)04150000021126(21)SERIAL"}, wantOut: ">80104150000021126" + "10LOT42>821SERIAL\n"},
		{name: "zpl escapes greater-than", args: []string{"encode", "-zpl", "(01)04150000021126(10)A>B"}, wantOut: ">80104150000021126" + "10A><B\n"},
		{name: "raw scan input", args: []string{"encode", "-hri", "]d20104150000021126" + "10LOT42"}, wantOut: "(01)04150000021126(10)LOT42\n"},
		{name: "logistic label", args: []string{"encode", "-hri", "(00)790123451234500116(02)49012345008794(37)12(10)ABC123"}, wantOut: "(00)790123451234500116(02)49012345008794(37)12(10)ABC123\n"},
		{name: "association", args: []string{"encode", "(02)49012345008794(37)12"}, wantCode: 1, wantErr: "AI (37) requires AI (00)"},
		{name: "check digit", args: []string{"encode", "(01)04150000021127"}, wantCode: 1, wantErr: "invalid check digit"},
		{name: "unknown AI", args: []string{"encode", "(999)1"}, wantCode: 1, wantErr: "unknown application identifier"},
		{name: "malformed HRI", args: []string{"encode", "(01"}, wantCode: 1, wantErr: "invalid data"},
		{name: "empty value", args: []string{"encode", "(01)(10)LOT"}, wantCode: 1, wantErr: "has no value"},
		{name: "both flags", args: []string{"encode", "-hri", "-zpl", "(01)04150000021126"}, wantCode: 2, wantErr: "mutually exclusive"},
		{name: "too many arguments", args: []string{"encode", "(01)04150000021126", "(10)X"}, wantCode: 2, wantErr: "at most one"},
		{name: "stdin stream", args: []string{"encode", "-hri"}, stdin: "(10)A(01)04150000021126\n\n(21)S\n(00)790123451234500116\n", wantCode: 1,
			wantOut: "(01)04150000021126(10)A\n(00)790123451234500116\n", wantErr: "AI (21) requires"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := exec(t, tt.stdin, tt.args...)
			if code != tt.wantCode || out != tt.wantOut || !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("exit %d stdout %q stderr %q, want exit %d stdout %q stderr containing %q", code, out, errOut, tt.wantCode, tt.wantOut, tt.wantErr)
			}
		})
	}
}

func TestEncodeRoundTripsThroughParse(t *testing.T) {
	code, raw, _ := exec(t, "", "encode", "(10)LOT42(01)04150000021126(17)250630")
	if code != 0 {
		t.Fatalf("encode exit %d", code)
	}
	code, out, _ := exec(t, "", "parse", "-strict", "-check-digits", "-json", strings.TrimSpace(raw))
	if code != 0 {
		t.Fatalf("parse exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Elements) != 3 || got.Elements[0].AI != "01" || got.Elements[1].AI != "17" || got.Elements[2].Value != "LOT42" {
		t.Errorf("parse(encode()) elements = %+v", got.Elements)
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

func TestKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{name: "SSCC", key: "106141411234567897", wantOut: "valid SSCC or GSRN\n"},
		{name: "GLN", key: "0614141000012", wantOut: "valid GTIN or GLN\n"},
		{name: "GSIN", key: "06141410000000012", wantOut: "valid GSIN\n"},
		{name: "GTIN-14", key: "04150000021126", wantOut: "valid GTIN\n"},
		{name: "bad SSCC check digit", key: "106141411234567891", wantCode: 1, wantErr: "invalid check digit"},
		{name: "bad GSIN check digit", key: "06141410000000013", wantCode: 1, wantErr: "invalid check digit"},
		{name: "unsupported length", key: "123456789", wantCode: 2, wantErr: "matches no GS1 key"},
		{name: "letters", key: "10614141123456789A", wantCode: 2, wantErr: "non-digit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := exec(t, "", "key", tt.key)
			if code != tt.wantCode || out != tt.wantOut || !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("exit %d stdout %q stderr %q, want exit %d stdout %q stderr containing %q", code, out, errOut, tt.wantCode, tt.wantOut, tt.wantErr)
			}
		})
	}
	if code, _, errOut := exec(t, "", "key"); code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("missing argument: exit %d stderr %q, want usage error", code, errOut)
	}
}

func TestParseCheckDigitsCoversKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "SSCC", input: "00106141411234567891", want: "AI (00)"},
		{name: "GLN", input: "4140614141000013", want: "AI (414)"},
		{name: "GSIN", input: "40206141410000000013", want: "AI (402)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, errOut := exec(t, "", "parse", tt.input); code != 0 || errOut != "" {
				t.Fatalf("lenient parse: exit %d stderr %q", code, errOut)
			}
			code, out, errOut := exec(t, "", "parse", "-check-digits", tt.input)
			if code != 1 || out != "" || !strings.Contains(errOut, "invalid check digit") || !strings.Contains(errOut, tt.want) {
				t.Errorf("exit %d stdout %q stderr %q, want check-digit error for %s", code, out, errOut, tt.want)
			}
		})
	}
}

func TestSSCC(t *testing.T) {
	code, out, _ := exec(t, "", "sscc", "-ext", "3", "-gcp", "7801234", "-serial", "000000009")
	if code != 0 || strings.TrimSpace(out) != "378012340000000095" {
		t.Errorf("SSCC: exit %d out %q", code, out)
	}
	if code, _, _ := exec(t, "", "sscc", "-ext", "3", "-gcp", "7", "-serial", "000000009"); code != 1 {
		t.Errorf("invalid SSCC: exit %d, want 1", code)
	}
}

func TestGTINBuilders(t *testing.T) {
	code, out, _ := exec(t, "", "gtin13", "-gcp", "7801234", "-item", "00005")
	if code != 0 || strings.TrimSpace(out) != "7801234000056" {
		t.Errorf("gtin13: exit %d out %q", code, out)
	}
	if code, _, errOut := exec(t, "", "gtin13", "-gcp", "7801234", "-item", "5"); code != 1 || !strings.Contains(errOut, "12 digits") {
		t.Errorf("gtin13 short: exit %d stderr %q, want 1", code, errOut)
	}
	if code, _, errOut := exec(t, "", "gtin13", "-gcp", "7801234"); code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("gtin13 missing item: exit %d stderr %q, want 2", code, errOut)
	}

	code, out, _ = exec(t, "", "gtin14", "-indicator", "1", "7801234000056")
	if code != 0 || strings.TrimSpace(out) != "17801234000053" {
		t.Errorf("gtin14: exit %d out %q", code, out)
	}
	if code, _, errOut := exec(t, "", "gtin14", "-indicator", "1", "7801234000057"); code != 1 || !strings.Contains(errOut, "invalid check digit") {
		t.Errorf("gtin14 bad GTIN-13: exit %d stderr %q, want 1", code, errOut)
	}
	if code, _, errOut := exec(t, "", "gtin14", "7801234000056"); code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("gtin14 missing indicator: exit %d stderr %q, want 2", code, errOut)
	}
	if code, _, _ := exec(t, "", "gtin14", "-indicator", "12", "7801234000056"); code != 2 {
		t.Errorf("gtin14 two-digit indicator: exit %d, want 2", code)
	}
}

func TestParseBareGTIN8Flag(t *testing.T) {
	code, out, _ := exec(t, "", "parse", "96385074")
	if code != 0 || !strings.Contains(out, "(96)") {
		t.Fatalf("default: exit %d out %q, want AI interpretation", code, out)
	}
	code, out, _ = exec(t, "", "parse", "-gtin8", "-json", "96385074")
	if code != 0 {
		t.Fatalf("-gtin8: exit %d", code)
	}
	var got parseOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Elements) != 1 || got.Elements[0].AI != "01" || got.Elements[0].Value != "00000096385074" {
		t.Errorf("-gtin8 elements = %+v, want GTIN-8 normalized to AI 01", got.Elements)
	}
	if code, _, errOut := exec(t, "", "parse", "-gtin8", "-check-digits", "96385075"); code != 1 || !strings.Contains(errOut, "invalid check digit") {
		t.Errorf("-gtin8 -check-digits: exit %d stderr %q", code, errOut)
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
	version, commit, date = "v9.9.9", "abc1234", "2026-01-02"
	defer func() { version, commit, date = "", "", "" }()
	if code, out, _ := exec(t, "", "version"); code != 0 || strings.TrimSpace(out) != "gs1 v9.9.9 (abc1234, 2026-01-02)" {
		t.Errorf("release version: exit %d out %q", code, out)
	}
}

// dataMatrixSVG is the expected document for input rendered with the given
// parameters, built from the library rather than the CLI.
func dataMatrixSVG(t *testing.T, input string, opts symbol.DataMatrixOptions, scale, quiet int) string {
	t.Helper()
	elements, err := gs1.ParseHRI(input)
	if err != nil {
		t.Fatalf("ParseHRI(%q) error = %v", input, err)
	}
	m, err := symbol.GS1DataMatrix(elements, opts)
	if err != nil {
		t.Fatalf("GS1DataMatrix() error = %v", err)
	}
	return m.SVG(scale, quiet)
}

func TestDataMatrixSVG(t *testing.T) {
	const input = "(01)04150000021126(17)250630(10)ABC123"
	code, out, errOut := exec(t, "", "datamatrix", "-scale", "3", "-quiet", "2", input)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if want := dataMatrixSVG(t, input, symbol.DataMatrixOptions{}, 3, 2); out != want {
		t.Errorf("SVG() =\n%s\nwant\n%s", out, want)
	}
}

func TestDataMatrixRectangular(t *testing.T) {
	const input = "(01)04150000021126(10)ABC123"
	code, out, errOut := exec(t, "", "datamatrix", "-rect", input)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	want := dataMatrixSVG(t, input, symbol.DataMatrixOptions{Rectangular: true}, 10, 1)
	if out != want {
		t.Errorf("SVG() =\n%s\nwant\n%s", out, want)
	}
}

// TestDataMatrixRawInput checks that the command accepts what gs1 encode
// accepts: bracket notation and a raw scan string.
func TestDataMatrixRawInput(t *testing.T) {
	b := dataMatrixSVG(t, "(01)04150000021126(10)LOT42", symbol.DataMatrixOptions{}, 10, 1)
	for _, input := range []string{"(10)LOT42(01)04150000021126", "]d2010415000002112610LOT42"} {
		code, out, errOut := exec(t, "", "datamatrix", input)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", input, code, errOut)
		}
		if out != b {
			t.Errorf("%s: SVG differs from bracket notation", input)
		}
	}
}

func TestDataMatrixWritesSVGFile(t *testing.T) {
	path := t.TempDir() + "/dm.svg"
	const input = "(01)04150000021126"
	code, _, errOut := exec(t, "", "datamatrix", "-o", path, input)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := dataMatrixSVG(t, input, symbol.DataMatrixOptions{}, 10, 1); string(got) != want {
		t.Errorf("file SVG =\n%s\nwant\n%s", got, want)
	}
}

func TestDataMatrixPNG(t *testing.T) {
	path := t.TempDir() + "/dm.PNG"
	code, _, errOut := exec(t, "", "datamatrix", "-o", path, "-scale", "4", "-quiet", "2", "0104150000021126")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	// The 16-character element string fits a 16x16 module symbol.
	if want := (16 + 2*2) * 4; img.Bounds().Dx() != want || img.Bounds().Dy() != want {
		t.Errorf("image size = %v, want %d x %d", img.Bounds(), want, want)
	}
}

func TestDataMatrixErrors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		args    []string
		code    int
		wantErr string
	}{
		{name: "no barcode", args: []string{"datamatrix"}, code: 2, wantErr: "usage: gs1 datamatrix"},
		{name: "too many arguments", args: []string{"datamatrix", "-scale", "2", "0104150000021126", "10LOT42"}, code: 2, wantErr: "usage: gs1 datamatrix"},
		{name: "bad flag", args: []string{"datamatrix", "-nope", "0104150000021126"}, code: 2, wantErr: "flag provided but not defined"},
		{name: "malformed bracket notation", args: []string{"datamatrix", "(01"}, code: 1, wantErr: "gs1: invalid data"},
		{name: "unknown AI", args: []string{"datamatrix", "(999)1"}, code: 1, wantErr: "gs1: unknown application identifier"},
		// Parse accepts a wrong check digit; gs1.Encode refuses to print it.
		{name: "wrong check digit", args: []string{"datamatrix", "(01)04150000021127"}, code: 1, wantErr: "invalid check digit"},
		{name: "too long for rectangular", args: []string{"datamatrix", "-rect", "(01)04150000021126(91)" + strings.Repeat("A", 90)}, code: 1, wantErr: "gs1: symbol: data too long"},
		{name: "unwritable path", args: []string{"datamatrix", "-o", dir, "0104150000021126"}, code: 1, wantErr: "gs1: open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := exec(t, "", tt.args...)
			if code != tt.code {
				t.Fatalf("exit %d, want %d (stderr %q)", code, tt.code, errOut)
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
			if !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("stderr = %q, want containing %q", errOut, tt.wantErr)
			}
		})
	}
}
