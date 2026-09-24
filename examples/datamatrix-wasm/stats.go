//go:build js && wasm

package main

import "github.com/galenzo17/gs1-go"

// The symbol package keeps its codeword stream private, so the demo counts
// it the same way for its statistics: ASCII encodation, digit pairs packed
// into one codeword, FNC1 first and after variable-length fields. The page
// checks each count against the capacity of the size the encoder chose.

// capacity lists the data codewords of each ECC 200 size (ISO/IEC 16022
// table 7), keyed by rows*1000+cols.
var capacity = map[int]int{
	10010: 3, 12012: 5, 14014: 8, 16016: 12, 18018: 18, 20020: 22, 22022: 30,
	24024: 36, 26026: 44, 32032: 62, 36036: 86, 40040: 114, 44044: 144,
	48048: 174, 52052: 204, 64064: 280, 72072: 368, 80080: 456, 88088: 576,
	96096: 696, 104104: 816, 120120: 1050, 132132: 1304, 144144: 1558,
	8018: 5, 8032: 10, 12026: 16, 12036: 22, 16036: 32, 16048: 49,
}

// codewords returns the ASCII-encodation codeword count of the data a
// symbol carries, and how many of them are packed digit pairs.
func codewords(input string, isGS1 bool) (n, pairs int) {
	data := input
	if isGS1 {
		b, err := gs1.Parse(input)
		if err != nil {
			return 0, 0
		}
		data = ""
		for i, e := range b.Elements {
			data += e.AI + e.Value
			if i < len(b.Elements)-1 && !predefined(e.AI) {
				data += "\x1D"
			}
		}
		n = 1 // leading FNC1
	}
	for i := 0; i < len(data); i++ {
		switch c := data[i]; {
		case isDigit(c) && i+1 < len(data) && isDigit(data[i+1]):
			pairs++
			i++
		case c > 127:
			n++ // upper shift
		}
		n++
	}
	return n, pairs
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// predefined reports whether an AI never needs a trailing FNC1 (GS1 General
// Specifications table 7.8.5-2).
func predefined(ai string) bool {
	switch ai[:2] {
	case "00", "01", "02", "03", "04", "11", "12", "13", "14", "15", "16",
		"17", "18", "19", "20", "31", "32", "33", "34", "35", "36", "41":
		return true
	}
	return false
}
