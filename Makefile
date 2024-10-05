fmt:
	@echo "Running source files through go fmt ..."
	go fmt ./...

test:
	go test -v -short ./...

full-test:
	go test -v ./...

static:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck --version
	staticcheck ./...

pre-commit: fmt static test

build:
	go install golang.org/x/tools/cmd/stringer@v0.25.0
	go install github.com/cheekybits/genny@v1.0.0

	go generate -x ./...
	go build

install:
	go install

.PHONY: test
