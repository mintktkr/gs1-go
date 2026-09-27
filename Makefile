GO      ?= go
PKG     := ./...
FUZZTIME ?= 30s

.PHONY: all test race cover lint vet fuzz bench verify-symbol wasm build clean

all: vet lint test

test:
	$(GO) test -count=1 $(PKG)

race:
	$(GO) test -count=1 -race $(PKG)

cover:
	$(GO) test -count=1 -coverprofile=coverage.out -covermode=atomic $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

vet:
	$(GO) vet $(PKG)

lint:
	golangci-lint run

fuzz:
	$(GO) test -run=^$$ -fuzz=FuzzParse -fuzztime=$(FUZZTIME) .

bench:
	$(GO) test -run=^$$ -bench=. -benchmem . ./symbol/

verify-symbol:
	bash symbol/testdata/verify-decode.sh

build:
	$(GO) build -o gs1 ./cmd/gs1

wasm:
	bash wasm/build.sh

clean:
	rm -f gs1 coverage.out wasm/gs1.wasm wasm/wasm_exec.js wasm/example/gs1.wasm wasm/example/wasm_exec.js
