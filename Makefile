.PHONY: all test cover bench bench-compare lint vet fmt check tidy clean

GO        ?= go
PKG       ?= ./...
COVERFILE ?= coverage.txt

all: check

## test: run all tests with the race detector
test:
	$(GO) test -race -count=1 $(PKG)

## cover: run tests and produce a coverage report (library packages only;
## the runnable examples under examples/ carry no tests)
cover:
	$(GO) test -race -covermode=atomic -coverprofile=$(COVERFILE) $$($(GO) list ./... | grep -v /examples/)
	$(GO) tool cover -func=$(COVERFILE) | tail -n 1

## bench: run benchmarks
bench:
	$(GO) test -run=^$$ -bench=. -benchmem $(PKG)

## bench-compare: comparative router benchmarks vs gin/chi/echo (separate module)
bench-compare:
	cd benchmarks && $(GO) test -run=^$$ -bench=. -benchmem ./...

## vet: run go vet
vet:
	$(GO) vet $(PKG)

## fmt: format the code
fmt:
	$(GO) fmt $(PKG)

## lint: run golangci-lint (must be installed)
lint:
	golangci-lint run

## tidy: tidy go.mod
tidy:
	$(GO) mod tidy

## check: vet + lint + test
check: vet lint test

## clean: remove build/test artifacts
clean:
	rm -f $(COVERFILE) coverage.html
	$(GO) clean
