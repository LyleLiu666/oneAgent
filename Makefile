.PHONY: build build-frontend sync-frontend build-backend serve doctor release clean

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

build-backend: sync-frontend
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
