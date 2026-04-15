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
make integration-test  # Integration tests (.NET: TUnit via `dotnet run` + Testcontainers Postgres/RabbitMQ; Go: //go:build integration)
make e2e               # End-to-end tests via Docker Compose (self-contained e2e/docker-compose.e2e.yml)
make lint              # Run linters (golangci-lint + dotnet format --verify + hadolint)
make lint-ci           # Lint GitHub Actions workflows (actionlint + shellcheck)
make vulncheck         # Run vulnerability scanners (govulncheck + dotnet list package --vulnerable)
make trivy-fs          # Trivy filesystem scan (CVEs + secrets + misconfigs)
make secrets           # Gitleaks scan for leaked secrets
make diagrams          # Render C4-PlantUML sources under docs/diagrams/ to PNG
make diagrams-check    # Verify committed PNGs match .puml sources (CI drift gate)
make static-check      # Composite quality gate (lint-ci + lint + vulncheck + secrets + trivy-fs + diagrams-check + deps-prune-check)
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
| `PLANTUML_VERSION` | `1.2026.2` | Pinned PlantUML Docker image for `make diagrams` (C4 rendering) |
| `NODE_VERSION` | `$(shell cat .nvmrc)` | Node major version sourced from `.nvmrc` (mise reads natively) |
| `GO_SERVICES` | `basket-service onboarding` | Go service directories |
| `DOTNET_SERVICES` | `order-service product-service` | .NET service directories |
| `DOTNET_TEST_PROJECTS` | `product-service.IntegrationTests order-service.IntegrationTests` | TUnit test projects run via `dotnet run --project` |

Version manager: **mise** (installed via `make deps-mise` / `renovate-bootstrap`). NVM has been removed per the portfolio-wide Version Manager Policy. User-local tool installs (`act`, `hadolint`, `govulncheck`) target `$HOME/.local/bin` — no `sudo` required.

## Project Structure

```text
basket-service/                      # Go service (Fiber; routes WIP)
onboarding/                          # Go service (Dapr Workflow: async POST 202 + GET status + approve/deny)
order-service/                       # .NET 10 service (EF Core + RabbitMQ consumer)
product-service/                     # .NET 10 service (EF Core / Postgres)
order-service.IntegrationTests/      # TUnit + Testcontainers (Postgres + RabbitMQ)
product-service.IntegrationTests/    # TUnit + Testcontainers (Postgres)
e2e/
  docker-compose.e2e.yml             # Self-contained e2e stack (incl. Redis + Dapr control plane for onboarding workflow)
  dapr/components/statestore.yaml    # Dapr state store component (Redis, actorStateStore=true)
  e2e-test.sh                        # curl-based e2e assertions (21 assertions covering all 3 services + onboarding async workflow lifecycle)
  k8s/README.md                      # KinD e2e scaffolding notes
docs/diagrams/                       # C4-PlantUML sources + rendered PNGs
.mise.toml                           # Project-local Go pin (matches go.mod)
.golangci.yml                        # Go lint config (gocritic/gosec/etc.)
.trivyignore                         # Trivy suppressions (with justification)
dapr-go-poly.slnx                    # .NET solution file (modern XML format)
docker-compose.yml                   # Base compose (Dapr + Postgres + RabbitMQ + services)
global.json                          # .NET SDK version pin
renovate.json                        # Renovate dependency update configuration
LICENSE                              # MIT
```

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push to `main`, tags `v*`, pull requests, and `workflow_call` (paths-ignore: `**/*.md`, `docs/**`, `LICENSE`, `.gitignore`):
- **static-check** job: Checkout, Set up Go, Set up .NET, `make static-check` (lint-ci + lint + vulncheck + secrets + trivy-fs + diagrams-check + deps-prune-check)
- **build** job (needs static-check): Checkout, Set up Go, Set up .NET, `make build`
- **test** job (needs static-check): Checkout, Set up Go, Set up .NET, `make test`
- **integration-test** job (needs static-check; skipped under act via `vars.ACT`): Set up Go/.NET, `make integration-test` (TUnit + Testcontainers). No Dapr sidecar — onboarding's Dapr workflow lifecycle is exercised by the e2e job instead
- **e2e** job (needs build + test; skipped under act): `make e2e` — brings up 9 containers (Dapr control plane + Redis state store + postgres + rabbitmq + 3 app services + onboarding sidecar), runs 21 curl-based assertions including the full onboarding async workflow lifecycle (POST 202 → approve/deny → poll GET status), captures compose logs on failure
- **docker** job (needs static-check + build + test): Checkout, Set up .NET, `make image-build` (step-level `if` gates on tag `v*`)
- **ci-pass** job (aggregator, `if: always()`): Verifies all upstream jobs passed (treats `skipped` as pass) — use as branch-protection required check

A cleanup workflow (`.github/workflows/cleanup-runs.yml`) removes old workflow runs weekly (also supports `workflow_dispatch`).

## Development Conventions

- Go services use standard `go build` / `go test` toolchain; `golangci-lint` config in `.golangci.yml` enables gocritic/gosec/errorlint/bodyclose/noctx/misspell
- .NET services use `dotnet build` / `dotnet format` with `TreatWarningsAsErrors` enabled
- **.NET tests use TUnit** (portfolio-wide hard requirement per `rules/dotnet/testing.md`). Run via `dotnet run --project <TestProject>` (native Microsoft.Testing.Platform entry point), NOT `dotnet test`. Fixtures use `[ClassDataSource<T>(Shared = SharedType.PerClass)]` + `IAsyncInitializer`/`IAsyncDisposable`. Assertions are async — always `await Assert.That(...)`. Mocking: FakeItEasy
- C4 architecture diagrams (`docs/diagrams/*.puml`) are rendered to PNG via the pinned `plantuml/plantuml` Docker image; `make diagrams-check` is wired into `static-check` so stale committed output fails CI
- Docker images built with `docker buildx`
- Dockerfiles linted with hadolint via `make lint`
- Dapr CLI version pinned as `DAPR_VERSION` in Makefile — always use a specific version, never `latest`
- All Makefile `_VERSION` constants carry `# renovate:` inline comments; a single generic `customManagers` regex in `renovate.json` tracks them all — no per-tool config drift
- User-local tool installs (`act`, `hadolint`, `govulncheck`, `trivy`, `gitleaks`, `actionlint`, `shellcheck`, `kind`) target `$HOME/.local/bin`; `export PATH` at the top of the Makefile makes them usable in the same `make` invocation

## Upgrade Backlog

Deferred upgrade items from `/upgrade-analysis` (last run 2026-04-15). Resolve or prune on next analysis.

**Wave 2 — minor (in progress):**
- [ ] `FakeItEasy` 8.3.0 → 9.0.1 (major — review release notes)
- [ ] `kind` 0.25.0 → 0.31.0 + `KIND_NODE_IMAGE` bump to matching v1.33.x
- [ ] **RabbitMQ 3.x is EOL (2024-12-31)** — bump `rabbitmq:3-management-alpine` → `rabbitmq:4-management-alpine`; verify `OrdersConsumer` reconnect logic against RabbitMQ 4

**Wave 3 — major bases (quarterly):**
- [ ] `redis:7-alpine` → `redis:8-alpine` (verify Dapr state store compatibility)
- [ ] `postgres:17-alpine` → `postgres:18-alpine` (rehearse EF migrations)
- [ ] `.nvmrc` 22 → 24 LTS

## Skills

Use the following skills when working on related files:

| File(s) | Skill |
|---------|-------|
| `Makefile` | `/makefile` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `.github/workflows/*.{yml,yaml}` | `/ci-workflow` |

When spawning subagents, always pass conventions from the respective skill into the agent's prompt.
