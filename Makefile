.DEFAULT_GOAL := help

export PATH := $(HOME)/.local/bin:$(PATH)

APP_NAME       := dapr-go-poly
CURRENTTAG     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")

# === Tool Versions (pinned, Renovate-tracked) ===
# renovate: datasource=github-releases depName=nektos/act
ACT_VERSION      := 0.2.87
# renovate: datasource=github-releases depName=dapr/cli
DAPR_VERSION     := 1.17.0
# renovate: datasource=github-releases depName=hadolint/hadolint
HADOLINT_VERSION := 2.14.0
# renovate: datasource=go depName=golang.org/x/vuln/cmd/govulncheck
GOVULNCHECK_VERSION := 1.1.4
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_VERSION := 2.11.4
# renovate: datasource=github-releases depName=aquasecurity/trivy
TRIVY_VERSION    := 0.69.3
# renovate: datasource=github-releases depName=zricethezav/gitleaks
GITLEAKS_VERSION := 8.30.1
# renovate: datasource=github-releases depName=rhysd/actionlint
ACTIONLINT_VERSION := 1.7.12
# renovate: datasource=github-releases depName=koalaman/shellcheck
SHELLCHECK_VERSION := 0.11.0
# renovate: datasource=github-releases depName=kubernetes-sigs/kind
KIND_VERSION     := 0.25.0
# Pair with KIND_VERSION per KinD release notes.
KIND_NODE_IMAGE  := kindest/node:v1.31.2

# Node major version sourced from .nvmrc (mise reads it natively)
NODE_VERSION := $(shell cat .nvmrc 2>/dev/null || echo 22)

# === Project Paths ===
SOLUTION       := dapr-go-poly.slnx
GO_SERVICES    := basket-service onboarding
DOTNET_SERVICES := order-service product-service

# === Docker ===
export KO_DOCKER_REPO := docker.io/andriykalashnykov

#help: @ List available tasks
help:
	@echo "Usage: make COMMAND"
	@echo "Commands :"
	@grep -E '[a-zA-Z\.\-]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[32m%-20s\033[0m - %s\n", $$1, $$2}'

#deps: @ Verify required tools (auto-installs mise locally; idempotent)
deps:
	@# Bootstrap mise locally so contributors converge on the pinned Go
	@# version from .mise.toml. CI uses actions/setup-go directly — skip there.
	@if [ -z "$$CI" ] && ! command -v mise >/dev/null 2>&1; then \
		echo "Installing mise (no root required, installs to ~/.local/bin)..."; \
		curl -fsSL https://mise.run | sh; \
		echo ""; \
		echo "mise installed. Activate it in your shell, then re-run 'make deps':"; \
		echo '  bash: echo '\''eval "$$(~/.local/bin/mise activate bash)"'\'' >> ~/.bashrc'; \
		echo '  zsh:  echo '\''eval "$$(~/.local/bin/mise activate zsh)"'\''  >> ~/.zshrc'; \
		exit 0; \
	fi
	@if [ -z "$$CI" ] && command -v mise >/dev/null 2>&1; then \
		mise install --yes >/dev/null; \
	fi
	@command -v go >/dev/null 2>&1 || { echo "Error: Go required. See https://go.dev/dl/"; exit 1; }
	@command -v dotnet >/dev/null 2>&1 || { echo "Error: .NET SDK required. See https://dotnet.microsoft.com/download"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker required. See https://docs.docker.com/get-docker/"; exit 1; }
	@command -v dapr >/dev/null 2>&1 || echo "Note: Dapr CLI $(DAPR_VERSION) not installed (optional). See https://docs.dapr.io/getting-started/install-dapr-cli/"

#deps-act: @ Install act for local CI (idempotent, user-local)
deps-act: deps
	@command -v act >/dev/null 2>&1 || { echo "Installing act $(ACT_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL https://raw.githubusercontent.com/nektos/act/master/install.sh | bash -s -- -b $$HOME/.local/bin v$(ACT_VERSION); \
	}

#deps-hadolint: @ Install hadolint for Dockerfile linting (user-local)
deps-hadolint:
	@command -v hadolint >/dev/null 2>&1 || { echo "Installing hadolint $(HADOLINT_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL -o $$HOME/.local/bin/hadolint https://github.com/hadolint/hadolint/releases/download/v$(HADOLINT_VERSION)/hadolint-Linux-x86_64 && \
		chmod +x $$HOME/.local/bin/hadolint; \
	}

#deps-govulncheck: @ Install govulncheck for Go vulnerability scanning
deps-govulncheck: deps
	@command -v govulncheck >/dev/null 2>&1 || { echo "Installing govulncheck $(GOVULNCHECK_VERSION)..."; \
		GOBIN=$$HOME/.local/bin go install golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION); \
	}

#deps-golangci: @ Install golangci-lint for Go static analysis
deps-golangci: deps
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint $(GOLANGCI_VERSION)..."; \
		GOBIN=$$HOME/.local/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_VERSION); \
	}

#deps-trivy: @ Install Trivy for filesystem CVE scanning
deps-trivy:
	@command -v trivy >/dev/null 2>&1 || { echo "Installing trivy $(TRIVY_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b $$HOME/.local/bin v$(TRIVY_VERSION); \
	}

#deps-gitleaks: @ Install gitleaks for secret scanning
deps-gitleaks:
	@command -v gitleaks >/dev/null 2>&1 || { echo "Installing gitleaks $(GITLEAKS_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL -o /tmp/gitleaks.tar.gz https://github.com/zricethezav/gitleaks/releases/download/v$(GITLEAKS_VERSION)/gitleaks_$(GITLEAKS_VERSION)_linux_x64.tar.gz && \
		tar -xzf /tmp/gitleaks.tar.gz -C /tmp gitleaks && \
		install -m 755 /tmp/gitleaks $$HOME/.local/bin/gitleaks && \
		rm -f /tmp/gitleaks.tar.gz /tmp/gitleaks; \
	}

#deps-actionlint: @ Install actionlint for GitHub Actions workflow linting
deps-actionlint:
	@command -v actionlint >/dev/null 2>&1 || { echo "Installing actionlint $(ACTIONLINT_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL -o /tmp/actionlint.tar.gz https://github.com/rhysd/actionlint/releases/download/v$(ACTIONLINT_VERSION)/actionlint_$(ACTIONLINT_VERSION)_linux_amd64.tar.gz && \
		tar -xzf /tmp/actionlint.tar.gz -C /tmp actionlint && \
		install -m 755 /tmp/actionlint $$HOME/.local/bin/actionlint && \
		rm -f /tmp/actionlint.tar.gz /tmp/actionlint; \
	}

#deps-shellcheck: @ Install shellcheck (required by actionlint to lint workflow run: steps)
deps-shellcheck:
	@command -v shellcheck >/dev/null 2>&1 || { echo "Installing shellcheck $(SHELLCHECK_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL -o /tmp/shellcheck.tar.xz https://github.com/koalaman/shellcheck/releases/download/v$(SHELLCHECK_VERSION)/shellcheck-v$(SHELLCHECK_VERSION).linux.x86_64.tar.xz && \
		tar -xJf /tmp/shellcheck.tar.xz -C /tmp && \
		install -m 755 /tmp/shellcheck-v$(SHELLCHECK_VERSION)/shellcheck $$HOME/.local/bin/shellcheck && \
		rm -rf /tmp/shellcheck.tar.xz /tmp/shellcheck-v$(SHELLCHECK_VERSION); \
	}

#deps-mise: @ Install mise (user-local, no root required)
deps-mise:
	@command -v mise >/dev/null 2>&1 || { echo "Installing mise to $$HOME/.local/bin..."; \
		curl -fsSL https://mise.run | sh; \
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

#test: @ Run unit tests (Go + .NET, fast, no Docker)
test: deps
	@for svc in $(GO_SERVICES); do \
		echo "Testing $$svc..."; \
		(cd $$svc && go test -race ./...) || exit 1; \
	done

#integration-test: @ Run integration tests (Testcontainers for Postgres/RabbitMQ; requires Docker)
integration-test: deps
	@echo "Running .NET integration tests..."
	@dotnet test $(SOLUTION) --filter "Category=Integration" -c Release --nologo || exit 1
	@echo "Running Go integration tests (sidecar-dependent tests skip if Dapr not reachable)..."
	@for svc in $(GO_SERVICES); do \
		echo "Integration testing $$svc..."; \
		(cd $$svc && go test -tags=integration -race -v ./...) || exit 1; \
	done

#deps-kind: @ Install kind (Kubernetes-in-Docker) for K8s e2e
deps-kind:
	@command -v kind >/dev/null 2>&1 || { echo "Installing kind $(KIND_VERSION) to $$HOME/.local/bin..."; \
		mkdir -p $$HOME/.local/bin; \
		curl -sSfL -o $$HOME/.local/bin/kind https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-linux-amd64 && \
		chmod +x $$HOME/.local/bin/kind; \
	}

#kind-up: @ Create a KinD cluster and install Dapr + MetalLB
kind-up: deps-kind
	@command -v kubectl >/dev/null 2>&1 || { echo "Error: kubectl required. See https://kubernetes.io/docs/tasks/tools/"; exit 1; }
	@command -v dapr >/dev/null 2>&1 || { echo "Error: Dapr CLI required. See https://docs.dapr.io/getting-started/install-dapr-cli/"; exit 1; }
	@kind create cluster --name $(APP_NAME) --image $(KIND_NODE_IMAGE) || true
	@kubectl cluster-info --context kind-$(APP_NAME)
	@dapr init -k --runtime-version 1.17.3 --wait

#kind-down: @ Tear down the KinD cluster
kind-down:
	@kind delete cluster --name $(APP_NAME) || true

#e2e-kind: @ K8s e2e (KinD + Dapr + MetalLB) — scaffolding; see e2e/k8s/README.md
e2e-kind: kind-up
	@echo ""
	@echo "=== e2e-kind scaffolding ==="
	@echo ""
	@echo "KinD cluster up and Dapr installed. Remaining work to reach"
	@echo "a runnable K8s e2e (intentionally left as a TODO):"
	@echo ""
	@echo "  1. Add Deployment + Service manifests for product/order-service"
	@echo "     with Dapr sidecar injection annotations under e2e/k8s/."
	@echo "  2. Add MetalLB for LoadBalancer IPs (so curl can reach services"
	@echo "     from the host)."
	@echo "  3. Add Dapr Components (statestore/pubsub) pointing at in-cluster"
	@echo "     Redis (or the existing .iac/dapr/local/ components migrated)."
	@echo "  4. Apply manifests: kubectl apply -f e2e/k8s/"
	@echo "  5. Run curl-based assertions similar to e2e/e2e-test.sh against"
	@echo "     the LoadBalancer IP."
	@echo ""
	@echo "For now, Docker Compose e2e (make e2e) remains the authoritative"
	@echo "e2e path. KinD is additive — for validating K8s-specific concerns"
	@echo "(manifests, sidecar injector, service discovery, readiness probes)."
	@echo ""
	@echo "Run 'make kind-down' to tear down the cluster when done."

#e2e: @ Run end-to-end tests via Docker Compose (full stack, self-contained)
e2e: deps
	@echo "Starting full stack via e2e compose file..."
	@docker compose -f e2e/docker-compose.e2e.yml up -d --wait --build
	@echo "Running e2e test script..."
	@./e2e/e2e-test.sh || { \
		echo "E2E failed — capturing logs and tearing down stack..."; \
		docker compose -f e2e/docker-compose.e2e.yml logs --tail=100; \
		docker compose -f e2e/docker-compose.e2e.yml down --remove-orphans --volumes; \
		exit 1; \
	}
	@echo "E2E passed — tearing down stack..."
	@docker compose -f e2e/docker-compose.e2e.yml down --remove-orphans --volumes

#lint: @ Run linters (golangci-lint + dotnet format --verify + hadolint)
lint: deps deps-hadolint deps-golangci
	@for svc in $(GO_SERVICES); do \
		echo "Linting $$svc..."; \
		(cd $$svc && golangci-lint run ./...) || exit 1; \
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

#lint-ci: @ Lint GitHub Actions workflows (actionlint + shellcheck)
lint-ci: deps-actionlint deps-shellcheck
	@actionlint

#trivy-fs: @ Scan filesystem for vulnerabilities, secrets, misconfigurations
trivy-fs: deps-trivy
	@trivy fs --scanners vuln,secret,misconfig --severity CRITICAL,HIGH --exit-code 1 .

#secrets: @ Scan git history for leaked secrets
secrets: deps-gitleaks
	@gitleaks detect --source . --verbose --redact --no-banner

#vulncheck: @ Run vulnerability scanners (Go + .NET)
vulncheck: deps deps-govulncheck
	@for svc in $(GO_SERVICES); do \
		echo "Vuln-checking $$svc..."; \
		(cd $$svc && govulncheck ./...) || exit 1; \
	done
	@for svc in $(DOTNET_SERVICES); do \
		echo "Vuln-checking $$svc..."; \
		(cd $$svc && dotnet list package --vulnerable --include-transitive 2>&1 | tee /tmp/vuln-$$svc.log; \
			grep -qE '(>|has the following vulnerable)' /tmp/vuln-$$svc.log && { echo "Vulnerable packages found in $$svc"; exit 1; } || true) || exit 1; \
	done

#static-check: @ Composite quality gate (lint-ci + lint + vulncheck + secrets + trivy-fs + deps-prune-check)
static-check: lint-ci lint vulncheck secrets trivy-fs deps-prune-check

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
	@if ls *.sln *.slnx >/dev/null 2>&1; then \
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
	if ls *.sln *.slnx >/dev/null 2>&1; then \
		if dotnet build $(SOLUTION) -c Release --nologo 2>&1 | grep -q 'NU1510'; then \
			echo "ERROR: .NET has redundant PackageReferences (NU1510). Run 'make deps-prune'."; FOUND=1; \
		fi; \
	fi; \
	if [ $$FOUND -ne 0 ]; then exit 1; fi; \
	echo "No prunable dependencies found."

#image-build: @ Build Docker images
image-build: build
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
ci: deps clean format static-check test integration-test build
	@echo "Local CI pipeline passed."

#ci-run: @ Run GitHub Actions workflow locally using act
ci-run: deps-act
	@ACT_PORT=$$(shuf -i 40000-59999 -n 1); \
	ARTIFACTS=$$(mktemp -d -t act-artifacts.XXXXXX); \
	act push --container-architecture linux/amd64 \
		--artifact-server-port $$ACT_PORT \
		--artifact-server-path $$ARTIFACTS \
		--var ACT=true

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

#renovate-bootstrap: @ Install mise + Node for Renovate
renovate-bootstrap: deps-mise
	@command -v node >/dev/null 2>&1 || { \
		echo "Installing Node $(NODE_VERSION) via mise..."; \
		mise install node@$(NODE_VERSION); \
	}

#renovate-validate: @ Validate Renovate configuration
renovate-validate: renovate-bootstrap
	@if [ -n "$$GH_ACCESS_TOKEN" ]; then \
		GITHUB_COM_TOKEN=$$GH_ACCESS_TOKEN npx --yes renovate --platform=local; \
	else \
		echo "Warning: GH_ACCESS_TOKEN not set, some dependency lookups may fail"; \
		npx --yes renovate --platform=local; \
	fi

.PHONY: help deps deps-act deps-hadolint deps-govulncheck deps-mise deps-kind deps-golangci deps-trivy deps-gitleaks deps-actionlint deps-shellcheck clean format build test integration-test e2e e2e-kind kind-up kind-down lint lint-ci trivy-fs secrets vulncheck static-check update \
	deps-prune deps-prune-check \
	image-build run compose-down compose-up \
	ci ci-run release \
	renovate-bootstrap renovate-validate
