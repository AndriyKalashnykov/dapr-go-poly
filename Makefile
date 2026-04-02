.DEFAULT_GOAL := help

APP_NAME       := dapr-go-poly
CURRENTTAG     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")

# === Tool Versions (pinned) ===
ACT_VERSION      := 0.2.87
DAPR_VERSION     := 1.17.0
HADOLINT_VERSION := 2.14.0
NVM_VERSION      := 0.40.4
NODE_VERSION     := 22

# === Project Paths ===
SOLUTION       := dapr-go-poly.sln
GO_SERVICES    := basket-service onboarding
DOTNET_SERVICES := order-service product-service

# === Docker ===
export KO_DOCKER_REPO := docker.io/andriykalashnykov

#help: @ List available tasks
help:
	@echo "Usage: make COMMAND"
	@echo "Commands :"
	@grep -E '[a-zA-Z\.\-]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[32m%-20s\033[0m - %s\n", $$1, $$2}'

#deps: @ Verify required tools (idempotent)
deps:
	@command -v go >/dev/null 2>&1 || { echo "Error: Go required. See https://go.dev/dl/"; exit 1; }
	@command -v dotnet >/dev/null 2>&1 || { echo "Error: .NET SDK required. See https://dotnet.microsoft.com/download"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker required. See https://docs.docker.com/get-docker/"; exit 1; }
	@command -v dapr >/dev/null 2>&1 || { echo "Warning: Dapr CLI $(DAPR_VERSION) not found. See https://docs.dapr.io/getting-started/install-dapr-cli/"; }

#deps-act: @ Install act for local CI (idempotent)
deps-act: deps
	@command -v act >/dev/null 2>&1 || { echo "Installing act $(ACT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash -s -- -b /usr/local/bin v$(ACT_VERSION); \
	}

#deps-hadolint: @ Install hadolint for Dockerfile linting
deps-hadolint:
	@command -v hadolint >/dev/null 2>&1 || { echo "Installing hadolint $(HADOLINT_VERSION)..."; \
		curl -sSfL -o /tmp/hadolint https://github.com/hadolint/hadolint/releases/download/v$(HADOLINT_VERSION)/hadolint-Linux-x86_64 && \
		install -m 755 /tmp/hadolint /usr/local/bin/hadolint && \
		rm -f /tmp/hadolint; \
	}

#clean: @ Remove build artifacts
clean:
	@for svc in $(GO_SERVICES); do \
		rm -f $$svc/main; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		(cd $$svc && dotnet clean $$svc.csproj -c Release --nologo -v q); \
	done
	@find . -type d \( -name bin -o -name obj \) -exec rm -rf {} + 2>/dev/null || true

#format: @ Auto-fix formatting (Go + .NET)
format: deps
	@for svc in $(GO_SERVICES); do \
		echo "Formatting $$svc..."; \
		gofmt -w $$svc/; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Formatting $$svc..."; \
		(cd $$svc && dotnet format $$svc.csproj) || exit 1; \
	done

#build: @ Build all services
build: deps
	@for svc in $(GO_SERVICES); do \
		echo "Building $$svc..."; \
		(cd $$svc && go mod download && go build -o main main.go) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Building $$svc..."; \
		(cd $$svc && dotnet build $$svc.csproj --nologo) || exit 1; \
	done

#test: @ Run tests
test: deps
	@for svc in $(GO_SERVICES); do \
		echo "Testing $$svc..."; \
		(cd $$svc && go test -race ./...) || exit 1; \
	done

#lint: @ Run linters (go vet + dotnet format --verify + hadolint)
lint: deps deps-hadolint
	@for svc in $(GO_SERVICES); do \
		echo "Vetting $$svc..."; \
		(cd $$svc && go vet ./...) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Checking format $$svc..."; \
		(cd $$svc && dotnet format $$svc.csproj --verify-no-changes) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		if [ -f $$svc/Dockerfile ]; then \
			echo "Linting $$svc/Dockerfile..."; \
			hadolint $$svc/Dockerfile || exit 1; \
		fi; \
	done

#update: @ Update all dependencies to latest versions
update: deps
	@for svc in $(GO_SERVICES); do \
		echo "Updating $$svc..."; \
		(cd $$svc && go get -u ./... && go mod tidy) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Updating $$svc..."; \
		(cd $$svc && dotnet list package --outdated | grep -o '> \S*' | grep '[^> ]*' -o | xargs --no-run-if-empty -L 1 dotnet add package) || exit 1; \
	done

#deps-prune: @ Remove unused and redundant dependencies
deps-prune: deps
	@echo "=== Dependency Pruning ==="
	@for svc in $(GO_SERVICES); do \
		echo "--- Go: tidying $$svc ---"; \
		(cd $$svc && go mod tidy) || exit 1; \
	done
	@if ls *.sln >/dev/null 2>&1; then \
		echo "--- .NET: checking for redundant PackageReferences ---"; \
		dotnet build $(SOLUTION) -c Release --nologo 2>&1 | grep 'NU1510' && echo "  ^^^ Remove these PackageReferences from .csproj files" || echo "  No redundant .NET packages found."; \
	fi
	@echo "=== Pruning complete ==="

#deps-prune-check: @ Verify no prunable dependencies (CI gate)
deps-prune-check: deps
	@FOUND=0; \
	for svc in $(GO_SERVICES); do \
		(cd $$svc && go mod tidy); \
		if ! (cd $$svc && git diff --exit-code go.mod go.sum >/dev/null 2>&1); then \
			echo "ERROR: $$svc go.mod/go.sum not tidy. Run 'make deps-prune'."; FOUND=1; \
			(cd $$svc && git checkout go.mod go.sum); \
		fi; \
	done; \
	if ls *.sln >/dev/null 2>&1; then \
		if dotnet build $(SOLUTION) -c Release --nologo 2>&1 | grep -q 'NU1510'; then \
			echo "ERROR: .NET has redundant PackageReferences (NU1510). Run 'make deps-prune'."; FOUND=1; \
		fi; \
	fi; \
	if [ $$FOUND -ne 0 ]; then exit 1; fi; \
	echo "No prunable dependencies found."

#image-build: @ Build Docker images
image-build: deps
	@for svc in $(DOTNET_SERVICES); do \
		if [ -f $$svc/Dockerfile ]; then \
			echo "Building image for $$svc..."; \
			(cd $$svc && docker buildx build --load -t andriykalashnykov/$(APP_NAME)-$$svc:$(CURRENTTAG) .) || exit 1; \
		fi; \
	done

#run: @ Run order-service via Dapr
run: deps
	@cd order-service && dapr run --app-id product-service --app-port 8080 --placement-host-address host.docker.internal:50006 --dapr-http-port 3500

#compose-down: @ Stop and remove Docker Compose services
compose-down:
	@docker compose down --remove-orphans --volumes

#compose-up: @ Start Docker Compose services (rebuild)
compose-up: compose-down
	@docker compose up --build

#ci: @ Run full local CI pipeline
ci: deps clean format lint test build
	@echo "Local CI pipeline passed."

#ci-run: @ Run GitHub Actions workflow locally using act
ci-run: deps-act
	@act push --container-architecture linux/amd64

#release: @ Create and push a new tag
release:
	@bash -c 'read -p "New tag (current: $(CURRENTTAG)): " newtag && \
		echo "$$newtag" | grep -qE "^v[0-9]+\.[0-9]+\.[0-9]+$$" || { echo "Error: Tag must match vN.N.N"; exit 1; } && \
		echo -n "Create and push $$newtag? [y/N] " && read ans && [ "$${ans:-N}" = y ] && \
		echo $$newtag > ./version.txt && \
		git add version.txt && \
		git commit -s -m "Cut $$newtag release" && \
		git tag $$newtag && \
		git push origin $$newtag && \
		git push && \
		echo "Done."'

#renovate-bootstrap: @ Install nvm and npm for Renovate
renovate-bootstrap:
	@command -v node >/dev/null 2>&1 || { \
		echo "Installing nvm $(NVM_VERSION)..."; \
		curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v$(NVM_VERSION)/install.sh | bash; \
		export NVM_DIR="$$HOME/.nvm"; \
		[ -s "$$NVM_DIR/nvm.sh" ] && . "$$NVM_DIR/nvm.sh"; \
		nvm install $(NODE_VERSION); \
	}

#renovate-validate: @ Validate Renovate configuration
renovate-validate: renovate-bootstrap
	@npx --yes renovate --platform=local

.PHONY: help deps deps-act deps-hadolint clean format build test lint update \
	deps-prune deps-prune-check \
	image-build run compose-down compose-up \
	ci ci-run release \
	renovate-bootstrap renovate-validate
