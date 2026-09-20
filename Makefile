# Load .env when it exists, so `make` and `go run` agree on configuration.
# .env is gitignored; copy .env.example to create it. The defaults below apply
# when there is no .env, which keeps a fresh clone runnable with no setup.
ifneq (,$(wildcard .env))
include .env
export
endif

DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable
HTTP_ADDR    ?= :8080
export DATABASE_URL HTTP_ADDR

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: ## Start Postgres and wait until it is ready
	docker compose up -d --wait

.PHONY: down
down: ## Stop Postgres and delete its data
	docker compose down -v

.PHONY: seed
seed: ## Load the deterministic synthetic dataset
	go run ./cmd/seed

.PHONY: reset
reset: down up seed ## Recreate the database from scratch and reseed

.PHONY: run
run: ## Run the API server
	go run ./cmd/api

.PHONY: test
test: ## Run all tests with the race detector
	go test -race ./...

.PHONY: lint
lint: ## Static analysis
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...

.PHONY: verify
verify: ## Measure both implementations and write results/
	go run ./cmd/verify

.PHONY: bench
bench: ## Run the invoice benchmarks once
	go test ./internal/invoice/ -run '^$$' -bench . -benchtime 50x

.PHONY: compare
compare: ## Benchmark both implementations n=10 and compare with benchstat
	./scripts/compare.sh

.PHONY: demo
demo: ## Show statement cost per order for both endpoints (server must be running)
	@printf '%-8s %-8s %10s %10s\n' order lines invoice summary
	@printf '%s\n' "-----------------------------------------"
	@for spec in "7 1" "2 5" "1 200"; do \
	  set -- $$spec; \
	  inv=$$(curl -s -D - -o /dev/null http://localhost:8080/orders/$$1/invoice \
	    | grep -i x-query-count | tr -d '\r' | awk '{print $$2}'); \
	  sum=$$(curl -s -D - -o /dev/null http://localhost:8080/orders/$$1/summary \
	    | grep -i x-query-count | tr -d '\r' | awk '{print $$2}'); \
	  printf '%-8s %-8s %10s %10s\n' "$$1" "$$2" "$$inv" "$$sum"; \
	done
	@printf '\ninvoice: fixed, constant. summary: DEFECT-2 non-goal, untouched.\n'

.PHONY: submission
submission: ## Copy the two required docs to ../submission/, stamped with this commit
	@mkdir -p ../submission
	@sha=$$(git rev-parse HEAD); \
	for f in intent directive; do \
	  { echo "<!-- Canonical: https://github.com/pasipiya/invoice-service-quest/blob/$$sha/docs/$$f.md -->"; \
	    echo; cat docs/$$f.md; } > ../submission/$$f.md; \
	  echo "wrote ../submission/$$f.md (stamped $$sha)"; \
	done

.PHONY: env
env: ## Create .env from .env.example if it does not exist
	@test -f .env && echo ".env already exists" || (cp .env.example .env && echo "created .env")

.PHONY: config
config: ## Print the resolved configuration
	@echo "DATABASE_URL = $(DATABASE_URL)"
	@echo "HTTP_ADDR    = $(HTTP_ADDR)"

.PHONY: psql
psql: ## Open a psql shell against the local database
	docker compose exec db psql -U postgres -d invoices
