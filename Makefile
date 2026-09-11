SHELL := /bin/bash
BIN := zmk-vim-mode
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
GO ?= go
NVIM ?= nvim
CC ?= cc

.PHONY: all build test test-go test-lua test-firmware fmt vet lint install uninstall doctor clean cross

all: build

UNAME_S := $(shell uname -s)
# Linux is pure Go (static binary); macOS needs cgo for IOKit and Cocoa.
CGO ?= $(if $(filter Darwin,$(UNAME_S)),1,0)
# macOS ties Input Monitoring to the binary's code signature. Go's linker
# leaves a "linker-signed" ad-hoc signature identified as "a.out", which TCC
# cannot hold a grant against, so the binary is re-signed with a stable
# identifier. Set CODESIGN_IDENTITY to a self-signed certificate in your
# keychain to keep the grant across rebuilds; ad-hoc ("-") needs re-granting
# each time the binary changes.
CODESIGN_IDENTITY ?= -
BUNDLE_ID := dev.rafaelromao.zmk-vim-mode

build: ## build the daemon for this platform
	CGO_ENABLED=$(CGO) $(GO) build $(LDFLAGS) -o $(BIN) ./cmd/zmk-vim-mode
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		if codesign --force --sign $(CODESIGN_IDENTITY) --identifier $(BUNDLE_ID) $(BIN) 2>/dev/null; then \
			echo "signed $(BIN) as $(BUNDLE_ID) ($(CODESIGN_IDENTITY))"; \
		else \
			echo "codesign failed with identity '$(CODESIGN_IDENTITY)'."; \
			echo "code-signing identities in your keychain:"; \
			security find-identity -v -p codesigning | sed 's/^/  /'; \
			echo "create one in Keychain Access: Certificate Assistant → Create a Certificate,"; \
			echo "  Identity Type 'Self Signed Root', Certificate Type 'Code Signing' (not the default SSL),"; \
			echo "or build ad-hoc and re-grant Input Monitoring after each rebuild:  make install"; \
			exit 1; \
		fi; \
	fi

cross: ## cross-compile for the Omarchy box (linux/amd64 and linux/arm64)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN)-linux-amd64 ./cmd/zmk-vim-mode
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BIN)-linux-arm64 ./cmd/zmk-vim-mode

test: test-go test-lua test-firmware ## run every test suite

test-go: ## Go unit tests (socket tests skip where bind is not permitted)
	$(GO) test ./...

test-lua: ## Neovim plugin tests, headless
	$(NVIM) --clean --headless -l tests/lua/run.lua

test-firmware: ## host-side tests for the ZMK module's decode/timing policy
	@mkdir -p build
	$(CC) -std=c11 -Wall -Wextra -Werror -O1 -o build/test_code_policy firmware/tests/test_code_policy.c
	./build/test_code_policy

fmt: ## format Go sources
	gofmt -w ./cmd ./internal

vet: ## vet for this platform and for Linux
	$(GO) vet ./...
	GOOS=linux GOARCH=amd64 $(GO) vet ./...

lint: fmt vet ## format then vet

install: build ## install the binary and the user service
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)
	@echo "installed $(PREFIX)/bin/$(BIN)"
	@$(PREFIX)/bin/$(BIN) install --nvim --tmux --udev

uninstall: ## remove the user service and the binary
	-$(PREFIX)/bin/$(BIN) uninstall
	-rm -f $(PREFIX)/bin/$(BIN)

doctor: ## check the local setup
	@$(PREFIX)/bin/$(BIN) doctor 2>/dev/null || ./$(BIN) doctor

clean: ## remove build artefacts
	rm -rf build $(BIN) $(BIN)-linux-amd64 $(BIN)-linux-arm64

help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-16s %s\n", $$1, $$2}'
