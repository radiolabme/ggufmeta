SHELL := /bin/sh
.SHELLFLAGS := -eu -o pipefail -c

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
TARGET ?= ggufmeta
LOCALBIN := bin

.DEFAULT_GOAL := release

.PHONY: release debug install check_deps clean help

release: check_deps
	@set -e; \
	mkdir -p "$(LOCALBIN)"; \
	BUILD_DIR=$$(mktemp -d 2>/dev/null || mktemp -d -t ggufbuild); \
	echo "[*] Using build dir: $$BUILD_DIR"; \
	mkdir -p "$$BUILD_DIR/cmd"; \
	cp -R "$(CURDIR)/cmd/ggufmeta/." "$$BUILD_DIR/cmd/ggufmeta/"; \
	if ! ls "$$BUILD_DIR/cmd/ggufmeta/"*.go >/dev/null 2>&1; then \
		echo "Error: no .go files were copied into $$BUILD_DIR/cmd/ggufmeta"; \
		exit 1; \
	fi; \
	ls -1 "$$BUILD_DIR/cmd/ggufmeta" | sed 's/^/[src] /'; \
	( cd "$$BUILD_DIR"; \
	  go mod init example.com/gguf >/dev/null 2>&1 || true; \
	  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$(TARGET)" ./cmd/ggufmeta; \
	); \
	cp "$$BUILD_DIR/$(TARGET)" "$(CURDIR)/$(LOCALBIN)/$(TARGET)"; \
	rm -rf "$$BUILD_DIR"; \
	echo "[*] Built $(LOCALBIN)/$(TARGET)"; \
	echo "# Install system-wide (optional): sudo cp ./$(LOCALBIN)/$(TARGET) $(BINDIR)/$(TARGET)"

debug: check_deps
	@set -e; \
	mkdir -p "$(LOCALBIN)"; \
	BUILD_DIR=$$(mktemp -d 2>/dev/null || mktemp -d -t ggufbuild); \
	echo "[*] Using build dir: $$BUILD_DIR"; \
	mkdir -p "$$BUILD_DIR/cmd"; \
	cp -R "$(CURDIR)/cmd/ggufmeta/." "$$BUILD_DIR/cmd/ggufmeta/"; \
	if ! ls "$$BUILD_DIR/cmd/ggufmeta/"*.go >/dev/null 2>&1; then \
		echo "Error: no .go files were copied into $$BUILD_DIR/cmd/ggufmeta"; \
		exit 1; \
	fi; \
	ls -1 "$$BUILD_DIR/cmd/ggufmeta" | sed 's/^/[src] /'; \
	( cd "$$BUILD_DIR"; \
	  go mod init example.com/gguf >/dev/null 2>&1 || true; \
	  CGO_ENABLED=0 go build -gcflags="all=-N -l" -o "$(TARGET)-debug" ./cmd/ggufmeta; \
	); \
	cp "$$BUILD_DIR/$(TARGET)-debug" "$(CURDIR)/$(LOCALBIN)/$(TARGET)-debug"; \
	rm -rf "$$BUILD_DIR"; \
	echo "[*] Built $(LOCALBIN)/$(TARGET)-debug with debug symbols"; \
	echo "# Debug with: dlv exec ./$(LOCALBIN)/$(TARGET)-debug -- /path/to/model.gguf"

install: release
	@echo "[*] Installing $(TARGET) into $(DESTDIR)$(BINDIR)"
	@install -Dm755 "./$(LOCALBIN)/$(TARGET)" "$(DESTDIR)$(BINDIR)/$(TARGET)"

check_deps:
	@if ! command -v go >/dev/null 2>&1; then \
		echo "Error: Go toolchain not found. Please install Go (>= 1.20) and ensure it is on your PATH."; \
		exit 1; \
	fi

clean:
	@rm -rf "./$(LOCALBIN)"

help:
	@echo "Targets:"
	@echo "  make (release)  Build $(LOCALBIN)/$(TARGET) in a mktemp dir (default)"
	@echo "  make debug      Build $(LOCALBIN)/$(TARGET)-debug with debug symbols for dlv"
	@echo "  make install    Install to $(BINDIR)"
	@echo "  make clean      Remove $(LOCALBIN) directory and all builds"