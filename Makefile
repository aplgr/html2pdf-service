.PHONY: help start stop restart build logs ps pull clean examples-index

COMPOSE_FILE := deploy/docker-compose.yml
DC := docker compose -f $(COMPOSE_FILE)

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

examples-index:
	@./scripts/generate_examples_index.sh ./examples ./examples/index.json

start: examples-index
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
