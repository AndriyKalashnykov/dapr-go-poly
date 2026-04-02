# CLAUDE.md

## Project Overview

**dapr-go-poly** is a polyglot microservices project using [Dapr](https://dapr.io/) with Go and .NET services, orchestrated via Docker Compose.

- **Repository**: `AndriyKalashnykov/dapr-go-poly`
- **Go services**: `basket-service`, `onboarding` (Go 1.26+)
- **DotNet services**: `order-service`, `product-service` (.NET 10)

## Build & Test

```bash
make deps          # Verify required tools (go, dotnet, docker)
make build         # Build all services
make test          # Run Go tests
make lint          # Run linters (go vet + dotnet format --verify + hadolint)
make format        # Auto-fix formatting (Go + .NET)
make clean         # Remove build artifacts
make ci            # Full local CI pipeline (format, lint, test, build)
make ci-run        # Run GitHub Actions locally via act
```

## Key Variables

| Variable | Value | Purpose |
|----------|-------|---------|
| `ACT_VERSION` | `0.2.87` | Pinned act version for local CI |
| `DAPR_VERSION` | `1.17.0` | Pinned Dapr CLI version |
| `HADOLINT_VERSION` | `2.14.0` | Pinned hadolint version for Dockerfile linting |
| `NVM_VERSION` | `0.40.4` | Pinned nvm version for Renovate validation |
| `NODE_VERSION` | `22` | Pinned Node.js version for Renovate validation |
| `GO_SERVICES` | `basket-service onboarding` | Go service directories |
| `DOTNET_SERVICES` | `order-service product-service` | .NET service directories |

## Project Structure

```
basket-service/    # Go service
onboarding/        # Go service
order-service/     # .NET service (with Dockerfile)
product-service/   # .NET service (with Dockerfile)
dapr-go-poly.sln   # .NET solution file
docker-compose.yml # Local orchestration
global.json        # .NET SDK version pin
renovate.json      # Renovate dependency update configuration
```

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push to `main`, tags `v*`, pull requests, and `workflow_call`:
- **static-check** job: Checkout, Setup Go, Setup .NET, Lint
- **build** job (needs static-check): Checkout, Setup Go, Setup .NET, Build
- **test** job (needs static-check): Checkout, Setup Go, Setup .NET, Test
- **docker** job (needs build, tag-gated): Checkout, Setup .NET, Image Build

A cleanup workflow (`.github/workflows/cleanup-runs.yml`) removes old workflow runs weekly (also supports `workflow_dispatch`).

## Development Conventions

- Go services use standard `go build` / `go test` toolchain
- .NET services use `dotnet build` / `dotnet format` with `TreatWarningsAsErrors` enabled
- Docker images built with `docker buildx`
- Dockerfiles linted with hadolint via `make lint`
- Dapr CLI version pinned as `DAPR_VERSION` in Makefile — always use a specific version, never `latest`
- Makefile tool versions (`ACT_VERSION`, `DAPR_VERSION`, `HADOLINT_VERSION`, `NVM_VERSION`) tracked by Renovate via `customManagers`

## Skills

Use the following skills when working on related files:

| File(s) | Skill |
|---------|-------|
| `Makefile` | `/makefile` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `.github/workflows/*.yml` | `/ci-workflow` |

When spawning subagents, always pass conventions from the respective skill into the agent's prompt.

## Monitoring Checklist

Items to check on next upgrade analysis:

- [ ] **grpc GO-2026-4762** — `google.golang.org/grpc` v1.79.2 has auth bypass fixed in v1.79.3. Indirect dep via `dapr/go-sdk`. Not called by our code. Will resolve when Dapr Go SDK bumps grpc. Track: `dapr/go-sdk` releases.
