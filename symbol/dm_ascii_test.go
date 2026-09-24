package symbol

import (
	"bytes"
	"errors"
	"testing"
)

func TestDMASCII(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		gs1     bool
		want    []byte
		wantErr bool
	}{
		{name: "six digits", data: "123456", want: []byte{142, 164, 186}},
		{name: "letter", data: "A", want: []byte{66}},
		{name: "odd digit count", data: "123", want: []byte{142, 52}},
		{name: "letter then digit", data: "a1", want: []byte{98, 50}},
		{name: "upper shift", data: "\xe9", want: []byte{235, 106}},
		{name: "upper shift between ascii", data: "A\xe9A", want: []byte{66, 235, 106, 66}},
		{
			// zint --square -d 123456789: Data (5): 142 164 186 208 58
			name: "nine digits",
			data: "123456789",
			want: []byte{142, 164, 186, 208, 58},
		},
		{
			// zint --gs1 -d [01]04150000021126: Data (9): 232 ..., 156
			name: "gs1 fixed length element string",
			data: "0104150000021126",
			gs1:  true,
			want: []byte{232, 131, 134, 145, 130, 130, 132, 141, 156},
		},
		{
			name: "gs1 variable length separator",
			data: "10AB\x1d2112",
			gs1:  true,
			want: []byte{232, 140, 66, 67, 232, 151, 142},
		},
		{
			// Equivalent to zint --gs1 -d [01]01234567890128[21]XYZ123[10]A1B2,
			// whose Data (21) codewords are exactly these. The 0x1D is the
			// separator the element string puts after the variable length AI 21.
			name: "gs1 element string with separator",
			data: "010123456789012821XYZ123\x1d10A1B2",
			gs1:  true,
			want: []byte{232, 131, 131, 153, 175, 197, 219, 131, 158, 151,
				89, 90, 91, 142, 52, 232, 140, 66, 50, 67, 51},
		},
		{
			// The 0x1D breaks the pairing: 12 -> 142, 3 -> 52, FNC1, 45 -> 175.
			name: "gs1 separator splits digits",
			data: "123\x1d45",
			gs1:  true,
			want: []byte{232, 142, 52, 232, 175},
		},
		{name: "gs1 high byte", data: "\xe9", gs1: true, wantErr: true},
		{name: "gs1 high byte after ascii", data: "\xffA", gs1: true, wantErr: true},
		{name: "empty", data: "", wantErr: true},
		{name: "empty gs1", data: "", gs1: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dmEncodeASCII(tt.data, tt.gs1)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("dmEncodeASCII(%q, %v) = %v, want error", tt.data, tt.gs1, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("dmEncodeASCII(%q, %v) error: %v", tt.data, tt.gs1, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("dmEncodeASCII(%q, %v) = %v, want %v", tt.data, tt.gs1, got, tt.want)
			}
		})
	}
}

func TestDMPad(t *testing.T) {
	tests := []struct {
		name     string
		cw       []byte
		capacity int
		want     []byte
	}{
		{name: "one codeword", cw: []byte{66}, capacity: 3, want: []byte{66, 129, 70}},
		{name: "already full", cw: []byte{1, 2, 3}, capacity: 3, want: []byte{1, 2, 3}},
		{name: "empty data", cw: nil, capacity: 2, want: []byte{129, 175}},
		{
			// First pad is 129 wherever it starts; 6 data codewords leave the
			// first pad at stream position 7. zint --square -d ABCdef reports
			// Pads (2): 129 56.
			name:     "first pad not at position two",
			cw:       []byte{66, 67, 68, 101, 102, 103},
			capacity: 8,
			want:     []byte{66, 67, 68, 101, 102, 103, 129, 56},
		},
		{
			// First pad is always 129; later pads at 1-based position p are
			// 129 + ((149*p) mod 253) + 1, minus 254 when that exceeds 254:
			//   p=3: 129 + ( 447 mod 253) + 1 = 129 + 194 + 1 = 324 ->   70
			//   p=4: 129 + ( 596 mod 253) + 1 = 129 +  90 + 1 = 220
			//   p=5: 129 + ( 745 mod 253) + 1 = 129 + 239 + 1 = 369 ->  115
			//   p=6: 129 + ( 894 mod 253) + 1 = 129 + 135 + 1 = 265 ->   11
			//   p=7: 129 + (1043 mod 253) + 1 = 129 +  31 + 1 = 161
			//   p=8: 129 + (1192 mod 253) + 1 = 129 + 180 + 1 = 310 ->   56
			// zint --vers=3 -d A reports the same Pads (7): 129 70 220 115 11 161 56.
			name:     "six pad codewords",
			cw:       []byte{66},
			capacity: 8,
			want:     []byte{66, 129, 70, 220, 115, 11, 161, 56},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dmPad(tt.cw, tt.capacity)
			if len(got) != tt.capacity {
				t.Fatalf("dmPad(%v, %d) length = %d, want %d", tt.cw, tt.capacity, len(got), tt.capacity)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("dmPad(%v, %d) = %v, want %v", tt.cw, tt.capacity, got, tt.want)
			}
		})
	}

	t.Run("does not alias input", func(t *testing.T) {
		cw := []byte{66}
		got := dmPad(cw, 3)
		got[0] = 99
		if cw[0] != 66 {
			t.Errorf("dmPad wrote through to its input: cw = %v", cw)
		}
	})
}

func TestDMSelectSize(t *testing.T) {
	tests := []struct {
		name        string
		n           int
		rectangular bool
		wantRows    int
		wantCols    int
		wantErr     bool
	}{
		{name: "one codeword", n: 1, wantRows: 10, wantCols: 10},
		{name: "smallest square", n: 3, wantRows: 10, wantCols: 10},
		{name: "next square", n: 4, wantRows: 12, wantCols: 12},
		{name: "largest square", n: 1558, wantRows: 144, wantCols: 144},
		{name: "past largest square", n: 1559, wantErr: true},
		{name: "smallest rectangular", n: 5, rectangular: true, wantRows: 8, wantCols: 18},
		{name: "next rectangular", n: 6, rectangular: true, wantRows: 8, wantCols: 32},
		{name: "largest rectangular", n: 49, rectangular: true, wantRows: 16, wantCols: 48},
		{name: "past largest rectangular", n: 50, rectangular: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dmSelectSize(tt.n, tt.rectangular)
			if tt.wantErr {
				if !errors.Is(err, ErrDataTooLong) {
					t.Fatalf("dmSelectSize(%d, %v) error = %v, want ErrDataTooLong", tt.n, tt.rectangular, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("dmSelectSize(%d, %v) error: %v", tt.n, tt.rectangular, err)
			}
			if got.Rows != tt.wantRows || got.Cols != tt.wantCols {
				t.Errorf("dmSelectSize(%d, %v) = %dx%d, want %dx%d",
					tt.n, tt.rectangular, got.Rows, got.Cols, tt.wantRows, tt.wantCols)
			}
		})
	}
}
