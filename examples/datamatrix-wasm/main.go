//go:build js && wasm

// Command datamatrix-wasm is a browser demo of the symbol package: it exposes
// one function, gs1dm, that encodes its input as a Data Matrix symbol and
// reports how long encoding took. Build it with build.sh.
package main

import (
	"syscall/js"
	"time"

	"github.com/galenzo17/gs1-go"
	"github.com/galenzo17/gs1-go/symbol"
)

// enc is the demo's reusable Encoder, used when the page asks for it.
var enc symbol.Encoder

// minBench is how long each input is re-encoded to measure it. Browsers
// coarsen timers to 5-100 µs and a small symbol encodes in a few µs, so
// one call cannot be timed on its own.
const minBench = 3 * time.Millisecond

// maxBench caps the re-encodes in case the clock does not advance, as in
// headless browsers with virtual time.
const maxBench = 100000

// gs1dm(input, {gs1, rect, cache, bench, stats}) returns {svg, rows, cols,
// elements, error}, plus {perOpNs, iters} when bench is set and {codewords,
// digitPairs, capacity} when stats is set.
func gs1dm(_ js.Value, args []js.Value) any {
	input := args[0].String()
	opts := args[1]
	isGS1 := opts.Get("gs1").Truthy()
	dmOpts := symbol.DataMatrixOptions{Rectangular: opts.Get("rect").Truthy()}
	cache := opts.Get("cache").Truthy()

	encode := func() (*symbol.Matrix, error) {
		switch {
		case isGS1 && cache:
			return enc.GS1DataMatrix(input, dmOpts)
		case isGS1:
			return symbol.GS1DataMatrix(input, dmOpts)
		case cache:
			return enc.DataMatrix(input, dmOpts)
		default:
			return symbol.DataMatrix(input, dmOpts)
		}
	}

	m, err := encode()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	res := map[string]any{
		"svg":      m.SVG(8, 1),
		"rows":     m.Rows,
		"cols":     m.Cols,
		"elements": elements(input, isGS1),
	}
	if opts.Get("stats").Truthy() {
		n, pairs := codewords(input, isGS1)
		res["codewords"] = n
		res["digitPairs"] = pairs
		res["capacity"] = capacity[m.Rows*1000+m.Cols]
	}
	if !opts.Get("bench").Truthy() {
		return res
	}
	iters := 0
	start := time.Now()
	for (time.Since(start) < minBench || iters < 10) && iters < maxBench {
		_, _ = encode()
		iters++
	}
	res["perOpNs"] = (time.Since(start) / time.Duration(iters)).Nanoseconds()
	res["iters"] = iters
	return res
}

// elements lists the parsed AIs for display.
func elements(input string, isGS1 bool) []any {
	if !isGS1 {
		return []any{}
	}
	b, err := gs1.Parse(input)
	if err != nil {
		return []any{}
	}
	out := make([]any, 0, len(b.Elements))
	for _, e := range b.Elements {
		name := ""
		if ai, ok := gs1.LookupAI(e.AI); ok {
			name = ai.Name
		}
		out = append(out, map[string]any{"ai": e.AI, "name": name, "value": e.Value})
	}
	return out
}

func main() {
	js.Global().Set("gs1dm", js.FuncOf(gs1dm))
	select {}
}
