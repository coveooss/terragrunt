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

coveralls:
	@sh ./scripts/coverage.sh --coveralls

html-coverage:
	@sh ./scripts/coverage.sh --html

build:
	go generate -x ./...
	go build

install:
	glide install
	go install

.PHONY: test