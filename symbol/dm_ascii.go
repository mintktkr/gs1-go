package symbol

import (
	"fmt"
)

// ECC 200 ASCII encodation codewords (ISO/IEC 16022 5.2.3).
const (
	fnc1       = 232 // FNC1: leading GS1 flag and element separator
	upperShift = 235 // shifts the next codeword to bytes 128..255
	padCW      = 129 // first pad codeword, before the 253-state randomizer
)

// dmEncodeASCII converts data to ASCII-encodation codewords. When gs1 is
// true, every ASCII 29 maps to FNC1, including the leading one that
// gs1.Encode emits and that marks the symbol as GS1 DataMatrix.
func dmEncodeASCII(data string, gs1 bool) []byte {
	cw := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case gs1 && c == 0x1D:
			cw = append(cw, fnc1)
		case isDigit(c) && i+1 < len(data) && isDigit(data[i+1]):
			cw = append(cw, 130+10*(c-'0')+(data[i+1]-'0'))
			i++
		case c > 0x7F:
			cw = append(cw, upperShift, c-127)
		default:
			cw = append(cw, c+1)
		}
	}
	return cw
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// dmPad pads cw to capacity codewords with the ECC 200 pad sequence: the
// first pad is 129, each later pad at 1-based stream position p is
// 129 + ((149*p) mod 253) + 1, reduced by 254 when it exceeds 254
// (ISO/IEC 16022 5.2.3, 253-state randomizing).
func dmPad(cw []byte, capacity int) []byte {
	out := make([]byte, capacity)
	n := copy(out, cw)
	if n >= capacity {
		return out
	}
	out[n] = padCW
	for p := n + 2; p <= capacity; p++ {
		v := padCW + (149*p)%253 + 1
		if v > 254 {
			v -= 254
		}
		out[p-1] = byte(v & 0xFF) // v is in 1..254
	}
	return out
}

// dmSelectSize returns the smallest square (or rectangular) size whose data
// capacity is at least n codewords.
func dmSelectSize(n int, rectangular bool) (dmSize, error) {
	for _, s := range dmSizes {
		if (s.Rows != s.Cols) != rectangular || s.DataCW < n {
			continue
		}
		return s, nil
	}
	return dmSize{}, fmt.Errorf("%w: %d codewords", ErrDataTooLong, n)
}
