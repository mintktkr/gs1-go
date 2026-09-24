package symbol

// dmGFExp and dmGFLog are the antilog and log tables of GF(256) for the
// ECC 200 prime polynomial x^8+x^5+x^3+x^2+1 (0x12D), with alpha = 2
// (ISO/IEC 16022 5.7). dmGFExp is doubled in length so exponents of the
// product of two logs never need a reduction.
var dmGFExp, dmGFLog = dmGaloisTables()

// dmMaxECC is the largest number of ECC codewords per block in dmSizes. It
// bounds the degree index into dmGenLog.
const dmMaxECC = 68

// dmGenLog holds the generator polynomial of degree n in log form for every
// ECC codelength dmSizes uses: dmGenLog[n][i] is log(g_i), the coefficient of
// x^(n-i) for i in 1..n. The leading coefficient g_0 is 1 and is not stored.
// Degrees no size uses stay nil. The polynomials have no zero coefficients,
// so the log form loses nothing; TestDMGenLog checks both properties.
//
// dmGenerators fills it at init and nothing writes to it afterwards, so
// concurrent reads are safe.
var dmGenLog = dmGenerators()

// dmGenerators builds the generator polynomials for the degrees dmSizes
// uses, g(x) = (x-a^1)(x-a^2)...(x-a^n) for n ECC codewords (ISO/IEC 16022
// 5.7).
func dmGenerators() [dmMaxECC + 1][]byte {
	var gen [dmMaxECC + 1][]byte
	for _, s := range dmSizes {
		n := s.ECCCW / s.Blocks
		if gen[n] == nil {
			gen[n] = dmGeneratorLog(n)
		}
	}
	return gen
}

// dmGaloisTables builds the log and antilog tables of GF(256) by repeated
// multiplication by alpha.
func dmGaloisTables() (exp [512]byte, log [256]byte) {
	x := 1
	for i := 0; i < 255; i++ {
		exp[i] = byte(x)
		log[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x12D
		}
	}
	for i := 255; i < len(exp); i++ {
		exp[i] = exp[i-255]
	}
	return exp, log
}

// dmMul multiplies two elements of GF(256).
func dmMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return dmGFExp[int(dmGFLog[a])+int(dmGFLog[b])]
}

// dmGenerator returns the generator polynomial g(x) = (x-a^1)(x-a^2)...(x-a^n)
// for n ECC codewords (ISO/IEC 16022 5.7), highest degree coefficient first.
// The leading coefficient is always 1.
func dmGenerator(n int) []byte {
	g := []byte{1}
	for i := 1; i <= n; i++ {
		root := dmGFExp[i]
		next := make([]byte, len(g)+1)
		for j, c := range g {
			next[j] ^= c
			next[j+1] ^= dmMul(c, root)
		}
		g = next
	}
	return g
}

// dmGeneratorLog returns dmGenerator(n) in log form, without the leading 1.
func dmGeneratorLog(n int) []byte {
	g := dmGenerator(n)
	logs := make([]byte, n)
	for i := range logs {
		logs[i] = dmGFLog[g[i+1]]
	}
	return logs
}

// dmBlockECC writes the ECC codewords of one Reed-Solomon block into rem,
// which must have len(glog) bytes: the remainder of block(x) * x^n divided by
// the generator polynomial, highest degree first. glog is the generator in
// log form, so each term costs one antilog lookup instead of a full multiply.
func dmBlockECC(block, glog, rem []byte) {
	n := len(glog)
	clear(rem)
	for _, c := range block {
		factor := c ^ rem[0]
		copy(rem, rem[1:])
		rem[n-1] = 0
		if factor == 0 {
			continue
		}
		logF := int(dmGFLog[factor])
		for j, logG := range glog {
			rem[j] ^= dmGFExp[logF+int(logG)]
		}
	}
}

// dmECC computes Reed-Solomon error correction for data (already padded to
// s.DataCW) and returns data followed by the interleaved ECC codewords.
func dmECC(data []byte, s dmSize) []byte {
	out := make([]byte, s.DataCW+s.ECCCW)
	copy(out, data)
	blocks := s.Blocks
	n := s.ECCCW / blocks
	glog := dmGenLog[n]
	rem := make([]byte, n)
	var block []byte
	for b := 0; b < blocks; b++ {
		block = block[:0]
		for i := b; i < s.DataCW; i += blocks {
			block = append(block, data[i])
		}
		dmBlockECC(block, glog, rem)
		for j, ecc := range rem {
			out[dmECCPos(s, b, j)] = ecc
		}
	}
	return out
}

// dmECCPos returns the stream position of ECC codeword j of block b.
//
// When the data does not split evenly over the blocks (only 144x144: eight
// blocks of 156 and two of 155), the ECC of the shorter blocks comes first.
// A literal reading of ISO/IEC 16022 gives DataCW + j*Blocks + b, but the
// deployed readers and encoders (zxing, libdmtx, BWIPP, zint) all use this
// rotated layout; see https://github.com/zxing-cpp/zxing-cpp/issues/259.
func dmECCPos(s dmSize, b, j int) int {
	long := s.DataCW % s.Blocks
	return s.DataCW + j*s.Blocks + (b+s.Blocks-long)%s.Blocks
}
