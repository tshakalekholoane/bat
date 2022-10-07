## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## audit: format, vet, and test code
.PHONY: audit 
audit: test
	@echo "Formatting code."
	gofumpt -w .
	@echo "Vetting code."
	go vet ./...
	staticcheck ./...

tag = $(shell git describe --always --dirty --tags --long)
linker_flags = "-s -X 'tshaka.co/bat/internal/cli.tag=${tag}'"

## build: build the cmd/bat application
.PHONY: build
build:
	@echo "Building bat."
	GOOS=linux GOARCH=amd64 go build -ldflags=${linker_flags} ./cmd/bat/

## release: package bat binary into a zip file
.PHONY: release
release: build
	@echo "Packaging bat."
	zip bat.zip ./bat

## test: runs tests
.PHONY: test 
test: 
	@echo "Running tests."
	go test -v -race -vet=off -ldflags=${linker_flags} ./...

## cover: shows application coverage in browser 
.PHONY: cover 
cover: 
	@echo "Running coverage."
	go test -coverprofile=cover.out -ldflags=${linker_flags} ./...
	go tool cover -html=cover.out
