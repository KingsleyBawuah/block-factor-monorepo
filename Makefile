export PATH := $(HOME)/.bun/bin:$(HOME)/go/bin:$(PATH)

.PHONY: dev build test lint migrate-up seed generate generate-check

# Start full local development stack (DB, OTP2, Go API with hot-reload, Vite)
dev:
	docker compose up -d db otp
	@echo "Starting Go API (Air) and Vite Frontend..."
	# runs concurrently via air and bun run --bun dev

# Run all unit and integration tests
#
# --isolate: this suite shares one global/module registry across all web
# test files by default (see packages/web/src/test/setup.ts), which CI has
# shown can bleed state between files in a way that never reproduces
# locally -- --isolate resets the global object and module registry between
# each file, trading a little speed for eliminating that class of flake.
test:
	go test -v ./services/api/...
	cd packages/web && bun test --isolate

# Run linters across Go and TypeScript
lint:
	golangci-lint run ./services/api/...
	cd packages/web && bun run --bun typecheck && bun run --bun lint

# Run database migrations
migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

# Ingest and seed Chicago base datasets
seed:
	go run ./services/api/cmd/seed $(ARGS)

# Regenerate OpenAPI contracts and client SDKs
generate:
	cd services/api && oapi-codegen -config openapi.cfg.yaml ../../docs/openapi.yaml
	cd packages/web && bun run --bun generate-api


# Verify generated contracts are up-to-date (CI drift gate)
generate-check: generate
	git add -A -N .
	git diff --exit-code
