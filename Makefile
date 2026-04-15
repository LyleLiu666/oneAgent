.PHONY: build build-frontend sync-frontend build-backend serve doctor release clean openspec-truth benchmark benchmark-smoke docker-up docker-up-sdk-local check-sdk-pins

DIST_DIR := dist
ONEAGENT_BIN := $(DIST_DIR)/oneagent

build: build-backend

build-frontend:
	@cd frontend && ( [ -d node_modules ] || npm ci )
	@cd frontend && npm run build

sync-frontend: build-frontend
	@mkdir -p backend/internal/web/static
	@find backend/internal/web/static -mindepth 1 -maxdepth 1 ! -name .gitkeep -exec rm -rf {} +
	@cp -R frontend/dist/* backend/internal/web/static/

check-sdk-pins:
	@bash scripts/check_sdk_pins.sh

build-backend: check-sdk-pins sync-frontend
	@mkdir -p $(DIST_DIR)
	@cd backend && go build -o ../$(ONEAGENT_BIN) ./cmd/oneagent

serve: build
	@./$(ONEAGENT_BIN) serve

doctor: build
	@./$(ONEAGENT_BIN) doctor

release:
	@bash scripts/release_local.sh

clean:
	@rm -rf $(DIST_DIR)

openspec-truth:
	@bash scripts/openspec_truth_check.sh

benchmark:
	@bash scripts/benchmark_run.sh

benchmark-smoke:
	@BENCHMARK_LIMIT=3 bash scripts/benchmark_run.sh

docker-up:
	@bash scripts/check_sdk_pins.sh
	@docker compose -f docker-compose.yml up -d --build backend

docker-up-sdk-local:
	@docker compose -f docker-compose.yml -f docker-compose.sdk-local.yml up -d --build backend
