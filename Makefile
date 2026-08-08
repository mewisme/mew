# MewJS — developer interface for build, test, quality, generation, certification,
# release preparation, and repository maintenance.
#
# Usage:
#   make help           Show this help
#   make build          Build production binaries (m, mx)
#   make test           Run full unit and integration suite
#   make test-short     Run fast suite (skips soak / wall-clock)
#   make quality        Formatting, generation, vet, lint, and structural checks
#   make ci             Mirror normal PR CI locally
#   make assets         Regenerate runtime asset manifest
#   make assets-check   Verify runtime asset manifest is current
#   make generate-check Verify all generated files are current
#
# Requirements: GNU Make, Go >= 1.26.5, Python 3, golangci-lint (for lint).
# On Windows without Make, prefer plain `go` commands from CONTRIBUTING.md.

# ── Configuration ──────────────────────────────────────────────────────

GO          ?= go
PYTHON      ?= python3
GIT         ?= git
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= govulncheck

BIN_DIR     := bin
REPORTS_DIR := reports

# Production build metadata.
VERSION     ?= dev
COMMIT      ?= $(shell $(GIT) rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE  ?= $(shell $(GIT) log -1 --format=%cd --date=format:%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS     := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

TESTEXEC           := $(GO) run ./tools/testexec
TESTEXEC_WORKERS    ?= auto
TEST_TIMEOUT        ?= 25m
TEST_SHORT_TIMEOUT  ?= 25m
TEST_INTEGRATION_TIMEOUT ?= 30m
TEST_RACE_TIMEOUT   ?= 40m
TEST_E2E_TIMEOUT    ?= 15m
TEST_CRASH_TIMEOUT  ?= 30m

# Platform. EXE is .exe on Windows; empty elsewhere.
EXE ?=

# Repository root (derived from Makefile location, not caller's CWD).
ROOT := $(dir $(lastword $(MAKEFILE_LIST)))

# ── Help ───────────────────────────────────────────────────────────────

# help is the default target.
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo "MewJS Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "General:"
	@echo "  help              Show this help"
	@echo "  info              Print repository and tool versions"
	@echo "  doctor            Verify required tools and files"
	@echo "  setup             Download modules"
	@echo "  tools             Install repository-pinned developer tools"
	@echo "  verify-tools      Check pinned tool versions are installed"
	@echo ""
	@echo "Formatting and generation:"
	@echo "  fmt               Format changed Go files"
	@echo "  fmt-check         Check formatting without modifying files"
	@echo "  generate          Run every generator (assets, plans)"
	@echo "  generate-check    Fail when generated files are stale"
	@echo "  assets            Regenerate runtime asset manifest"
	@echo "  assets-check      Verify runtime asset manifest is current"
	@echo "  plans             Regenerate plans and checklists"
	@echo "  plans-check       Verify plans and checklists are current"
	@echo ""
	@echo "Build:"
	@echo "  build             Build production binaries (m, mx)"
	@echo "  build-m           Build m binary"
	@echo "  build-mx          Build mx binary"
	@echo "  build-all         Build all binaries (alias for build)"
	@echo "  install           Build and install binaries to GOPATH/bin"
	@echo "  clean-build       Remove build output directory"
	@echo ""
	@echo "Testing:"
	@echo "  test              Run full unit and integration suite (adaptive parallel)"
	@echo "  test-short        Run fast suite (skips soak and wall-clock)"
	@echo "  test-unit         Run unit tests only"
	@echo "  test-integration  Run integration tests (process-level sharding)"
	@echo "  test-e2e          Run runtime E2E and Node version tests"
	@echo "  test-crash        Run crash recovery suite (build tag: crash)"
	@echo "  test-runtime      Run runtime, transform, and node tests"
	@echo "  test-transform    Run transform tests"
	@echo "  test-cli          Run CLI tests"
	@echo "  test-runner       Run runner, process, and lifecycle tests"
	@echo "  test-workspace    Run workspace and snapshot tests"
	@echo "  test-race         Run race detector (requires CGO)"
	@echo "  test-all          Run full and race suites"
	@echo ""
	@echo "Quality:"
	@echo "  vet               Run go vet"
	@echo "  lint              Run golangci-lint"
	@echo "  diff-check        Detect whitespace errors and formatting drift"
	@echo "  staticcheck       Alias for lint (runs inside golangci-lint)"
	@echo "  quality           Run all quality gates"
	@echo ""
	@echo "CI-equivalent:"
	@echo "  ci                Mirror normal PR CI locally (alias for ci-normal)"
	@echo "  ci-normal         Mirror normal PR CI locally"
	@echo "  ci-full-local     Mirror full CI checks runnable on one host"
	@echo "  pre-commit        Fast pre-commit validation"
	@echo "  pre-push          Broader pre-push validation"
	@echo ""
	@echo "Certification:"
	@echo "  cert-runtime      Run runtime certification (full)"
	@echo "  cert-runtime-local Run runtime certification (fast subset)"
	@echo "  cert-runtime-report Run runtime certification with JSON report"
	@echo "  cert-check        Verify certification consistency"
	@echo "  core-cert         [alias] cert-runtime"
	@echo "  core-cert-fast    [alias] cert-runtime-local"
	@echo "  core-cert-security [alias] Run security certification"
	@echo "  core-cert-crash   [alias] Run crash certification"
	@echo "  core-cert-performance [alias] Run performance certification"
	@echo ""
	@echo "Benchmarks:"
	@echo "  bench             Run all benchmarks"
	@echo "  bench-runtime     Run runtime and transform benchmarks"
	@echo "  bench-transform   Run transform benchmarks"
	@echo ""
	@echo "Release preparation:"
	@echo "  release-check     Verify release readiness"
	@echo "  release-build     Build production binaries for release"
	@echo "  version           Print version metadata"
	@echo ""
	@echo "Maintenance:"
	@echo "  tidy              Run go mod tidy"
	@echo "  clean             Remove build artifacts"
	@echo "  clean-cache       Remove Go build cache"
	@echo "  clean-reports     Remove report output directory"
	@echo "  clean-all         Remove all build artifacts, caches, and reports"
	@echo ""
	@echo "Compatibility:"
	@echo "  fuzz-smoke        Smoke-test all fuzz targets"
	@echo "  vuln              Run vulnerability scan"
	@echo "  conformance       Run lock bridge conformance suite"
	@echo "  allowlist         Verify dependency and license allowlists"
	@echo "  install-dev       Install development binaries to PATH"
	@echo "  uninstall-dev     Uninstall development binaries"
	@echo ""
	@echo "Overridable variables:"
	@echo "  GO=$(GO)  PYTHON=$(PYTHON)  GIT=$(GIT)"
	@echo "  GOLANGCI_LINT=$(GOLANGCI_LINT)  GOVULNCHECK=$(GOVULNCHECK)"
	@echo "  TESTEXEC_WORKERS=$(TESTEXEC_WORKERS)  (auto, 1, or explicit N)"
	@echo "  VERSION=$(VERSION)  BIN_DIR=$(BIN_DIR)  REPORTS_DIR=$(REPORTS_DIR)  EXE=$(EXE)"

.PHONY: fmt
fmt: ## Format changed Go files
	@echo "=== fmt ==="
	$(GIT) ls-files -z '*.go' | xargs -0 gofmt -w

.PHONY: fmt-check
fmt-check: ## Check formatting without modifying files
	@echo "=== fmt-check ==="
	@files=$$($(GIT) ls-files -z '*.go' | xargs -0 gofmt -l); \
	if [ -n "$$files" ]; then \
	  echo "The following files need gofmt:" >&2; \
	  echo "$$files" >&2; \
	  exit 1; \
	fi

.PHONY: generate
generate: assets plans ## Run every generator in deterministic order

.PHONY: generate-check
generate-check: ## Fail when generated files are stale
	@echo "=== generate-check ===" && \
	$(MAKE) generate && \
	if ! $(GIT) diff --exit-code -- internal/runtime/assets/manifest.json plans/ >/dev/null 2>&1; then \
	  echo "Generated files are stale. Run 'make generate'." >&2; \
	  $(GIT) diff --stat -- internal/runtime/assets/manifest.json plans/; \
	  $(GIT) checkout -- internal/runtime/assets/manifest.json plans/; \
	  exit 1; \
	fi

.PHONY: assets
assets: ## Regenerate runtime asset manifest
	$(PYTHON) tools/update-runtime-assets.py --write

.PHONY: assets-check
assets-check: ## Verify runtime asset manifest is current
	$(PYTHON) tools/update-runtime-assets.py --check

.PHONY: plans
plans: ## Regenerate plans and checklists
	$(PYTHON) plans/scripts/enrich_and_generate.py

.PHONY: plans-check
plans-check: ## Verify plans and checklists are current
	@$(PYTHON) plans/scripts/enrich_and_generate.py && \
	if ! $(GIT) diff --exit-code -- plans/ >/dev/null 2>&1; then \
	  echo "Plans are stale. Run 'make plans'." >&2; \
	  $(GIT) diff --stat -- plans/; \
	  $(GIT) checkout -- plans/; \
	  exit 1; \
	fi

.PHONY: build
build: build-m build-mx ## Build production binaries (m, mx)

.PHONY: build-m
build-m: ## Build m binary
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/m$(EXE) ./cmd/m

.PHONY: build-mx
build-mx: ## Build mx binary
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/mx$(EXE) ./cmd/mx

.PHONY: build-all
build-all: build ## Build all binaries (alias for build)

.PHONY: install
install: build ## Build and install binaries to GOPATH/bin
	CGO_ENABLED=0 $(GO) install -trimpath -ldflags "$(LDFLAGS)" ./cmd/m ./cmd/mx

.PHONY: clean-build
clean-build: ## Remove build output directory
	@echo "Removing $(BIN_DIR)/"
	rm -rf $(BIN_DIR)

.PHONY: test
test: ## Run full unit and integration suite (adaptive parallel execution)
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -timeout $(TEST_TIMEOUT)

.PHONY: test-short
test-short: ## Run fast suite (skips soak and wall-clock; adaptive parallel)
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -short -timeout $(TEST_SHORT_TIMEOUT)

.PHONY: test-unit
test-unit: ## Run unit tests only (no integration, conformance, or E2E)
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -timeout $(TEST_TIMEOUT) $$(go list ./... | grep -v '/tests/')

.PHONY: test-integration
test-integration: ## Run integration tests (process-level sharding)
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -timeout $(TEST_INTEGRATION_TIMEOUT) ./tests/integration/...

.PHONY: test-e2e
test-e2e: ## Run runtime E2E and Node version tests
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -run 'RuntimeE2E|NodeVersion' -v -timeout $(TEST_E2E_TIMEOUT) ./tests/integration/...

.PHONY: test-runtime
test-runtime: ## Run runtime, transform, and node tests
	$(GO) test ./internal/runtime/... ./internal/transform/... ./internal/node/... -count=1 -timeout $(TEST_TIMEOUT)

.PHONY: test-transform
test-transform: ## Run transform tests
	$(GO) test ./internal/transform/... -count=1 -timeout $(TEST_TIMEOUT)

.PHONY: test-cli
test-cli: ## Run CLI tests
	$(GO) test ./internal/cli/... -count=1 -timeout $(TEST_TIMEOUT)

.PHONY: test-runner
test-runner: ## Run runner/process/lifecycle tests
	$(GO) test ./internal/runner/... ./internal/process/... ./internal/lifecycle/... -count=1 -timeout $(TEST_TIMEOUT)

.PHONY: test-workspace
test-workspace: ## Run workspace and snapshot tests
	$(GO) test ./internal/workspace/... ./internal/snapshot/... -count=1 -timeout $(TEST_TIMEOUT)

.PHONY: test-crash
test-crash: ## Run crash recovery suite (build tag: crash)
	$(TESTEXEC) -workers $(TESTEXEC_WORKERS) -tags crash -timeout $(TEST_CRASH_TIMEOUT) ./tests/integration/...

.PHONY: test-race
test-race: ## Run race detector (requires CGO)
	CGO_ENABLED=1 $(TESTEXEC) -workers $(TESTEXEC_WORKERS) -race -timeout $(TEST_RACE_TIMEOUT)

.PHONY: test-all
test-all: test test-race ## Run full and race suites

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (pinned version)
	$(GOLANGCI_LINT) run ./...

.PHONY: diff-check
diff-check: ## Detect whitespace errors and formatting drift
	$(GIT) diff --check

.PHONY: staticcheck
staticcheck: lint ## Alias: staticcheck runs inside golangci-lint

.PHONY: quality
quality: fmt-check generate-check vet lint diff-check arch-check docs-check allowlist ## Run all quality gates

.PHONY: ci
ci: ci-normal ## Mirror normal PR CI locally (alias)

.PHONY: ci-normal
ci-normal: quality test-short build ## Mirror normal PR CI locally

.PHONY: ci-full-local
ci-full-local: quality test test-race build ## Mirror full CI checks runnable on one host
	@echo ""
	@echo "Note: full CI includes a three-OS matrix, cross-compilation,"
	@echo "lock conformance against real package managers (pnpm/npm/yarn/bun),"
	@echo "crash integration, certification, benchmarks, and a Node version matrix."
	@echo "These require external tools and multiple platforms."
	@echo "Run 'make cert-check' for certification consistency."
	@echo "See .github/workflows/full.yml for the complete suite."

.PHONY: pre-commit
pre-commit: fmt-check vet build test-short ## Fast pre-commit validation

.PHONY: pre-push
pre-push: quality test-short build ## Broader pre-push validation

.PHONY: cert-runtime
cert-runtime: ## Run runtime certification (full)
	$(GO) run ./cmd/m conformance run runtime --json

.PHONY: cert-runtime-local
cert-runtime-local: ## Run runtime certification (fast subset)
	$(GO) run ./cmd/m conformance run runtime --filter runtime-failure --json

.PHONY: cert-runtime-report
cert-runtime-report: ## Run runtime certification with JSON report
	@mkdir -p $(REPORTS_DIR)
	$(GO) run ./cmd/m conformance run runtime --json > $(REPORTS_DIR)/runtime-report.json

.PHONY: cert-check
cert-check: ## Verify certification consistency (no external tools)
	$(GO) test ./internal/conformance/... -count=1
	$(GO) test ./tests/conformance/runner/... -count=1
	$(GO) run ./cmd/m conformance run runtime --filter runtime-failure --json >/dev/null

# Legacy certification aliases (preserved for compatibility).
.PHONY: core-cert
core-cert: ## [alias] cert-runtime
	$(PYTHON) tools/certification/run_core_cert.py core-cert

.PHONY: core-cert-fast
core-cert-fast: ## [alias] cert-runtime-local
	$(PYTHON) tools/certification/run_core_cert.py core-cert-fast

.PHONY: core-cert-security
core-cert-security: ## [alias] Run security certification
	$(PYTHON) tools/certification/run_core_cert.py core-cert-security

.PHONY: core-cert-crash
core-cert-crash: ## [alias] Run crash certification
	$(PYTHON) tools/certification/run_core_cert.py core-cert-crash

.PHONY: core-cert-performance
core-cert-performance: ## [alias] Run performance certification
	$(PYTHON) tools/certification/run_core_cert.py core-cert-performance

.PHONY: bench
bench: ## Run all benchmarks
	$(GO) test ./... -run '^$$' -bench . -benchtime 10x -count=1 -timeout 35m

.PHONY: bench-runtime
bench-runtime: ## Run runtime and transform benchmarks
	$(GO) test ./internal/runtime/... ./internal/transform/... ./internal/node/... \
	  -run '^$$' -bench . -benchtime 10x -count=1 -timeout 20m

.PHONY: bench-transform
bench-transform: ## Run transform benchmarks
	$(GO) test ./internal/transform/... -run '^$$' -bench . -benchtime 10x -count=1 -timeout 10m

.PHONY: release-check
release-check: ## Verify release readiness
	@echo "=== release-check ==="
	@if ! $(GIT) diff --quiet; then \
	  echo "Working tree is dirty. Commit or stash changes first." >&2; \
	  exit 1; \
	fi
	@$(MAKE) generate-check
	@$(MAKE) quality
	@$(MAKE) test
	@$(MAKE) build
	@echo "release-check: OK"

.PHONY: release-build
release-build: ## Build production binaries for release
	@$(MAKE) build VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE)

.PHONY: version
version: ## Print version metadata
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build date: $(BUILD_DATE)"

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GO) mod tidy

.PHONY: clean
clean: clean-build ## Remove build artifacts
	@echo "Done."

.PHONY: clean-cache
clean-cache: ## Remove Go build cache
	$(GO) clean -cache

.PHONY: clean-reports
clean-reports: ## Remove report output directory
	@echo "Removing $(REPORTS_DIR)/"
	rm -rf $(REPORTS_DIR)

.PHONY: clean-all
clean-all: clean-build clean-cache clean-reports ## Remove all build artifacts, caches, and reports
	@echo "All clean."

# ── Internal / shared helpers ──────────────────────────────────────────

.PHONY: arch-check
arch-check: ## Verify production import architecture
	$(GO) test ./internal/archcheck/... -count=1 -run TestProduction

.PHONY: docs-check
docs-check: ## Verify AI instruction files reference TOOLS.md
	$(GO) test ./internal/archcheck/... -count=1 -run TestDocsConsistency

.PHONY: fixtures-check
fixtures-check: ## Verify generated fixtures are current
	$(GO) run ./tools/conformance/verify-fixtures

.PHONY: crash-shards-check
crash-shards-check: ## Verify crash shard assignments
	$(GO) run ./tools/ci/verify-crash-shards

# ── Setup and diagnostics ──────────────────────────────────────────────

.PHONY: info
info: ## Print repository and tool versions
	@echo "Repository root: $(ROOT)"
	@echo "OS:             $$(uname -s)"
	@echo "Architecture:   $$(uname -m)"
	@echo "Go version:     $$($(GO) version)"
	@echo "Python version: $$($(PYTHON) --version 2>&1 || echo 'not found')"
	@echo "Node version:   $$(node --version 2>/dev/null || echo 'not found')"
	@echo "Git commit:     $$($(GIT) rev-parse --short HEAD 2>/dev/null || echo unknown)"
	@echo "Dirty:          $$($(GIT) diff --quiet || echo 'yes'; $(GIT) diff --cached --quiet || echo 'yes (staged)')"

.PHONY: doctor
doctor: ## Verify required tools and files
	@echo "=== doctor ===" && \
	ok=true && \
	if ! command -v $(GO) >/dev/null 2>&1; then \
	  echo "FAIL: go not found" >&2; ok=false; \
	else \
	  echo "  go: $$($(GO) version)"; \
	fi && \
	if ! command -v $(PYTHON) >/dev/null 2>&1; then \
	  echo "FAIL: python3 not found" >&2; ok=false; \
	else \
	  echo "  python: $$($(PYTHON) --version 2>&1)"; \
	fi && \
	if ! command -v $(GIT) >/dev/null 2>&1; then \
	  echo "FAIL: git not found" >&2; ok=false; \
	else \
	  echo "  git: $$($(GIT) --version)"; \
	fi && \
	if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
	  echo "WARN: golangci-lint not found (lint targets will fail)" >&2; \
	else \
	  echo "  golangci-lint: $$($(GOLANGCI_LINT) --version 2>&1 | head -1)"; \
	fi && \
	if ! $(GIT) rev-parse --git-dir >/dev/null 2>&1; then \
	  echo "FAIL: not a git repository" >&2; ok=false; \
	fi && \
	if [ ! -f go.mod ]; then \
	  echo "FAIL: go.mod not found" >&2; ok=false; \
	else \
	  echo "  go.mod: found"; \
	fi && \
	if [ ! -f internal/runtime/assets/manifest.json ]; then \
	  echo "FAIL: manifest.json not found" >&2; ok=false; \
	else \
	  echo "  manifest.json: found"; \
	fi && \
	if [ "$$ok" = false ]; then \
	  echo ""; \
	  echo "Fix the failures above before running other targets."; \
	  exit 1; \
	fi && \
	echo "doctor: OK"

.PHONY: setup
setup: ## Download modules and verify tooling
	$(GO) mod download
	@echo "Setup complete. Run 'make doctor' to verify tooling."

.PHONY: tools
tools: ## Install repository-pinned developer tools
	@set -a && . tools/versions.env && set +a && \
	echo "Installing golangci-lint $${GOLANGCI_LINT_VERSION}..." && \
	$(GO) install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$${GOLANGCI_LINT_VERSION}" && \
	echo "Installing govulncheck $${GOVULNCHECK_VERSION}..." && \
	$(GO) install "golang.org/x/vuln/cmd/govulncheck@$${GOVULNCHECK_VERSION}" && \
	echo "Tools installed. Ensure GOPATH/bin is on your PATH."

.PHONY: verify-tools
verify-tools: ## Check that pinned tool versions are installed
	@echo "=== verify-tools ==="
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
	  echo "FAIL: $(GOLANGCI_LINT) not found. Run 'make tools'." >&2; \
	  exit 1; \
	fi
	@if ! command -v $(GOVULNCHECK) >/dev/null 2>&1; then \
	  echo "FAIL: $(GOVULNCHECK) not found. Run 'make tools'." >&2; \
	  exit 1; \
	fi
	@echo "verify-tools: OK"

# ── Compatibility aliases ──────────────────────────────────────────────

.PHONY: race
race: test-race ## [alias] Run race detector (backward-compatible name)

.PHONY: fuzz-smoke
fuzz-smoke: ## Smoke-test all fuzz targets
	$(PYTHON) tools/fuzz_smoke.py

.PHONY: vuln
vuln: ## Run vulnerability scan
	$(GOVULNCHECK) ./...

.PHONY: conformance
conformance: ## Run lock bridge conformance suite
	$(GO) test ./tests/conformance/... -count=1 -timeout $(TEST_INTEGRATION_TIMEOUT)

.PHONY: allowlist
allowlist: ## Verify dependency and license allowlists
	$(GO) run ./tools/check-license
	$(GO) run ./tools/check-deps
	$(GO) test ./tools/check-deps -count=1

.PHONY: install-dev
install-dev: ## Install development binaries to PATH
	@if [ "$$(uname -s)" = "MINGW"* ] || [ "$$(uname -s)" = "MSYS"* ]; then \
	  pwsh -NoProfile -File scripts/install-dev.ps1; \
	else \
	  ./scripts/install-dev.sh; \
	fi

.PHONY: uninstall-dev
uninstall-dev: ## Uninstall development binaries
	@if [ "$$(uname -s)" = "MINGW"* ] || [ "$$(uname -s)" = "MSYS"* ]; then \
	  pwsh -NoProfile -File scripts/uninstall-dev.ps1; \
	else \
	  ./scripts/uninstall-dev.sh; \
	fi

# Legacy target aliases — preserved for CI and documentation compatibility.
.PHONY: update-runtime-assets
update-runtime-assets: assets ## [alias] Regenerate runtime asset manifest

.PHONY: check-runtime-assets
check-runtime-assets: assets-check ## [alias] Verify runtime asset manifest

# ── Aggregate .PHONY declarations ──────────────────────────────────────

# All public targets are phony. This block must include every non-file target.
.PHONY: help info doctor setup tools verify-tools
.PHONY: fmt fmt-check generate generate-check assets assets-check plans plans-check
.PHONY: build build-m build-mx build-all install clean-build
.PHONY: test test-short test-unit test-integration test-e2e test-crash
.PHONY: test-runtime test-transform test-cli test-runner test-workspace
.PHONY: test-race test-all
.PHONY: vet lint diff-check staticcheck quality arch-check docs-check fixtures-check crash-shards-check
.PHONY: ci ci-normal ci-full-local pre-commit pre-push
.PHONY: cert-runtime cert-runtime-local cert-runtime-report cert-check
.PHONY: core-cert core-cert-fast core-cert-security core-cert-crash core-cert-performance
.PHONY: bench bench-runtime bench-transform
.PHONY: release-check release-build version
.PHONY: tidy clean clean-cache clean-reports clean-all
.PHONY: race fuzz-smoke vuln conformance allowlist install-dev uninstall-dev
.PHONY: update-runtime-assets check-runtime-assets
