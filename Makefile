.PHONY: swagger license license-check build-embedded build-test cross-build code-check format canary

VERSION ?= dev
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MODULE := $(shell go list -m)

swagger:
	scripts/swagger.sh

license:
	scripts/update_go_license.sh

license-check:
	scripts/update_go_license.sh --check

format:
	@echo "==> Formatting backend Go source..."
	gofmt -w $$(find . -type f -name '*.go' -not -path './.git/*' -not -path './frontend/*')
	@echo "==> Formatting frontend source..."
	cd frontend && pnpm format

build-embedded:
	@echo "==> Building embedded frontend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd frontend && \
		NEXT_PUBLIC_APP_VERSION="$(VERSION)" \
		NEXT_PUBLIC_APP_BUILD_DATE="$(BUILD_DATE)" \
		pnpm build:embed
	rm -rf backend/plugins/drivers/driver_http/dist
	cp -R frontend/out backend/plugins/drivers/driver_http/dist
	go build \
		-tags embed_frontend \
		-ldflags "-s -w -X '$(MODULE)/backend/pkg/buildinfo.Version=$(VERSION)' -X '$(MODULE)/backend/pkg/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o bin/wavelet \
		backend/main.go

code-check:
	@echo "==> Architecture guards..."
	@command -v rg >/dev/null 2>&1 || { echo 'error: rg (ripgrep) is required for architecture guards' >&2; exit 1; }
	@echo "  → core/ must not import gin, gorm, asynq..."
	@if rg -n '"github.com/gin-gonic/gin|"gorm.io/gorm|"github.com/hibiken/asynq' backend/core/ --glob '*.go' -g '!*contracts*' -g '!*_test.go' 2>/dev/null; then \
		echo 'error: backend/core/ must not import gin, gorm, or asynq' >&2; \
		exit 1; \
	fi
	@echo "  → core/contracts/ must not import plugins/..."
	@if rg -n 'plugins/' backend/core/contracts --glob '*.go' 2>/dev/null; then \
		echo 'error: backend/core/contracts/ must not import plugins/' >&2; \
		exit 1; \
	fi
	@echo "  → pkg/ must not import plugins/..."
	@if rg -n 'plugins/' backend/pkg --glob '*.go' -g '!*testhelper*' -g '!*_test.go' 2>/dev/null; then \
		echo 'error: backend/pkg/ must not import plugins/' >&2; \
		exit 1; \
	fi
	@echo "  → plugins/domain/ must not import other plugins/domain/..."
	@for d in backend/plugins/domain/*/; do \
		name=$$(basename $$d); \
		imports=$$(rg -n '"github.com/Rain-kl/Wavelet/backend/plugins/domain/' backend/plugins/domain/"$$name" -g '*.go' 2>/dev/null | rg -v "backend/plugins/domain/$$name/" | rg -v '_test.go' || true); \
		if [ -n "$$imports" ]; then \
			echo "error: backend/plugins/domain/$$name must not import other domain plugins" >&2; \
			echo "$$imports" >&2; \
			exit 1; \
			fi; \
	done
	@echo "  → Architecture guards PASS"
	golangci-lint run ./backend/...
	cd frontend && pnpm tsc --noEmit --jsx preserve && npx eslint . --max-warnings 0

build-backend:
	@echo "==> Building backend version=$(VERSION) build_date=$(BUILD_DATE)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/backend/pkg/buildinfo.Version=$(VERSION)' -X '$(MODULE)/backend/pkg/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o bin/wavelet \
		backend/main.go

build-frontend:
	@echo "==> Building frontend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd frontend && \
		NEXT_PUBLIC_APP_VERSION="$(VERSION)" \
		NEXT_PUBLIC_APP_BUILD_DATE="$(BUILD_DATE)" \
		pnpm build:embed

build-test:
	@echo "==> Running frontend and backend build tests in parallel..."
	@PIDS=""; \
	STATUS=0; \
	( cd frontend && pnpm build:embed 2>&1 | sed 's/^/[frontend] /' ) & PIDS="$$PIDS $$!"; \
	( go test ./... && go build -o /dev/null ./... 2>&1 | sed 's/^/[backend]  /' ) & PIDS="$$PIDS $$!"; \
	for PID in $$PIDS; do \
		wait $$PID || STATUS=1; \
	done; \
	if [ $$STATUS -eq 0 ]; then \
		echo "==> All build tests passed."; \
	else \
		echo "==> Build test FAILED." >&2; \
		exit 1; \
	fi

cross-build:
	@echo "==> Cross-compiling \
	$(if $(GOOS),$(GOOS),linux/darwin/windows) × \
	$(if $(GOARCH),$(GOARCH),amd64/arm64) \
	(version=$(or $(VERSION),dev))..."
	@mkdir -p bin
	docker build \
		--file docker/Dockerfile.cross \
		--target export \
		--build-arg VERSION=$(or $(VERSION),dev) \
		--build-arg BUILD_DATE="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')" \
		$(if $(GOOS),--build-arg TARGET_OS=$(GOOS)) \
		$(if $(GOARCH),--build-arg TARGET_ARCH=$(GOARCH)) \
		--output type=local,dest=./bin \
		.
	@echo "==> Done. Binaries written to ./bin/"
	@ls -lh bin/

dev-f:
	@echo "==> Starting frontend development server..."
	cd frontend && pnpm dev

dev-b:
	@echo "==> Starting backend development server..."
	go run main.go all

dev:
	@echo "==> Starting frontend and backend development servers in parallel..."
	@PIDS=""; \
	STATUS=0; \
	( cd frontend && pnpm dev 2>&1 | sed 's/^/[frontend] /' ) & PIDS="$$PIDS $$!"; \
	( go run main.go all 2>&1 | sed 's/^/[backend]  /' ) & PIDS="$$PIDS $$!"; \
	for PID in $$PIDS; do \
		wait $$PID || STATUS=1; \
	done; \
	if [ $$STATUS -eq 0 ]; then \
		echo "==> All development servers exited successfully."; \
	else \
		echo "==> Development servers exited with errors." >&2; \
		exit 1; \
	fi


# Merge the current local branch into canary, push canary, then restore it.
# Dirty worktrees are auto-stashed and restored after the operation.
canary:
	@set -e; \
	if ! git rev-parse --git-dir >/dev/null 2>&1; then \
		echo "Error: not a git repository"; exit 1; \
	fi; \
	orig=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$orig" = "HEAD" ]; then \
		echo "Error: detached HEAD; checkout a branch first"; exit 1; \
	fi; \
	if [ "$$orig" = "canary" ]; then \
		echo "Error: already on canary; checkout a source branch first"; exit 1; \
	fi; \
	stashed=0; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Working tree is dirty; stashing local changes..."; \
		git stash push -u -m "make canary auto-stash from $$orig"; \
		stashed=1; \
	fi; \
	cleanup() { \
		cur=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true); \
		if [ "$$cur" != "$$orig" ]; then \
			echo "Restoring branch $$orig..."; \
			git checkout -q "$$orig"; \
		fi; \
		if [ "$$stashed" = "1" ]; then \
			echo "Restoring stashed local changes..."; \
			git stash pop; \
		fi; \
	}; \
	trap cleanup EXIT; \
	echo "Merging $$orig -> canary..."; \
	git checkout canary; \
	git merge --no-edit "$$orig"; \
	git push -u origin canary; \
	echo "Done: $$orig merged into canary and pushed."
