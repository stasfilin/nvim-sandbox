GO ?= go
SHELLCHECK ?= shellcheck
GOCACHE ?= /tmp/nvim-sandbox-go-cache
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DOCDIR ?= $(PREFIX)/share/doc/nvim-sandbox
VERSION := $(strip $(shell cat VERSION))
LDFLAGS := -X github.com/stasfilin/nvim-sandbox/internal/cli.version=$(VERSION)
PACKAGES := ./...
ARTIFACT_DIR ?= build-artifacts
RELEASE_DIR ?= release
ARTIFACT_BASENAME ?= nvim-sandbox
PACKAGE_BASENAME ?= nvim-sandbox_$(VERSION)

.PHONY: build check check-static fmt fmt-check install mod-check package-artifacts shellcheck test test-e2e-apple-container test-e2e-apple-container-bin test-e2e-docker test-e2e-docker-bin test-e2e-podman test-e2e-podman-bin test-homebrew test-install test-notices test-packages test-race uninstall vet

build:
	mkdir -p dist
	GOCACHE=$(GOCACHE) $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/nvim-sandbox ./cmd/nvim-sandbox

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 dist/nvim-sandbox "$(DESTDIR)$(BINDIR)/nvim-sandbox"
	install -d "$(DESTDIR)$(DOCDIR)"
	install -m 0644 LICENSE THIRD_PARTY_NOTICES.md "$(DESTDIR)$(DOCDIR)"

uninstall:
	rm -f "$(DESTDIR)$(BINDIR)/nvim-sandbox"
	rm -f "$(DESTDIR)$(DOCDIR)/LICENSE" "$(DESTDIR)$(DOCDIR)/THIRD_PARTY_NOTICES.md"

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (echo "Go files need formatting:"; gofmt -l cmd internal; exit 1)

mod-check:
	GOCACHE=$(GOCACHE) $(GO) mod tidy -diff
	GOCACHE=$(GOCACHE) $(GO) mod verify

vet:
	GOCACHE=$(GOCACHE) $(GO) vet $(PACKAGES)

shellcheck:
	$(SHELLCHECK) scripts/*.sh tests/*.sh

test:
	GOCACHE=$(GOCACHE) $(GO) test $(PACKAGES)

test-race:
	GOCACHE=$(GOCACHE) $(GO) test -race $(PACKAGES)

test-homebrew:
	./tests/homebrew_formula.sh

test-install:
	./tests/install.sh

test-notices:
	./tests/third_party_notices.sh

test-packages:
	./tests/release_packages.sh "$(RELEASE_DIR)" "$(VERSION)" "$(PACKAGE_BASENAME)"

test-e2e-docker: build
	$(MAKE) test-e2e-docker-bin

test-e2e-docker-bin:
	@test -x "$(CURDIR)/dist/nvim-sandbox" || (echo "built nvim-sandbox binary not found: $(CURDIR)/dist/nvim-sandbox"; exit 1)
	NVIM_SANDBOX_BIN="$(CURDIR)/dist/nvim-sandbox" ./tests/e2e_docker.sh

test-e2e-podman: build
	$(MAKE) test-e2e-podman-bin

test-e2e-podman-bin:
	@test -x "$(CURDIR)/dist/nvim-sandbox" || (echo "built nvim-sandbox binary not found: $(CURDIR)/dist/nvim-sandbox"; exit 1)
	NVIM_SANDBOX_BIN="$(CURDIR)/dist/nvim-sandbox" ./tests/e2e_podman.sh

test-e2e-apple-container: build
	$(MAKE) test-e2e-apple-container-bin

test-e2e-apple-container-bin:
	@test -x "$(CURDIR)/dist/nvim-sandbox" || (echo "built nvim-sandbox binary not found: $(CURDIR)/dist/nvim-sandbox"; exit 1)
	NVIM_SANDBOX_BIN="$(CURDIR)/dist/nvim-sandbox" ./tests/e2e_apple_container.sh

package-artifacts:
	./scripts/package_artifacts.sh "$(ARTIFACT_DIR)" "$(RELEASE_DIR)" "$(VERSION)" all "$(PACKAGE_BASENAME)" "$(ARTIFACT_BASENAME)"

check-static: fmt-check mod-check vet shellcheck test-homebrew test-notices

check: check-static test-race test-install
