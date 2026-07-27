# Variables
APP_NAME=asana
BUILD_DIR=build
# Full describe, not --abbrev=0: a local build several commits past v3.3.2 must
# not report itself as "3.3.2". It becomes v3.3.2-10-gabc1234, which is both
# obviously not a release and pins the exact commit built. Releases take their
# version from goreleaser, not from here.
#
# No --dirty: the sha already identifies the build, and an unrelated untracked
# file would otherwise pin "-dirty" on permanently, where it says nothing.
VERSION := $(shell git describe --tags --always)
LDFLAGS := -ldflags "-X github.com/timwehrle/asana/internal/build.Version=${VERSION} \
					-X github.com/timwehrle/asana/internal/build.Date=${shell date -u +%Y-%m-%dT%H:%M:%SZ}"

# Commands
GOCMD := go
GOMOD := $(GOCMD) mod
GOVET := $(GOCMD) vet
GOTEST := $(GOCMD) test
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOCLEAN := $(GOCMD) clean
LINT := golangci-lint run
GOFMT := gofmt
VULN := golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: build
build: ## Run build
	@echo "Running build"
	$(GOBUILD) ${LDFLAGS} -o $(BUILD_DIR)/$(APP_NAME) ./cmd/asana

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test/cover
test/cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=c.out ./...
	$(GOCMD) tool cover -html=c.out

.PHONY: lint
lint: ## Run linter
	@echo "Running lint..."
	$(LINT)

.PHONY: fmt
fmt: ## Run formatter
	@echo "Formatting code..."
	$(GOFMT) -s -l -e .

.PHONY: audit
audit: ## Audit code
	@echo "Running audit..."
	$(GOMOD) tidy
	$(GOMOD) verify
	$(GOVET) ./...
	$(GORUN) $(VULN) ./...

.PHONY: check/plugin-version
check/plugin-version: ## Verify plugin version matches between marketplace.json and plugin.json
	@./scripts/plugin-version.sh check

.PHONY: sync/plugin-version
sync/plugin-version: ## Copy plugin.json version into marketplace.json
	@./scripts/plugin-version.sh sync

.PHONY: release
release: check/plugin-version ## Run GoReleaser
	@echo "Releasing..."
	goreleaser release --clean

.PHONY: help
help: ## Show available commands
	@grep -E '^[a-zA-Z_/.-]+:.*?##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = "##"}; {printf "%-20s %s\n", $$1, $$2}'
