SHELL := /bin/bash

COMPOSE_FILE ?= docker-compose.yml
DC := docker compose -f $(COMPOSE_FILE)

# Optional service for service-scoped commands, e.g. make logs-svc SVC=api-gateway
SVC ?=
# Optional command for exec/run helpers
CMD ?= sh

.PHONY: help compose up up-d down down-v stop start restart build build-no-cache pull push \
	ps top images config logs logs-f logs-svc shell exec run \
	infra-up infra-down app-up app-down clean prune

help: ## Show available commands
	@echo "Microblog Docker Compose commands"
	@echo ""
	@grep -E '^[a-zA-Z0-9_.-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make up-d"
	@echo "  make logs-f"
	@echo "  make logs-svc SVC=auth-service"
	@echo "  make shell SVC=api-gateway"
	@echo "  make exec SVC=user-service CMD='ls -la /app'"
	@echo "  make compose ARGS='events --since 10m'"

compose: ## Pass through any docker compose args: make compose ARGS='ps'
	$(DC) $(ARGS)

up: ## Start all services in foreground
	$(DC) up

up-d: ## Start all services in detached mode
	$(DC) up -d

down: ## Stop and remove containers/network
	$(DC) down

down-v: ## Stop and remove containers/network/volumes
	$(DC) down -v

stop: ## Stop running services
	$(DC) stop

start: ## Start existing services
	$(DC) start

restart: ## Restart all services
	$(DC) restart

build: ## Build or rebuild services
	$(DC) build

build-no-cache: ## Build services without cache
	$(DC) build --no-cache

pull: ## Pull service images
	$(DC) pull

push: ## Push service images
	$(DC) push

ps: ## List containers
	$(DC) ps

top: ## Display running processes
	$(DC) top

images: ## List images used by compose services
	$(DC) images

config: ## Validate and view resolved compose config
	$(DC) config

logs: ## Show logs for all services
	$(DC) logs

logs-f: ## Follow logs for all services
	$(DC) logs -f --tail=200

logs-svc: ## Follow logs for one service (SVC=service-name)
	@test -n "$(SVC)" || (echo "SVC is required. Example: make logs-svc SVC=api-gateway" && exit 1)
	$(DC) logs -f --tail=200 $(SVC)

shell: ## Open shell in service container (SVC=service-name)
	@test -n "$(SVC)" || (echo "SVC is required. Example: make shell SVC=api-gateway" && exit 1)
	$(DC) exec $(SVC) sh

exec: ## Exec custom command in service container (SVC=..., CMD='...')
	@test -n "$(SVC)" || (echo "SVC is required. Example: make exec SVC=auth-service CMD='ls -la'" && exit 1)
	$(DC) exec $(SVC) $(CMD)

run: ## Run one-off command in new service container (SVC=..., CMD='...')
	@test -n "$(SVC)" || (echo "SVC is required. Example: make run SVC=post-service CMD='go test ./...'" && exit 1)
	$(DC) run --rm $(SVC) $(CMD)

infra-up: ## Start infrastructure only (redis, postgres, rabbitmq, opensearch, kafka)
	$(DC) up -d redis postgres_user postgres_post postgres_notification rabbitmq opensearch kafka

infra-down: ## Stop infrastructure only
	$(DC) stop redis postgres_user postgres_post postgres_notification rabbitmq opensearch kafka

app-up: ## Start app services only (without infra)
	$(DC) up -d auth-service user-service post-service notification-service search-service api-gateway

app-down: ## Stop app services only
	$(DC) stop auth-service user-service post-service notification-service search-service api-gateway

clean: ## Compose down + remove volumes
	$(DC) down -v --remove-orphans

prune: ## Compose down + remove volumes and local images
	$(DC) down -v --rmi local --remove-orphans
