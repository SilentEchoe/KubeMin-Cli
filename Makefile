SHELL := /bin/bash

# Module / binaries
MODULE      ?= KubeMin-Cli
BINARY_NAME ?= kubemin-cli

# Build cache (local path to avoid sandbox issues)
GOCACHE     ?= $(abspath ./.gocache)

# Go settings
GO           ?= go
CGO_ENABLED  ?= 0
GOFLAGS      ?=
GO111MODULE  ?= on
LDFLAGS      ?=

# Main entrypoint
MAIN_PKG := ./cmd

# Null device for discarding binaries
DEVNULL := /dev/null
ifeq ($(OS),Windows_NT)
  DEVNULL := NUL
endif

.PHONY: all build build-linux build-darwin build-windows clean test tidy fmt vet run

all: build

build:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOCACHE=$(GOCACHE) $(GO) build $(GOFLAGS) -o $(DEVNULL) $(LDFLAGS) $(MAIN_PKG)
	@echo "Build completed (binary discarded)"

# Cross-compile examples
build-linux:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOCACHE=$(GOCACHE) GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-amd64 $(LDFLAGS) $(MAIN_PKG)

build-darwin:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOCACHE=$(GOCACHE) GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(LDFLAGS) $(MAIN_PKG)

build-windows:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOCACHE=$(GOCACHE) GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-windows-amd64.exe $(LDFLAGS) $(MAIN_PKG)

run:
	$(GO) run $(MAIN_PKG)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(GOCACHE) $(BINARY_NAME)*
