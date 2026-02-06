.PHONY: help start stop stop-internal stop-mode restart build logs ps pull clean examples-index cert

COMPOSE_FILE := deploy/docker-compose.yml
mode ?= dev
examples ?= yes
domain ?=
email ?=
PROFILE := $(if $(filter prod,$(mode)),prod,)
DC := docker compose -f $(COMPOSE_FILE) $(if $(PROFILE),--profile $(PROFILE),)

# Use bash for stricter error handling (pipefail)
SHELL := /usr/bin/env bash

help:
	@echo "Targets:"
	@echo "  make start          Start the stack (detached)"
	@echo "  make stop           Stop the stack (keeps volumes)"
	@echo "  make restart        Restart the stack"
	@echo "  make build          Build images"
	@echo "  make ps             Show container status"
	@echo "  make logs           Tail logs"
	@echo "  make pull           Pull base images"
	@echo "  make clean          Stop stack and remove volumes"
	@echo "  make examples-index Regenerate examples/index.json"
	@echo "  make cert           Generate a local self-signed TLS cert"
	@echo "                      mode=prod requires domain=... email=..."
	@echo "                      examples=yes|no controls examples index regeneration (prod only)"

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


start:
	@if [ "$(mode)" = "prod" ]; then \
		if [ "$(examples)" = "yes" ]; then \
			$(MAKE) examples-index; \
		fi; \
		if [ -z "$(domain)" ] || [ -z "$(email)" ]; then \
			echo "ERROR: mode=prod requires domain=... and email=..."; \
			exit 1; \
		fi; \
		mkdir -p deploy/letsencrypt deploy/certbot/www; \
		docker compose -f $(COMPOSE_FILE) --profile prod up -d --no-build; \
		docker compose -f $(COMPOSE_FILE) --profile prod run --rm --entrypoint certbot certbot certonly --webroot -w /var/www/certbot \
			-d $(domain) \
			-m $(email) --agree-tos --no-eff-email \
			--deploy-hook /opt/certbot/copy-certs.sh; \
		docker compose -f $(COMPOSE_FILE) --profile prod up -d certbot --no-build; \
	else \
		$(MAKE) examples-index; \
		$(MAKE) cert; \
		$(DC) up -d --build --remove-orphans --force-recreate; \
	fi

stop:
	@$(MAKE) stop-internal
	@$(MAKE) stop-internal mode=prod

stop-internal:
	@$(DC) down

stop-mode:
	@if [ "$(mode)" = "prod" ]; then \
		$(MAKE) stop-internal mode=prod; \
	else \
		$(MAKE) stop-internal; \
	fi

restart:
	@$(MAKE) stop-mode mode=$(mode)
	@$(MAKE) start mode=$(mode) examples=$(examples) domain="$(domain)" email="$(email)"

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
