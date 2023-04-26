BIN = anchore-k8s-inventory
TEMPDIR = ./.tmp
RESULTSDIR = $(TEMPDIR)/results
COVER_REPORT = $(RESULTSDIR)/cover.report
COVER_TOTAL = $(RESULTSDIR)/cover.total
LICENSES_REPORT = $(RESULTSDIR)/licenses.json
LINTCMD = $(TEMPDIR)/golangci-lint run --config .golangci.yaml
BOLD := $(shell tput -T linux bold)
PURPLE := $(shell tput -T linux setaf 5)
GREEN := $(shell tput -T linux setaf 2)
CYAN := $(shell tput -T linux setaf 6)
RED := $(shell tput -T linux setaf 1)
RESET := $(shell tput -T linux sgr0)
TITLE := $(BOLD)$(PURPLE)
SUCCESS := $(BOLD)$(GREEN)
# the quality gate lower threshold for unit test total % coverage (by function statements)
COVERAGE_THRESHOLD := 50

CLUSTER_NAME=anchore-k8s-inventory-testing

GOLANG_CI_VERSION=v1.52.2
GOBOUNCER_VERSION=v0.3.0
GORELEASER_VERSION=v1.4.1

## Build variables
DISTDIR=./dist
SNAPSHOTDIR=./snapshot
GITTREESTATE=$(if $(shell git status --porcelain),dirty,clean)
SNAPSHOT_CMD=$(shell realpath $(shell pwd)/$(SNAPSHOTDIR)/anchore-k8s-inventory_linux_amd64/anchore-k8s-inventory)

ifeq "$(strip $(VERSION))" ""
 override VERSION = $(shell git describe --always --tags --dirty)
endif

## Variable assertions

ifndef TEMPDIR
	$(error TEMPDIR is not set)
endif

ifndef RESULTSDIR
	$(error RESULTSDIR is not set)
endif

ifndef DISTDIR
	$(error DISTDIR is not set)
endif

ifndef SNAPSHOTDIR
	$(error SNAPSHOTDIR is not set)
endif

define title
    @printf '$(TITLE)$(1)$(RESET)\n'
endef

.PHONY: all
all: clean static-analysis unit ## Run all checks (linting, license check, unit tests)
	@printf '$(SUCCESS)All checks pass!$(RESET)\n'

.PHONY: test
test: unit integration ## Run all tests (unit, integration)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(BOLD)$(CYAN)%-25s$(RESET)%s\n", $$1, $$2}'

ci-bootstrap: bootstrap
	sudo apt update && sudo apt install -y bc jq

.PHONY: bootstrap
bootstrap: ## Download and install all go dependencies (+ prep tooling in the ./tmp dir)
	$(call title,Boostrapping dependencies)
	@pwd
	# prep temp dirs
	mkdir -p $(TEMPDIR)
	mkdir -p $(RESULTSDIR)
	go version || ./scripts/install-go.sh
	# install go dependencies
	go mod download
	# install utilities
	[ -f "$(TEMPDIR)/golangci" ] || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TEMPDIR) $(GOLANG_CI_VERSION)
	[ -f "$(TEMPDIR)/bouncer" ] || curl -sSfL https://raw.githubusercontent.com/wagoodman/go-bouncer/master/bouncer.sh | sh -s -- -b $(TEMPDIR) $(GOBOUNCER_VERSION)
	[ -f "$(TEMPDIR)/goreleaser" ] || GOBIN=$(abspath $(TEMPDIR)) go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)

.PHONY: install-cluster-deps
install-cluster-deps: ## Install Helm and Kubectl
	./scripts/install-cluster-deps.sh

.PHONY: static-analysis
static-analysis: lint check-licenses

.PHONY: lint
lint: ## Run gofmt + golangci lint checks
	$(call title,Running linters)

	# run all golangci-lint rules
	$(LINTCMD)

	# go tooling does not play well with certain filename characters, ensure the common cases don't result in future "go get" failures
	$(eval MALFORMED_FILENAMES := $(shell find . | grep -e ':'))
	@bash -c "[[ '$(MALFORMED_FILENAMES)' == '' ]] || (printf '\nfound unsupported filename characters:\n$(MALFORMED_FILENAMES)\n\n' && false)"

.PHONY: lint-fix
lint-fix: ## Auto-format all source code + run golangci lint fixers
	$(call title,Running lint fixers)
	gofmt -w -s .
	$(LINTCMD) --fix

.PHONY: check-licenses
check-licenses:
	$(TEMPDIR)/bouncer check

.PHONY: unit
unit: ## Run unit tests (with coverage)
	$(call title,Running unit tests)
	mkdir -p $(RESULTSDIR)
	go test -coverprofile $(COVER_REPORT) `go list ./... | grep -v test`
	@go tool cover -func $(COVER_REPORT) | grep total |  awk '{print substr($$3, 1, length($$3)-1)}' > $(COVER_TOTAL)
	@echo "Coverage: $$(cat $(COVER_TOTAL))"
	@if [ $$(echo "$$(cat $(COVER_TOTAL)) >= $(COVERAGE_THRESHOLD)" | bc -l) -ne 1 ]; then echo "$(RED)$(BOLD)Failed coverage quality gate (> $(COVERAGE_THRESHOLD)%)$(RESET)" && false; fi

.PHONY: cluster-up
cluster-up: ## Bring up a kind cluster
	$(call title,Starting Kind Cluster)
	./scripts/cluster-up.sh $(CLUSTER_NAME)

.PHONY: cluster-down
cluster-down: ## Stop and delete kind cluster
	$(call title,Tearing Down Kind Cluster)
	./scripts/cluster-down.sh $(CLUSTER_NAME)

.PHONY: integration
integration: ## Run integration tests
	$(call title,Running integration tests)
	./test/integration/test-integration.sh $(CLUSTER_NAME)

.PHONY: check-pipeline
check-pipeline: ## Run local CircleCI pipeline locally (sanity check)
	$(call title,Check pipeline)
	# note: this is meant for local development & testing of the pipeline, NOT to be run in CI
	mkdir -p $(TEMPDIR)
	circleci config process .circleci/config.yml > .tmp/circleci.yml
	circleci local execute -c .tmp/circleci.yml --job "Static Analysis"
	circleci local execute -c .tmp/circleci.yml --job "Unit & Integration Tests (go-latest)"
	@printf '$(SUCCESS)Pipeline checks pass!$(RESET)\n'

.PHONY: build
build: $(SNAPSHOTDIR) ## Build release snapshot binaries and packages

.PHONY: linux-binary
linux-binary: clean bootstrap
	mkdir -p $(SNAPSHOTDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o $(SNAPSHOTDIR)/anchore-k8s-inventory .

.PHONY: linux-binary-arm64
linux-binary-arm64: clean bootstrap
	mkdir -p $(SNAPSHOTDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o $(SNAPSHOTDIR)/anchore-k8s-inventory .

.PHONY: mac-binary
mac-binary: clean bootstrap
	mkdir -p $(SNAPSHOTDIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -o $(SNAPSHOTDIR)/anchore-k8s-inventory .

.PHONY: mac-binary-arm64
mac-binary-arm64: clean bootstrap
	mkdir -p $(SNAPSHOTDIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -installsuffix cgo -o $(SNAPSHOTDIR)/anchore-k8s-inventory .

$(SNAPSHOTDIR): ## Build snapshot release binaries and packages
	$(call title,Building snapshot artifacts)
	# create a config with the dist dir overridden
	echo "dist: $(SNAPSHOTDIR)" > $(TEMPDIR)/goreleaser.yaml
	cat .goreleaser.yaml >> $(TEMPDIR)/goreleaser.yaml

	# build release snapshots
	BUILD_GIT_TREE_STATE=$(GITTREESTATE) \
	$(TEMPDIR)/goreleaser release --skip-publish --rm-dist --snapshot --config $(TEMPDIR)/goreleaser.yaml

.PHONY: release
release: clean-dist ## Build and publish final binaries and packages
	$(call title,Publishing release artifacts)
	# create a config with the dist dir overridden
	echo "dist: $(DISTDIR)" > $(TEMPDIR)/goreleaser.yaml
	cat .goreleaser.yaml >> $(TEMPDIR)/goreleaser.yaml

	# release
	BUILD_GIT_TREE_STATE=$(GITTREESTATE) \
	$(TEMPDIR)/goreleaser --rm-dist --config $(TEMPDIR)/goreleaser.yaml

.PHONY: clean
clean: clean-dist clean-snapshot  ## Remove previous builds and result reports
	rm -rf $(RESULTSDIR)/*

.PHONY: clean-snapshot
clean-snapshot:
	rm -rf $(SNAPSHOTDIR) $(TEMPDIR)/goreleaser.yaml

.PHONY: clean-dist
clean-dist:
	rm -rf $(DISTDIR) $(TEMPDIR)/goreleaser.yaml

.PHONY: bootstrap-skaffold
bootstrap-skaffold:
	$(call title, Cloning chart for local skaffold dev)
	git clone https://github.com/anchore/anchore-charts.git
