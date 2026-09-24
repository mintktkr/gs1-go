# Performance notes: `symbol` Data Matrix

How the encoder and renderers were profiled, what changed, what is
configurable and why, and what was tried and dropped. All numbers are from
one machine (AMD Ryzen 9 5900HS, Go 1.26, linux/amd64); compare ratios,
not absolute times.

## Summary

- Encoding is about 30% faster for small and large symbols and allocates
  62-74% less. SVG is twice as fast; every renderer now allocates twice
  per call.
- Rendering, not encoding, is the expensive part: a 144x144 symbol at scale
  10 encodes in ~0.16 ms and takes ~14 ms to write as PNG.
- Two trade-offs are opt-in instead of defaults: `symbol.Encoder` (layout
  cache, memory for speed) and `Matrix.Paletted` (1-bit PNG, file size for
  CPU on huge images).
- Output is unchanged everywhere: same modules (zint goldens), same SVG
  bytes, same PNG pixels.

## Method

- Benchmarks live in `symbol/bench_test.go`. Two inputs: a typical pharma
  unit pack, `(01)(17)(10)(21)`, which encodes as 22x22, and a large one
  (GTIN plus sixteen 90-character internal AIs) that fills 144x144 and
  exercises multi-block Reed-Solomon.
- Profiles: `go test -bench X -cpuprofile cpu.out -memprofile mem.out`,
  then `go tool pprof -top` (CPU and `-sample_index=alloc_space`).
- Comparisons: the same benchmark file compiled into each variant, run in
  interleaved rounds with alternating order, compared with `benchstat`.
  Rows that were close were rerun pinned to one core (`taskset -c 11`).
- Allocation counts are exact. Time differences of a few percent are
  within noise on a laptop.

```bash
make bench                     # parser and symbol benchmarks
go test -run '^$' -bench . -benchmem -count 10 ./symbol/ > new.txt
benchstat old.txt new.txt
```

## Where the time went (before)

| Stage, 144x144 | Time | Allocs | Hot spot (pprof) |
|---|---|---|---|
| Reed-Solomon | 113 µs | 77 | generator polynomial rebuilt on every call (80% of allocated objects); every multiply did two log lookups and a zero test |
| Placement | 103 µs | 4 | an int8 mapping grid filled by the Annex F walk, then copied region by region into the symbol |
| Full encode | 274 µs | 96 | the two above, plus parsing and ASCII encodation |
| SVG | 664 µs | 9,821 | `strconv.Itoa` allocates for every number >= 100, then `strings.Builder` copies and regrows |
| PNG (1460x1460 px) | 16 ms | 32 | deflate and PNG filtering over 2.1 MP; `flate.NewWriter` allocates ~600 KiB of tables per call |

Rendering dominates: writing a large symbol as PNG takes about 60 times as
long as encoding it. For a 22x22 symbol at scale 10, the PNG encoder's
per-call allocations (894 KiB for a 57 KiB image) were most of the cost.

## What changed

### Always on

| Change | Effect | Why no option |
|---|---|---|
| Reed-Solomon generators built once at init for the 16 ECC lengths in use, stored in log form; one antilog lookup per term (the zint and zxing-cpp approach) | ECC -14%, allocations 77 -> 5 | immutable tables, no trade-off |
| Placement writes straight into the symbol through row and column offset tables; a bitset of corner-pattern modules replaces the full "placed" grid, since only those can be revisited by a sweep | placement -37% | no state, strictly faster |
| SVG built in one pre-sized `[]byte` with `strconv.AppendInt` | SVG -47..50%, allocations 9,821 -> 2 | output byte for byte identical |
| PNG encoder buffers pooled (`png.Encoder.BufferPool` over `sync.Pool`) | small PNG -20% and 93% less memory; large PNG about the same | output identical; the pool is safe for concurrent use |

### Opt-in

| Option | Gain | Cost |
|---|---|---|
| `symbol.Encoder`: caches each size's module layout (one `uint16` per module) | placement another ~2.5x faster (64 -> 24 µs for 144x144); full encode ~30% faster than the package functions | holds 2 bytes per module per size used (41 KiB for 144x144, 220 KiB for all 30); first encode of a size pays the walk once |
| `Matrix.Paletted` + `png.Encode`: 1-bit PNG | files about half the size (22x22: 706 -> 354 B; 144x144: 13.6 -> 7.0 KB) | about 14% slower on very large images: `image/png` packs 1-bit rows through a per-pixel interface call |

The cache is an explicit value rather than package-level state, so the
package functions stay stateless like the rest of the module, the memory is
owned and freed by the caller, and tests cannot leak state into each other.
This mirrors `png.Encoder`, where the caller opts into reuse.

## Tried and dropped

| Idea | Result |
|---|---|
| Hand-rolled two-digit formatter for SVG numbers | 3-5% faster than `strconv.AppendInt` for ~25 more lines |
| Merge SVG runs vertically into taller rectangles | ~10% smaller SVG; needs a second pass with per-row state |
| Branchless placement from the layout with sentinel codewords | slower: the extra copy costs more than the branch |
| Full 256-entry multiplication tables per generator coefficient | memory for no measurable gain over log form |
| `png.BestSpeed` / `png.BestCompression` | twice the file size / 7% smaller for 1.6x the time; default kept |
| Our own 1-bit IDAT writer | would beat the `image/png` packing loop; more code than the gain is worth here |
| Package-level placement cache (map + `RWMutex`) | as fast as `Encoder`, but global state and memory nobody can release |

## Results (this branch vs. the first working version)

`feat/datamatrix` (first working version) against this branch: same
benchmark file, eight interleaved rounds, pinned to one core with
`taskset -c 11`. A backup was running on the machine, hence the wide
intervals; allocation counts are exact.

| Benchmark | Before | After | Change | Allocs before -> after |
|---|---|---|---|---|
| Full encode, 22x22 | 5.22 µs | 3.68 µs | -30% | 34 -> 13 |
| Full encode, 144x144 | 232 µs | 163 µs | -30% | 96 -> 25 |
| Reed-Solomon, 144x144 | 113 µs | 97 µs | -14% | 77 -> 5 |
| Placement, 144x144 | 102 µs | 64 µs | -37% | 4 -> 5 |
| PNG, 22x22 | 505 µs | 403 µs | -20% | 31 -> 2 |
| PNG, 144x144 | 15.7 ms | 13.9 ms | ~ (p=0.23) | 32 -> 2 |
| SVG, 22x22 | 14.0 µs | 7.0 µs | -50% | 161 -> 2 |
| SVG, 144x144 | 706 µs | 374 µs | -47% | 9,821 -> 2 |

Memory per call: PNG 22x22 894 -> 64 KiB, SVG 144x144 538 -> 240 KiB,
full encode 144x144 56 -> 40 KiB.

Opt-in paths, same machine, measured after the table above (not
interleaved, so compare loosely):

| Benchmark | Time | Allocs | Compare with |
|---|---|---|---|
| `Encoder`, full encode 22x22 | 2.16 µs | 10 | 3.68 µs package function |
| `Encoder`, full encode 144x144 | 114 µs | 22 | 163 µs package function |
| `Encoder`, placement 144x144 | 24 µs | 1 | 64 µs direct |
| `png.Encode(m.Paletted)`, 22x22 | 384 µs | 34 | 403 µs `PNG`; half the file size |

`png.Encode` does not pool buffers. Callers who want 1-bit output and
pooling can use their own `png.Encoder` with a `BufferPool`.

Direct placement allocates once more than before (row and column offset
tables plus the corner bitset instead of one grid) while writing a third
less memory. The `Encoder` path allocates only the output matrix.
