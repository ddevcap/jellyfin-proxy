.PHONY: test e2e e2e-up e2e-down jellyfin-up jellyfin-down jellyfin-setup

# Run unit/integration tests (no Docker needed).
test:
	go test -race -count=1 ./...

# ── Dev Jellyfin servers ──────────────────────────────────────────────────────

# Start both dev Jellyfin servers and run the setup wizard automatically.
jellyfin-up:
	@echo "==> Starting Jellyfin dev servers..."
	docker compose -f docker-compose.jellyfin.yml up -d
	$(MAKE) jellyfin-setup

# Tear down the dev Jellyfin servers.
jellyfin-down:
	@echo "==> Stopping Jellyfin dev servers..."
	docker compose -f docker-compose.jellyfin.yml down

# Run the startup wizard on both servers (idempotent — skips if already done).
jellyfin-setup:
	@echo "==> Setting up Jellyfin server 1..."
	./scripts/setup-jellyfin.sh http://localhost:8196
	@echo "==> Setting up Jellyfin server 2..."
	./scripts/setup-jellyfin.sh http://localhost:8296

# ── E2E tests ─────────────────────────────────────────────────────────────────

# Run e2e tests against a live Docker stack.
e2e: e2e-up
	@echo "==> Running e2e tests..."
	go test -tags e2e -v -count=1 -timeout 10m ./e2e/... || ($(MAKE) e2e-down; exit 1)
	$(MAKE) e2e-down

# Start the e2e Docker stack (proxy + Postgres + 2 Jellyfin backends).
e2e-up:
	@echo "==> Starting e2e stack..."
	docker compose -f docker-compose.e2e.yml up --build --wait -d

# Tear down the e2e Docker stack.
e2e-down:
	@echo "==> Tearing down e2e stack..."
	docker compose -f docker-compose.e2e.yml down -v --remove-orphans

