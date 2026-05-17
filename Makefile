MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := dev/up
.DELETE_ON_ERROR:
.SUFFIXES:

# include makefiles
export SELF ?= $(MAKE)
PROJECT_PATH ?= $(shell pwd)
include $(PROJECT_PATH)/Makefile.*

REPO_NAME ?= $(shell basename $(CURDIR))

#-------------------------------------------------------------------------------
# docker
#-------------------------------------------------------------------------------

## Check if Docker daemon is running
docker/check:
	@docker info > /dev/null 2>&1 || (echo "[ERROR] Docker daemon is not running." && exit 1)
.PHONY: docker/check

DOCKER_USER ?= hansohn
DOCKER_REPO ?= $(REPO_NAME)
DOCKER_TAG_BASE ?= $(DOCKER_USER)/$(DOCKER_REPO)

GO_VERSION ?= 1.24
GOLANGCI_VERSION ?= latest
GOSEC_VERSION ?= latest

# Local Go target platform — used to cross-compile inside Docker for the dev's native OS/arch.
GO_GOOS   ?= $(shell go env GOOS)
GO_GOARCH ?= $(shell go env GOARCH)

GIT_BRANCH ?= $(shell git branch --show-current 2>/dev/null || echo 'unknown')
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'pre')
GIT_VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo $(GIT_HASH))

DOCKER_TAGS ?=
DOCKER_TAGS += --tag $(DOCKER_TAG_BASE):$(GIT_HASH)
ifeq ($(GIT_BRANCH), main)
DOCKER_TAGS += --tag $(DOCKER_TAG_BASE):latest
endif

DOCKER_BUILD_PATH ?= ./docker
DOCKER_BUILD_CACHE_PATH ?= /tmp/.buildx-cache/$(REPO_NAME)

# Platform configuration - default to local platform for single-platform builds with --load
# For multi-platform builds, set DOCKER_PLATFORMS to "linux/amd64,linux/arm64"
DOCKER_LOCAL_PLATFORM ?= $(shell docker version --format '{{.Server.Os}}/{{.Server.Arch}}' 2>/dev/null || echo 'linux/amd64')
DOCKER_PLATFORMS ?= $(DOCKER_LOCAL_PLATFORM)
DOCKER_MULTI_PLATFORM := $(shell echo "$(DOCKER_PLATFORMS)" | grep -q ',' && echo true || echo false)

DOCKER_BUILD_ARGS ?=
DOCKER_BUILD_ARGS += --build-arg GO_VERSION=$(GO_VERSION)
DOCKER_BUILD_ARGS += --build-arg GOLANGCI_VERSION=$(GOLANGCI_VERSION)
DOCKER_BUILD_ARGS += --build-arg GOSEC_VERSION=$(GOSEC_VERSION)
DOCKER_BUILD_ARGS += --platform=$(DOCKER_PLATFORMS)
# Only import cache if it exists and has content
ifneq ($(wildcard $(DOCKER_BUILD_CACHE_PATH)/index.json),)
DOCKER_BUILD_ARGS += --cache-from type=local,src=$(DOCKER_BUILD_CACHE_PATH)
endif
DOCKER_BUILD_ARGS += --cache-to type=local,dest=$(DOCKER_BUILD_CACHE_PATH)
# Only add --load for single-platform builds (multi-platform builds require --push)
ifeq ($(DOCKER_MULTI_PLATFORM),false)
DOCKER_BUILD_ARGS += --load
endif
DOCKER_BUILD_ARGS += $(DOCKER_TAGS)

DOCKER_RUN_ARGS ?=
DOCKER_RUN_ARGS += --interactive
DOCKER_RUN_ARGS += --tty
DOCKER_RUN_ARGS += --rm

DOCKER_PUSH_ARGS ?=
DOCKER_PUSH_ARGS += --all-tags
DOCKER_PUSH_ARGS += --platform=$(DOCKER_PLATFORMS)

## Lint Dockerfile
docker/lint: docker/check
	@echo "[INFO] Linting '$(DOCKER_REPO)/Dockerfile'."
	@docker run --rm -i -v $(abspath $(DOCKER_BUILD_PATH)):/mnt:ro hadolint/hadolint hadolint --failure-threshold error /mnt/Dockerfile
.PHONY: docker/lint

## Docker build image
docker/build: docker/check
	@echo "[INFO] Building '$(DOCKER_USER)/$(DOCKER_REPO)' docker image."
	@docker buildx build --file $(DOCKER_BUILD_PATH)/Dockerfile $(DOCKER_BUILD_ARGS) .
.PHONY: docker/build

## Docker run image
docker/run: docker/check
	@echo "[INFO] Running '$(DOCKER_USER)/$(DOCKER_REPO)' docker image"
	@docker run $(DOCKER_RUN_ARGS) "$(DOCKER_TAG_BASE):$(GIT_HASH)"
.PHONY: docker/run

## Docker push image
docker/push: docker/check
	@echo "[INFO] Pushing '$(DOCKER_USER)/$(DOCKER_REPO)' docker image"
	@docker push $(DOCKER_PUSH_ARGS) $(DOCKER_TAG_BASE)
.PHONY: docker/push

## Docker clean build images
docker/clean: docker/check
	@if docker inspect --type=image "$(DOCKER_TAG_BASE):$(GIT_HASH)" > /dev/null 2>&1; then \
		echo "[INFO] Removing docker image '$(DOCKER_USER)/$(DOCKER_REPO)'"; \
		docker rmi -f $$(docker inspect --format='{{ .Id }}' --type=image $(DOCKER_TAG_BASE):$(GIT_HASH)); \
	fi
	@if [ -d "$(DOCKER_BUILD_CACHE_PATH)" ] && [ "$$(ls -A $(DOCKER_BUILD_CACHE_PATH))" ]; then \
		echo "[INFO] Removing docker build cache found at '$(DOCKER_BUILD_CACHE_PATH)'"; \
		rm -rf $(DOCKER_BUILD_CACHE_PATH)/*; \
	fi
.PHONY: docker/clean

## Initialize development environment
dev/up: docker/lint docker/build docker/run
.PHONY: dev/up

#-------------------------------------------------------------------------------
# go
#-------------------------------------------------------------------------------

## Lint Go code (docker)
go/lint: docker/check
	@echo "[INFO] Linting Go code using Docker..."
	@docker buildx build --file $(DOCKER_BUILD_PATH)/Dockerfile --target lint .
.PHONY: go/lint

## Security analysis (docker)
go/security: docker/check
	@echo "[INFO] Running security analysis using Docker..."
	@docker buildx build --file $(DOCKER_BUILD_PATH)/Dockerfile --target security .
.PHONY: go/security

## Build binary locally (native)
go/build-local:
	@echo "[INFO] Building hvm binary locally..."
	@mkdir -p bin
	@go build -ldflags "-X main.version=$(GIT_VERSION)" -o bin/hvm .
.PHONY: go/build-local

## Build binary for local OS/arch (docker)
go/build: docker/check
	@echo "[INFO] Building hvm for $(GO_GOOS)/$(GO_GOARCH) using Docker..."
	@docker buildx build \
		--file $(DOCKER_BUILD_PATH)/Dockerfile \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg GOOS=$(GO_GOOS) \
		--build-arg GOARCH=$(GO_GOARCH) \
		--build-arg VERSION=$(GIT_VERSION) \
		--target export \
		--output type=local,dest=./bin .
.PHONY: go/build

## Run tests locally (native)
go/test-local:
	@echo "[INFO] Running Go tests locally..."
	@go test ./...
.PHONY: go/test-local

## Run tests (docker)
go/test: docker/check
	@echo "[INFO] Running Go tests using Docker..."
	@docker buildx build --file $(DOCKER_BUILD_PATH)/Dockerfile --target test .
.PHONY: go/test

## Clean Go build artifacts
go/clean:
	@echo "[INFO] Cleaning Go build artifacts..."
	@go clean -cache -modcache -i -r
	@rm -f coverage.out coverage.html
.PHONY: go/clean

## Run all checks (lint, security, build, test)
go/check: go/lint go/security go/build go/test
.PHONY: go/check

#-------------------------------------------------------------------------------
# clean
#-------------------------------------------------------------------------------

## Clean everything
clean: docker/clean
.PHONY: clean
