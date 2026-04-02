.DEFAULT_GOAL := help

APP_NAME       := dapr-go-poly
CURRENTTAG     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")

# === Tool Versions (pinned) ===
ACT_VERSION      := 0.2.87
HADOLINT_VERSION := 2.12.0

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

#deps: @ Install required tools (idempotent)
deps:
	@command -v go >/dev/null 2>&1 || { echo "Error: Go required. See https://go.dev/dl/"; exit 1; }
	@command -v dotnet >/dev/null 2>&1 || { echo "Error: .NET SDK required. See https://dotnet.microsoft.com/download"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker required. See https://docs.docker.com/get-docker/"; exit 1; }

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

#build: @ Build all services
build: deps
	@for svc in $(GO_SERVICES); do \
		echo "Building $$svc..."; \
		(cd $$svc && go mod download && go build -o main main.go) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Building $$svc..."; \
		(cd $$svc && dotnet build $$svc.csproj) || exit 1; \
	done

#test: @ Run tests
test: deps
	@for svc in $(GO_SERVICES); do \
		echo "Testing $$svc..."; \
		(cd $$svc && go test ./...) || exit 1; \
	done

#lint: @ Run linters
lint: deps deps-hadolint
	@for svc in $(GO_SERVICES); do \
		echo "Vetting $$svc..."; \
		(cd $$svc && go vet ./...) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Formatting $$svc..."; \
		(cd $$svc && dotnet format $$svc.csproj --verify-no-changes) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		if [ -f $$svc/Dockerfile ]; then \
			echo "Linting $$svc/Dockerfile..."; \
			hadolint $$svc/Dockerfile; \
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

#image-build: @ Build Docker images
image-build: deps
	@cd order-service && docker buildx build --load -t andriykalashnykov/dapr-go-poly-order-service:latest .
	@cd product-service && docker buildx build --load -t andriykalashnykov/dapr-go-poly-product-service:latest .

#run: @ Run order-service via Dapr
run: deps
	@cd order-service && dapr run --config ../.dapr/config.yaml --app-id product-service --app-port 8080 --placement-host-address host.docker.internal:50006 --dapr-http-port 3500

#compose-down: @ Stop and remove Docker Compose services
compose-down:
	@docker compose down --remove-orphans --volumes

#compose-up: @ Start Docker Compose services (rebuild)
compose-up: compose-down
	@docker compose up --build

#ci: @ Run full local CI pipeline
ci: deps clean build lint test
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
		git add -A && \
		git commit -a -s -m "Cut $$newtag release" && \
		git tag $$newtag && \
		git push origin $$newtag && \
		git push && \
		echo "Done."'

# === Renovate ===
NVM_VERSION := 0.40.4

#renovate-bootstrap: @ Install nvm and npm for Renovate
renovate-bootstrap:
	@command -v node >/dev/null 2>&1 || { \
		echo "Installing nvm $(NVM_VERSION)..."; \
		curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v$(NVM_VERSION)/install.sh | bash; \
		export NVM_DIR="$$HOME/.nvm"; \
		[ -s "$$NVM_DIR/nvm.sh" ] && . "$$NVM_DIR/nvm.sh"; \
		nvm install --lts; \
	}

#renovate-validate: @ Validate Renovate configuration
renovate-validate: renovate-bootstrap
	@npx --yes renovate --platform=local

.PHONY: help deps deps-act deps-hadolint clean build test lint update \
	image-build run compose-down compose-up \
	ci ci-run release \
	renovate-bootstrap renovate-validate
