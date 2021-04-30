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
	go get honnef.co/go/tools/cmd/staticcheck
	staticcheck ./...
	go mod tidy

pre-commit: fmt static test

build:
	go generate -x ./...
	go build

install:
	go install

.PHONY: test