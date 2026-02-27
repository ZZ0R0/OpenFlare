.PHONY: build up down restart logs test-e2e smoke purge-cache clean help

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all Docker images
	docker compose build

up: ## Start all services
	docker compose up --build -d

down: ## Stop all services
	docker compose down

restart: ## Restart all services
	docker compose restart

logs: ## Follow proxy logs
	docker compose logs -f proxy

logs-all: ## Follow all service logs
	docker compose logs -f

test-e2e: ## Run E2E tests
	docker compose run --rm e2e

test-e2e-verbose: ## Run E2E tests (verbose)
	docker compose run --rm e2e pytest -v --tb=short

smoke: ## Run smoke tests
	./scripts/smoke.sh

purge-cache: ## Purge all cache
	./scripts/purge-cache.sh

clean: ## Stop services and remove volumes
	docker compose down -v --remove-orphans

status: ## Show service status
	docker compose ps

health: ## Check health of all services
	@echo "=== Proxy ===" && curl -sf http://localhost:8080/healthz && echo "" || echo "UNHEALTHY"
	@echo "=== Testsite ===" && curl -sf http://localhost:8000/healthz && echo "" || echo "UNHEALTHY"

metrics: ## Show proxy metrics
	@curl -sf http://localhost:8080/metrics | grep openflare || echo "No metrics available"

cache-stats: ## Show cache statistics
	@curl -sf -H "Authorization: Bearer change-me" http://localhost:8081/admin/cache/stats | python3 -m json.tool 2>/dev/null || echo "Admin API unavailable"
