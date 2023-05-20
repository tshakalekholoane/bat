BUILD = $(shell date -u +"%Y-%m-%d")
TAG = $(shell git describe --always --dirty --tags --long)
FLAGS = "-s -X $(PKG).build=$(BUILD) -X $(PKG).tag=$(TAG)"

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## audit: format, test, and vet the code
.PHONY: audit
audit: test
	@$(info Formatting and vetting.)
	gofumpt -w .
	go vet ./...
	staticcheck ./...

## build: build the application
.PHONY: build 
build: PKG := main
build: 
	@$(info Building bat.)
	GOOS=linux GOARCH=amd64 go build -ldflags=$(FLAGS) -o=./bin/bat .

## test: runs tests
.PHONY: test
test: PKG := tshaka.dev/x/bat
test: 
	@$(info Running tests.)
	go test -v -race -vet=off -ldflags=$(FLAGS) ./...
