SHELL := /bin/bash

# Module / binaries
MODULE      ?= kubemin-cli
BINARY_NAME ?= kubemin-cli


# Go settings
GO           ?= go
CGO_ENABLED  ?= 0
GOFLAGS      ?=
GO111MODULE  ?= on
LDFLAGS      ?=

# Main entrypoint
MAIN_PKG := ./cmd

# Container settings
DOCKER          ?= docker
DOCKER_BUILDER  ?= kubemin-cli-builder
IMAGE           ?= kubemin-cli:latest
DIST_DIR        ?= dist
BUILDX_CACHE    ?= .buildx-cache

# Null device for discarding binaries
DEVNULL := /dev/null
ifeq ($(OS),Windows_NT)
  DEVNULL := NUL
endif

.PHONY: all build build-linux build-darwin build-windows clean test tidy fmt vet run docker-build docker-build-linux docker-build-arm docker-build-macos docker-builder-init

all: build

build:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) $(GO) build $(GOFLAGS) -o $(DEVNULL) $(LDFLAGS) $(MAIN_PKG)
	@echo "Build completed (binary discarded)"

# Cross-compile examples
build-linux:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-amd64 $(LDFLAGS) $(MAIN_PKG)

build-darwin:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(LDFLAGS) $(MAIN_PKG)

build-windows:
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-windows-amd64.exe $(LDFLAGS) $(MAIN_PKG)

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
	rm -rf $(BINARY_NAME)*

docker-build: docker-builder-init docker-build-linux docker-build-arm docker-build-macos

docker-builder-init:
	@$(DOCKER) buildx inspect $(DOCKER_BUILDER) >/dev/null 2>&1 || \
		( $(DOCKER) buildx create --name $(DOCKER_BUILDER) --driver docker-container >/dev/null && \
		  $(DOCKER) buildx inspect --builder $(DOCKER_BUILDER) --bootstrap >/dev/null )
	@$(DOCKER) buildx use $(DOCKER_BUILDER) >/dev/null

docker-build-linux:
	@which $(DOCKER) >/dev/null || (echo "docker not found in PATH"; exit 1)
	@mkdir -p $(BUILDX_CACHE)
	$(DOCKER) buildx build \
		--builder $(DOCKER_BUILDER) \
		--platform linux/amd64 \
		--build-arg TARGETOS=linux \
		--build-arg TARGETARCH=amd64 \
		--cache-from type=local,src=$(BUILDX_CACHE) \
		--cache-to type=local,dest=$(BUILDX_CACHE),mode=max \
		-t $(IMAGE)-linux-amd64 \
		--load .

docker-build-arm:
	@which $(DOCKER) >/dev/null || (echo "docker not found in PATH"; exit 1)
	@mkdir -p $(BUILDX_CACHE)
	$(DOCKER) buildx build \
		--builder $(DOCKER_BUILDER) \
		--platform linux/arm64 \
		--build-arg TARGETOS=linux \
		--build-arg TARGETARCH=arm64 \
		--cache-from type=local,src=$(BUILDX_CACHE) \
		--cache-to type=local,dest=$(BUILDX_CACHE),mode=max \
		-t $(IMAGE)-linux-arm64 \
		--load .

docker-build-macos:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=$(GO111MODULE) GOOS=darwin GOARCH=amd64 \
		$(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(LDFLAGS) $(MAIN_PKG)
	@echo "macOS binary written to $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64"
