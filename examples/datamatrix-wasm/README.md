# Live GS1 DataMatrix in the browser

A one-page demo of the `symbol` package compiled to WebAssembly. Type an
element string such as `(01)04150000021126(17)250630(10)ABC123`; every
keystroke re-encodes the symbol and shows:

- the encode time, averaged over ~3 ms of re-encodes, because browsers
  coarsen timers too much to time a single encode of a few microseconds;
- the time of one encode plus drawing the SVG;
- the symbol size and the parsed Application Identifiers.

**Demo mode** (the ▶ button) invents realistic labels (GTINs with valid
check digits, real dates, lots, serials, SSCCs, some rectangular, some that
fill 144x144) and types them in, re-encoding on every character. Finished
symbols collect in a gallery; click one to load it. *calm* and *fast* type
visibly; *turbo* encodes whole labels as fast as the frame allows and shows
the throughput. Typing yourself stops the demo.

While the demo runs, **live statistics** summarize what it generated: which
of the 30 sizes the encoder picked, how full each symbol's data area is
(always at least ~60%, because the encoder picks the smallest size that
fits and each size holds at most ~1.6x the previous one), how many
codewords digit-pair compaction saved, the AI mix, and throughput over the
last 30 seconds. The demo counts codewords itself (`stats.go`), because the
package keeps its codeword stream private, and checks every count against
the size the encoder chose.

While the input is not a valid element string yet, the last symbol stays
faded and the parser's error explains what is missing. The input is kept in
the URL hash, so a symbol can be shared as a link.

```bash
bash examples/datamatrix-wasm/build.sh
python3 -m http.server -d examples/datamatrix-wasm 8080   # any static server
```

The page loads the module with `fetch` and `WebAssembly.instantiate`, so the
server does not need to send `application/wasm`.
