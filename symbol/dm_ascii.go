package symbol

// dmEncodeASCII converts data to ASCII-encodation codewords. When gs1 is
// true, FNC1 (232) is emitted first and every ASCII 29 maps to FNC1.
func dmEncodeASCII(data string, gs1 bool) ([]byte, error) {
	panic("dmEncodeASCII: not implemented")
}

// dmPad pads cw to capacity codewords with the ECC 200 pad sequence.
func dmPad(cw []byte, capacity int) []byte {
	panic("dmPad: not implemented")
}

// dmSelectSize returns the smallest square (or rectangular) size whose data
// capacity is at least n codewords.
func dmSelectSize(n int, rectangular bool) (dmSize, error) {
	panic("dmSelectSize: not implemented")
}
