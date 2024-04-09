GOFMT_FILES?=$$(find . -name '*.go' | grep -v vendor)

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

fmt:
	@echo "Running source files through gofmt..."
	gofmt -w $(GOFMT_FILES)

test:
	go test -v -short ./...

full-test:
	go test -v ./...

static:
	go install honnef.co/go/tools/cmd/staticcheck@2023.1.7
	staticcheck --version

	staticcheck ./...

pre-commit: fmt static test

build:
	go install golang.org/x/tools/cmd/stringer@latest
	go install github.com/cheekybits/genny@latest

	go generate -x ./...
	go build

install:
	go install

.PHONY: test
