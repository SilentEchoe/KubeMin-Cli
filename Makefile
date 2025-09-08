SHELL := /bin/bash

# Module / binaries
MODULE      ?= KubeMin-Cli
BINARY_NAME ?= kubemin-cli
OUT_DIR     ?= bin
OUT         := $(OUT_DIR)/$(BINARY_NAME)
GOCACHE     ?= $(abspath $(OUT_DIR)/.gocache)

# Go settings
GO           ?= go
CGO_ENABLED  ?= 0
GOFLAGS      ?=
LDFLAGS      ?=

# Main entrypoint
MAIN_PKG := ./cmd

.PHONY: all build build-linux build-darwin build-windows clean test tidy fmt vet run

all: build

build:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOCACHE=$(GOCACHE) $(GO) build $(GOFLAGS) -o $(OUT) $(LDFLAGS) $(MAIN_PKG)
	@echo "Built $(OUT)"

# Cross-compile examples
build-linux:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOCACHE=$(GOCACHE) GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUT)-linux-amd64 $(LDFLAGS) $(MAIN_PKG)

build-darwin:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOCACHE=$(GOCACHE) GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUT)-darwin-amd64 $(LDFLAGS) $(MAIN_PKG)

build-windows:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOCACHE=$(GOCACHE) GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUT)-windows-amd64.exe $(LDFLAGS) $(MAIN_PKG)

run: build
	./$(OUT)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(OUT_DIR)
