DEV_DATABASE_URL  ?= postgres://tally:tally@127.0.0.1:5442/tally?sslmode=disable
TEST_DATABASE_URL ?= postgres://tally:tally@127.0.0.1:5442/tally_test?sslmode=disable

# Development-only secrets. The admin password for local development is "tally-dev-password".
DEV_API_SECRET ?= dev-api-secret-0123456789abcdef0123456789
DEV_ADMIN_PASSWORD_HASH ?= $(shell cat .dev-admin-hash 2>/dev/null)

.PHONY: db db-down migrate run test test-integration lint tracker web-dev check dev-hash

db: ## Start the local Postgres
	docker compose up -d --wait postgres

db-down:
	docker compose down

migrate: ## Apply migrations to the development database
	DATABASE_URL='$(DEV_DATABASE_URL)' go run ./cmd/migrate up

dev-hash: ## Create the development admin password hash (.dev-admin-hash, git-ignored)
	@test -f .dev-admin-hash || (printf 'tally-dev-password\n' | go run ./cmd/server hash-password > .dev-admin-hash)

run: dev-hash ## Run the server against the development database
	DATABASE_URL='$(DEV_DATABASE_URL)' API_SECRET='$(DEV_API_SECRET)' \
	ADMIN_PASSWORD_HASH="$$(cat .dev-admin-hash)" LOG_LEVEL=debug \
	go run ./cmd/server

tracker: ## Rebuild the minified tracker after editing tracker/t.js
	go generate ./tracker

test: ## Unit tests only
	go test ./...

test-integration: ## Unit and integration tests (needs `make db`)
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -p 1 -count=1 ./...

lint:
	go vet ./...
	golangci-lint run ./...

web-dev:
	cd web && API_URL=http://127.0.0.1:8090 API_SECRET='$(DEV_API_SECRET)' \
	SESSION_SECRET=dev-session-secret-0123456789abcdef0123 pnpm dev

check: lint test-integration
