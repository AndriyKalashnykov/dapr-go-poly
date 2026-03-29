# CLAUDE.md

## Project Overview

**dapr-go-poly** is a polyglot microservices project using [Dapr](https://dapr.io/) with Go and .NET services, orchestrated via Docker Compose.

- **Repository**: `AndriyKalashnykov/dapr-go-poly`
- **Go services**: `basket-service`, `onboarding` (Go 1.24+)
- **DotNet services**: `order-service`, `product-service` (.NET 10)

## Build & Test

```bash
make deps          # Verify required tools (go, dotnet, docker)
make build         # Build all services
make test          # Run Go tests
make lint          # Run linters (go vet + dotnet format + hadolint)
make clean         # Remove build artifacts
make ci            # Full local CI pipeline (clean, build, lint, test)
make ci-run        # Run GitHub Actions locally via act
```

## Key Variables

| Variable | Value | Purpose |
|----------|-------|---------|
| `ACT_VERSION` | `0.2.86` | Pinned act version for local CI |
| `HADOLINT_VERSION` | `2.12.0` | Pinned hadolint version for Dockerfile linting |
| `NVM_VERSION` | `0.40.4` | Pinned nvm version for Renovate validation |
| `GO_SERVICES` | `basket-service onboarding` | Go service directories |
| `DOTNET_SERVICES` | `order-service product-service` | .NET service directories |

## Project Structure

```
basket-service/    # Go service
onboarding/        # Go service
order-service/     # .NET service (with Dockerfile)
product-service/   # .NET service (with Dockerfile)
.dapr/             # Dapr sidecar configuration
docker-compose.yml # Local orchestration
global.json        # .NET SDK version pin
```

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push to `main`, tags `v*`, and pull requests:
- **builds** job: Checkout, Setup Go, Setup .NET, Build, Lint, Image Build
- **tests** job: Checkout, Setup Go, Run unit tests

A cleanup workflow (`.github/workflows/cleanup-runs.yml`) removes old workflow runs weekly.

## Development Conventions

- Go services use standard `go build` / `go test` toolchain
- .NET services use `dotnet build` / `dotnet format`
- Docker images built with `docker buildx`
- Dockerfiles linted with hadolint via `make lint`
- Dapr sidecar configuration in `.dapr/`

## Skills

Use the following skills when working on related files:

| File(s) | Skill |
|---------|-------|
| `Makefile` | `/makefile` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `.github/workflows/*.yml` | `/ci-workflow` |

When spawning subagents, always pass conventions from the respective skill into the agent's prompt.
