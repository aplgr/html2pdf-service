.PHONY: help start stop restart build logs ps pull clean examples-index cert

COMPOSE_FILE := deploy/docker-compose.yml
DC := docker compose -f $(COMPOSE_FILE)

# Use bash for stricter error handling (pipefail)
SHELL := /usr/bin/env bash

help:
	@echo "Targets:"
	@echo "  make start          Build & start the stack (detached)"
	@echo "  make stop           Stop the stack (keeps volumes)"
	@echo "  make restart        Restart the stack"
	@echo "  make build          Build images"
	@echo "  make ps             Show container status"
	@echo "  make logs           Tail logs"
	@echo "  make pull           Pull base images"
	@echo "  make clean          Stop stack and remove volumes"
	@echo "  make examples-index Regenerate examples/index.json"
	@echo "  make cert           Generate a local self-signed TLS cert"

cert:
	@mkdir -p gateway/envoy/tls
	@rm -f gateway/envoy/tls/tls.key gateway/envoy/tls/tls.crt
	@openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
		-keyout gateway/envoy/tls/tls.key \
		-out gateway/envoy/tls/tls.crt \
		-subj "/CN=localhost"
	@chmod 644 gateway/envoy/tls/tls.key gateway/envoy/tls/tls.crt

examples-index:
	@./scripts/generate_examples_index.sh ./examples ./examples/index.json


start: examples-index cert
	@$(DC) up -d --build

stop:
	@$(DC) down

restart: stop start

build: examples-index
	@$(DC) build

ps:
	@$(DC) ps

logs:
	@$(DC) logs -f --tail=200

pull:
	@$(DC) pull

clean:
	@$(DC) down -v


### TESTING ###

# List of Go modules (relative to repo root)
MODULES := services/auth-service services/pdf-renderer

# Race tests require cgo. Allow override: CGO_ENABLED=0 make test-race
CGO_ENABLED ?= 1
export CGO_ENABLED

.PHONY: test-all test test-race test-cover test-integration lint fmt tidy clean-tests

# Run everything (tests, race, coverage, lint)
test-all: test test-race test-cover lint

# Run unit tests for all modules
test:
	@set -euo pipefail; \
	for m in $(MODULES); do \
		echo "==> $$m: unit tests"; \
		go -C $$m test ./...; \
	done

# Run race tests for all modules
test-race:
	@set -euo pipefail; \
	for m in $(MODULES); do \
		echo "==> $$m: race tests"; \
		go -C $$m test -race ./...; \
	done

# Run coverage for all modules and write coverage profiles + summaries to ./coverage/
test-cover:
	@set -euo pipefail; \
	coverage_dir="$$PWD/coverage"; \
	mkdir -p "$$coverage_dir"; \
	for m in $(MODULES); do \
		name=$$(basename $$m); \
		profile="$$coverage_dir/coverage-$${name}.out"; \
		html="$$coverage_dir/coverage-$${name}.html"; \
		txt="$$coverage_dir/coverage-$${name}.txt"; \
		echo "==> $$m: coverage"; \
		go -C $$m test ./... -coverprofile="$$profile"; \
		echo "==> $$m: coverage (summary + html report)"; \
		go -C $$m tool cover -func="$$profile" | tee "$$txt" | tail -n 1; \
		go -C $$m tool cover -html="$$profile" -o "$$html"; \
	done


# Run stack-level integration tests against docker compose services
test-integration:
	@./scripts/test_integration.sh

# Run golangci-lint for all modules (expects golangci-lint installed locally)
lint:
	@set -euo pipefail; \
	for m in $(MODULES); do \
		echo "==> $$m: golangci-lint"; \
		( cd $$m && golangci-lint run ./... ); \
	done

# Run gofmt for all modules
fmt:
	@set -euo pipefail; \
	for m in $(MODULES); do \
		echo "==> $$m: gofmt"; \
		( cd $$m && gofmt -w -s $$(go list -f '{{.Dir}}' ./... 2>/dev/null) ); \
	done

# Run go mod tidy for all modules
tidy:
	@set -euo pipefail; \
	for m in $(MODULES); do \
		echo "==> $$m: go mod tidy"; \
		go -C $$m mod tidy; \
	done

# Clean generated artifacts
clean-tests:
	rm -rf coverage
