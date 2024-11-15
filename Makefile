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

## install: install the application
.PHONY: install
install: build
	@$(info Installing binary.)
	install ./bin/bat /usr/local/bin/
	@$(info Installing manual page.)
	mkdir -p /usr/local/share/man/man1 && cp ./bat.1 /usr/local/share/man/man1/

## test: run tests
.PHONY: test
test: build
	@$(info Testing.)
	bin/bat --version | grep --quiet $(TAG)
