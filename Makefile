GO      ?= go
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)
BIN      = specd

.PHONY: all build install test lint clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	$(GO) install -ldflags "$(LDFLAGS)" .

test:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...

clean:
	rm -f $(BIN)
