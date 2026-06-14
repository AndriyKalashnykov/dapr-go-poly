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
make static-check      # Composite quality gate (check-go-alignment + lint-ci + lint + vulncheck + secrets + trivy-fs + diagrams-check + deps-prune-check)
make format        # Auto-fix formatting (Go + .NET)
make clean         # Remove build artifacts
make ci            # Full local CI pipeline (format, static-check, test, build)
make ci-run        # Run GitHub Actions locally via act (randomized artifact port/path)
```

## Key Variables

The CLI toolchain — `act`, `dapr` (CLI), `hadolint`, `govulncheck`, `golangci-lint`, `trivy`, `gitleaks`, `actionlint`, `shellcheck`, `kind`, plus `go` — is pinned in **`.mise.toml`** (the single source of truth) and installed by `mise install` (`make deps` locally; `jdx/mise-action` in CI). Only the values mise does not manage remain as Makefile constants:

| Variable | Value | Purpose |
|----------|-------|---------|
| `DAPR_RUNTIME_VERSION` | `1.17.4` | Pinned Dapr runtime version for `make kind-up` (`dapr init -k --runtime-version`) |
| `KIND_NODE_IMAGE` | `kindest/node:v1.35.0` (digest-pinned) | KinD node image paired with the `kind` pin in `.mise.toml` |
| `PLANTUML_VERSION` | `1.2026.2` | Pinned PlantUML Docker image for `make diagrams` (C4 rendering) |
| `NODE_VERSION` | `$(shell cat .nvmrc)` | Node major version sourced from `.nvmrc` (mise reads natively) |
| `GO_SERVICES` | `basket-service onboarding` | Go service directories |
| `DOTNET_SERVICES` | `order-service product-service` | .NET service directories |
| `DOTNET_TEST_PROJECTS` | `product-service.IntegrationTests order-service.IntegrationTests` | TUnit test projects run via `dotnet run --project` |

Version manager: **mise** (installed via `make deps` / `make deps-mise` / `renovate-bootstrap`). NVM has been removed per the portfolio-wide Version Manager Policy. mise installs the whole CLI toolchain from `.mise.toml`; `.NET` stays on `global.json` via `actions/setup-dotnet`. No `sudo` required.

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

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push to `main`, tags `v*`, pull requests, `workflow_call`, and `workflow_dispatch`. Instead of trigger-level `paths-ignore`, a **`changes` detector job** (`dorny/paths-filter`) gates the heavy jobs: doc-only changes (markdown, `docs/**`, license, dotfiles, image assets) skip them, while `CLAUDE.md` and `docs/diagrams/**/*.puml` are explicitly re-included as code (so `diagrams-check` still gates `.puml` edits). On tag pushes `changes.outputs.code` is forced `true` so the release pipeline never silently no-ops:
- **changes** job: `dorny/paths-filter` — every heavy job `needs: [changes]` + `if: needs.changes.outputs.code == 'true'`
- **static-check** job: Checkout, Install mise (Go + CLI toolchain), Set up .NET, `make static-check` (check-go-alignment + lint-ci + lint + vulncheck + secrets + trivy-fs + diagrams-check + deps-prune-check)
- **build** job (needs changes + static-check): Checkout, Install mise (Go + CLI toolchain), Set up .NET, `make build`
- **test** job (needs changes + static-check): Checkout, Install mise (Go + CLI toolchain), Set up .NET, `make test`
- **integration-test** job (needs changes + static-check; skipped under act via `vars.ACT`): Install mise (Go) + Set up .NET, `make integration-test` (TUnit + Testcontainers). No Dapr sidecar — onboarding's Dapr workflow lifecycle is exercised by the e2e job instead
- **e2e** job (needs build + test; skipped under act): `make e2e` — brings up 9 containers (Dapr control plane + Redis state store + postgres + rabbitmq + 3 app services + onboarding sidecar), runs 16 curl-based assertions including the full onboarding async workflow lifecycle (POST 202 → approve/deny → poll GET status), captures compose logs on failure
- **docker** job (needs changes + static-check + build + test): Checkout, Install mise (Go) + Set up .NET, `make image-build` (job-level `if` gates on tag `v*`)
- **ci-pass** job (aggregator, `if: always()`, needs all upstream jobs incl. `changes`): Verifies all upstream jobs passed (treats `skipped` as pass) — use as branch-protection required check

A cleanup workflow (`.github/workflows/cleanup-runs.yml`) weekly removes old workflow runs — keeping the newest `KEEP_MINIMUM` **per workflow** so a low-frequency workflow is never fully purged (a global minimum would flip the CI workflow to GitHub `state=deleted`) — and prunes caches from deleted branches. Also supports `workflow_dispatch`.

## Development Conventions

- Go services use standard `go build` / `go test` toolchain; `golangci-lint` config in `.golangci.yml` enables gocritic/gosec/errorlint/bodyclose/noctx/misspell
- .NET services use `dotnet build` / `dotnet format` with `TreatWarningsAsErrors` enabled
- **.NET tests use TUnit** (portfolio-wide hard requirement per `rules/dotnet/testing.md`). Run via `dotnet run --project <TestProject>` (native Microsoft.Testing.Platform entry point), NOT `dotnet test`. Fixtures use `[ClassDataSource<T>(Shared = SharedType.PerClass)]` + `IAsyncInitializer`/`IAsyncDisposable`. Assertions are async — always `await Assert.That(...)`. Mocking: FakeItEasy
- C4 architecture diagrams (`docs/diagrams/*.puml`) are rendered to PNG via the pinned `plantuml/plantuml` Docker image; `make diagrams-check` is wired into `static-check` so stale committed output fails CI
- Docker images built with `docker buildx`
- Dockerfiles linted with hadolint via `make lint`
- Dapr CLI (and the rest of the CLI toolchain) pinned in `.mise.toml` — always use a specific version, never `latest`
- The remaining Makefile `_VERSION` constants (`DAPR_RUNTIME_VERSION`, `PLANTUML_VERSION`) carry `# renovate:` inline comments; `customManagers` regexes track them plus the `kindest/node` image and the C4-PlantUML `!include`; the mise-pinned tools are tracked by Renovate's native `mise` manager
- The CLI toolchain is installed by `mise install` (`make deps` locally; `jdx/mise-action` in CI). The mise shims dir is on `PATH` (`export PATH` at the top of the Makefile) so the tools resolve in the same `make` invocation

## Skills

Use the following skills when working on related files:

| File(s) | Skill |
|---------|-------|
| `Makefile` | `/makefile` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `.github/workflows/*.{yml,yaml}` | `/ci-workflow` |

When spawning subagents, always pass conventions from the respective skill into the agent's prompt.
