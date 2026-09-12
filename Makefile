SHELL := /bin/bash
BIN := zmk-vim-mode
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
GO ?= go
NVIM ?= nvim
CC ?= cc

.PHONY: all build test test-go test-lua test-firmware fmt vet lint install uninstall doctor clean cross codesign-cert

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
CODESIGN_CERT ?= zmk-vim-mode-dev
# Sign with the self-signed certificate when the keychain has it, so the
# Input Monitoring and Accessibility grants survive rebuilds; ad-hoc otherwise.
CODESIGN_IDENTITY ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep -q '"$(CODESIGN_CERT)"' && echo $(CODESIGN_CERT) || echo -)
BUNDLE_ID := dev.rafaelromao.zmk-vim-mode

# What `install` sets up. The editor integrations skip whatever is not
# installed, so asking for all of them is safe.
INSTALL_FLAGS := --nvim --tmux --vscode --obsidian
ifeq ($(UNAME_S),Linux)
INSTALL_FLAGS += --atspi
endif

build: ## build the daemon for this platform
	CGO_ENABLED=$(CGO) $(GO) build $(LDFLAGS) -o $(BIN) ./cmd/zmk-vim-mode
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		if codesign --force --sign $(CODESIGN_IDENTITY) --identifier $(BUNDLE_ID) $(BIN) 2>/dev/null; then \
			echo "signed $(BIN) as $(BUNDLE_ID) ($(CODESIGN_IDENTITY))"; \
			if [ "$(CODESIGN_IDENTITY)" = "-" ]; then \
				echo "  ad-hoc: each rebuild voids the Input Monitoring and Accessibility grants."; \
				echo "  run 'make codesign-cert' once to keep them."; \
			fi; \
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

install: build ## install everything: binary, service, udev rule, editor integrations
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)
	@echo "installed $(PREFIX)/bin/$(BIN)"
	@$(PREFIX)/bin/$(BIN) install $(INSTALL_FLAGS)
	@if [ "$(UNAME_S)" = "Linux" ]; then \
		if ! cmp -s contrib/udev/60-zmk-vim-mode.rules /etc/udev/rules.d/60-zmk-vim-mode.rules; then \
			echo; echo "--- udev rule (sudo) ---"; \
			sudo install -m 0644 contrib/udev/60-zmk-vim-mode.rules /etc/udev/rules.d/60-zmk-vim-mode.rules \
				&& sudo udevadm control --reload-rules && sudo udevadm trigger \
				&& echo "installed /etc/udev/rules.d/60-zmk-vim-mode.rules"; \
		fi; \
		systemctl --user enable --now zmk-vim-mode.service && echo "service enabled and running"; \
	fi

codesign-cert: ## macOS: create the self-signed certificate that keeps TCC grants across rebuilds
	@set -e; \
	if [ "$(UNAME_S)" != "Darwin" ]; then echo "macOS only"; exit 1; fi; \
	if security find-certificate -c $(CODESIGN_CERT) >/dev/null 2>&1; then \
		echo "$(CODESIGN_CERT) already exists; build with:  make install CODESIGN_IDENTITY=$(CODESIGN_CERT)"; exit 0; \
	fi; \
	d=$$(mktemp -d); \
	: "PKCS#12 has to be written the way macOS's Security framework reads it:"; \
	: "SHA-1 MAC, 3DES, and a real password -- an empty one fails MAC verification."; \
	openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
		-keyout $$d/key.pem -out $$d/cert.pem -subj "/CN=$(CODESIGN_CERT)" \
		-addext "basicConstraints=critical,CA:false" \
		-addext "keyUsage=critical,digitalSignature" \
		-addext "extendedKeyUsage=critical,codeSigning" 2>/dev/null; \
	openssl pkcs12 -export -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg sha1 \
		-inkey $$d/key.pem -in $$d/cert.pem -out $$d/id.p12 -passout pass:zmkvim -name $(CODESIGN_CERT); \
	echo "importing into the login keychain (it may ask to allow codesign to use the key)"; \
	security import $$d/id.p12 -k "$$HOME/Library/Keychains/login.keychain-db" -P zmkvim -T /usr/bin/codesign -A; \
	echo "trusting it for code signing (it will ask for your login password)"; \
	security add-trusted-cert -r trustRoot -p codeSign -k "$$HOME/Library/Keychains/login.keychain-db" $$d/cert.pem; \
	rm -rf $$d; \
	echo; security find-identity -v -p codesigning; \
	echo "now:  make install CODESIGN_IDENTITY=$(CODESIGN_CERT)"; \
	echo "then grant Input Monitoring once; later rebuilds keep it."

uninstall: ## remove the user service and the binary
	-$(PREFIX)/bin/$(BIN) uninstall
	-rm -f $(PREFIX)/bin/$(BIN)

doctor: ## check the local setup
	@$(PREFIX)/bin/$(BIN) doctor 2>/dev/null || ./$(BIN) doctor

clean: ## remove build artefacts
	rm -rf build $(BIN) $(BIN)-linux-amd64 $(BIN)-linux-arm64

help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-16s %s\n", $$1, $$2}'
