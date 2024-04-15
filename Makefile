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
	go install honnef.co/go/tools/cmd/staticcheck@2022.1
	staticcheck --version

	staticcheck ./...

pre-commit: fmt static test

build:
	go install golang.org/x/tools/cmd/stringer@v0.19.0
	go install github.com/cheekybits/genny@v1.0.0

	go generate -x ./...
	go build

install:
	go install

.PHONY: test
