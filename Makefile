GO      ?= go
PKG     := ./...
FUZZTIME ?= 30s

.PHONY: all test race cover lint vet fuzz bench verify-symbol wasm build clean npm-check snapshot tidy-check generate check-generate conformance

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
	$(GO) test -run=^$$ -bench=. -benchmem .

# verify-symbol renders symbols and decodes them with zxing-cpp through uv,
# outside go.mod.
verify-symbol:
	bash symbol/testdata/verify-decode.sh

build:
	$(GO) build -o gs1 ./cmd/gs1

# tidy-check verifies the zero-dependency promise: go mod tidy changes
# nothing, go.mod has no require block and no go.sum exists.
tidy-check:
	$(GO) mod tidy
	git diff --exit-code -- go.mod go.sum
	@! grep -q '^require' go.mod || { echo "go.mod must not declare dependencies"; exit 1; }
	@test ! -e go.sum || { echo "go.sum must not exist in a zero-dependency module"; exit 1; }
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }

generate:
	$(GO) generate ./...

# bash with pipefail so the target fails on go test, not on grep.
conformance: SHELL := /bin/bash
conformance: .SHELLFLAGS := -o pipefail -c
conformance:
	$(GO) test -count=1 -run '^TestConformance$$|^TestLogisticLabelFixtures$$' -v . | grep -E 'conformance|^(--- |ok|FAIL)'

check-generate: generate
	git diff --exit-code -- ai_table.go

wasm:
	bash wasm/build.sh

# snapshot runs the whole release pipeline locally without a tag: binaries,
# archives, packages, PKGBUILD and cask into dist/, skipping publication,
# container images and SBOMs (which need a registry login and syft).
snapshot:
	goreleaser release --snapshot --clean --skip=publish,docker,sbom

# npm-check builds the WebAssembly bundle and verifies the npm package
# loads through its own loader and ships exactly the published files.
npm-check: wasm
	cp LICENSE wasm/
	node wasm/package.test.mjs
	cd wasm && npm pack --dry-run --json | node pack-check.mjs

clean:
	rm -rf dist
	rm -f gs1 coverage.out wasm/gs1.wasm wasm/wasm_exec.js wasm/LICENSE wasm/example/gs1.wasm wasm/example/wasm_exec.js
