# FSS — common dev targets.
#
# Run `make help` for the full list.

GO       ?= go
PKGS     := ./...
SMOKE_TIMEOUT ?= 5m
LINT     ?= golangci-lint

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the fss binary into ./fss.
	$(GO) build -o fss .

.PHONY: test
test: ## Run unit tests with race detector (no integration tag).
	$(GO) test -race -count=1 $(PKGS)

.PHONY: smoke
smoke: ## Run integration smoke tests against live sites. Manual only — never in CI.
	@echo "==> Integration smoke tests (live HTTP, not for CI)"
	@echo "==> Tests with placeholder URLs will SKIP. Edit liveStudioURL in each integration_test.go to enable."
	$(GO) test -tags=integration -timeout=$(SMOKE_TIMEOUT) -v ./internal/scrapers/...

.PHONY: smoke-one
smoke-one: ## Run smoke for one scraper. Usage: make smoke-one SCRAPER=manyvids
	@if [ -z "$(SCRAPER)" ]; then echo "usage: make smoke-one SCRAPER=<name>"; exit 1; fi
	$(GO) test -tags=integration -timeout=$(SMOKE_TIMEOUT) -v ./internal/scrapers/$(SCRAPER)/...

.PHONY: vet
vet: ## go vet on all packages (including integration-tagged).
	$(GO) vet $(PKGS)
	$(GO) vet -tags=integration $(PKGS)

.PHONY: lint
lint: vet ## Run go vet + golangci-lint.
	$(LINT) run --timeout=5m

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove built binary and test artifacts.
	rm -f fss fss.exe coverage.out test-output.txt

.PHONY: docker
docker: ## Build the docker image as fss:dev with version metadata from git.
	docker build \
	  --build-arg VERSION=$$(git describe --tags --always --dirty) \
	  --build-arg COMMIT=$$(git rev-parse --short HEAD) \
	  --build-arg DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
	  -t fss:dev .
