# CLAUDE.md

## Project Overview

**dapr-go-poly** is a polyglot microservices project using [Dapr](https://dapr.io/) with Go and .NET services, orchestrated via Docker Compose.

- **Repository**: `AndriyKalashnykov/dapr-go-poly`
- **Go services**: `basket-service`, `onboarding` (Go 1.26+)
- **DotNet services**: `order-service`, `product-service` (.NET 10)

## Build & Test

```bash
make deps          # Verify required tools (go, dotnet, docker)
make build             # Build all services
make test              # Run unit tests (Go + .NET, fast, no Docker)
make integration-test  # Integration tests (Testcontainers Postgres + RabbitMQ; requires Docker)
make e2e               # End-to-end tests via Docker Compose (self-contained e2e/docker-compose.e2e.yml)
make lint              # Run linters (golangci-lint + dotnet format --verify + hadolint)
make lint-ci           # Lint GitHub Actions workflows (actionlint + shellcheck)
make vulncheck         # Run vulnerability scanners (govulncheck + dotnet list package --vulnerable)
make trivy-fs          # Trivy filesystem scan (CVEs + secrets + misconfigs)
make secrets           # Gitleaks scan for leaked secrets
make static-check      # Composite quality gate (lint-ci + lint + vulncheck + secrets + trivy-fs + deps-prune-check)
make format        # Auto-fix formatting (Go + .NET)
make clean         # Remove build artifacts
make ci            # Full local CI pipeline (format, static-check, test, build)
make ci-run        # Run GitHub Actions locally via act (randomized artifact port/path)
```

## Key Variables

| Variable | Value | Purpose |
|----------|-------|---------|
| `ACT_VERSION` | `0.2.87` | Pinned act version for local CI |
| `DAPR_VERSION` | `1.17.0` | Pinned Dapr CLI version |
| `HADOLINT_VERSION` | `2.14.0` | Pinned hadolint version for Dockerfile linting |
| `GOVULNCHECK_VERSION` | `1.1.4` | Pinned govulncheck version for Go CVE scanning |
| `GOLANGCI_VERSION` | `2.11.4` | Pinned golangci-lint version (gocritic/gosec/errorlint/bodyclose/noctx) |
| `TRIVY_VERSION` | `0.69.3` | Pinned Trivy version for filesystem vulnerability scanning |
| `GITLEAKS_VERSION` | `8.30.1` | Pinned gitleaks version for secret scanning |
| `ACTIONLINT_VERSION` | `1.7.12` | Pinned actionlint version for workflow linting |
| `SHELLCHECK_VERSION` | `0.11.0` | Pinned shellcheck (required by actionlint to lint `run:` blocks) |
| `KIND_VERSION` | `0.25.0` | Pinned KinD version for `make e2e-kind` scaffolding |
| `KIND_NODE_IMAGE` | `kindest/node:v1.31.2` | KinD node image paired with `KIND_VERSION` |
| `NODE_VERSION` | `$(shell cat .nvmrc)` | Node major version sourced from `.nvmrc` (mise reads natively) |
| `GO_SERVICES` | `basket-service onboarding` | Go service directories |
| `DOTNET_SERVICES` | `order-service product-service` | .NET service directories |

Version manager: **mise** (installed via `make deps-mise` / `renovate-bootstrap`). NVM has been removed per the portfolio-wide Version Manager Policy. User-local tool installs (`act`, `hadolint`, `govulncheck`) target `$HOME/.local/bin` — no `sudo` required.

## Project Structure

```
basket-service/    # Go service
onboarding/        # Go service
order-service/     # .NET service (with Dockerfile)
product-service/   # .NET service (with Dockerfile)
dapr-go-poly.slnx  # .NET solution file (modern XML format)
docker-compose.yml # Local orchestration
global.json        # .NET SDK version pin
renovate.json      # Renovate dependency update configuration
```

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push to `main`, tags `v*`, pull requests, and `workflow_call` (paths-ignore: `**/*.md`, `docs/**`, `LICENSE`, `.gitignore`):
- **static-check** job: Checkout, Set up Go, Set up .NET, `make static-check`
- **build** job (needs static-check): Checkout, Set up Go, Set up .NET, `make build`
- **test** job (needs static-check): Checkout, Set up Go, Set up .NET, `make test`
- **docker** job (needs static-check + build + test): Checkout, Set up .NET, `make image-build` (step-level `if` gates on tag `v*`)
- **ci-pass** job (aggregator, `if: always()`): Verifies all upstream jobs passed — use as branch-protection required check

A cleanup workflow (`.github/workflows/cleanup-runs.yml`) removes old workflow runs weekly (also supports `workflow_dispatch`).

## Development Conventions

- Go services use standard `go build` / `go test` toolchain
- .NET services use `dotnet build` / `dotnet format` with `TreatWarningsAsErrors` enabled
- Docker images built with `docker buildx`
- Dockerfiles linted with hadolint via `make lint`
- Dapr CLI version pinned as `DAPR_VERSION` in Makefile — always use a specific version, never `latest`
- All Makefile `_VERSION` constants carry `# renovate:` inline comments; a single generic `customManagers` regex in `renovate.json` tracks them all — no per-tool config drift
- User-local tool installs (`act`, `hadolint`, `govulncheck`) target `$HOME/.local/bin`; `export PATH` at the top of the Makefile makes them usable in the same `make` invocation

## Skills

Use the following skills when working on related files:

| File(s) | Skill |
|---------|-------|
| `Makefile` | `/makefile` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `.github/workflows/*.{yml,yaml}` | `/ci-workflow` |

When spawning subagents, always pass conventions from the respective skill into the agent's prompt.

## Monitoring Checklist

Items to check on next upgrade analysis:

- [ ] **grpc GO-2026-4762** — `google.golang.org/grpc` v1.79.2 has auth bypass fixed in v1.79.3. Indirect dep via `dapr/go-sdk`. Not called by our code. Will resolve when Dapr Go SDK bumps grpc. Track: `dapr/go-sdk` releases.
