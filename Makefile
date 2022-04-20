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
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...

pre-commit: fmt static test

codecov:
	@sh ./scripts/coverage.sh --codecov

html-coverage:
	@sh ./scripts/coverage.sh --html

build:
	go generate -x ./...
	go build

install:
	go install

.PHONY: test
