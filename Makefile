.PHONY: test e2e e2e-up e2e-down

# Run unit/integration tests (no Docker needed).
test:
	go test -race -count=1 ./...

# Run e2e tests against a live Docker stack.
e2e: e2e-up
	@echo "==> Running e2e tests..."
	go test -tags e2e -v -count=1 -timeout 5m ./e2e/... || ($(MAKE) e2e-down; exit 1)
	$(MAKE) e2e-down

# Start the e2e Docker stack (proxy + Postgres + 2 Jellyfin backends).
e2e-up:
	@echo "==> Starting e2e stack..."
	docker compose -f docker-compose.e2e.yml up --build --wait -d

# Tear down the e2e Docker stack.
e2e-down:
	@echo "==> Tearing down e2e stack..."
	docker compose -f docker-compose.e2e.yml down -v --remove-orphans

