[![CI](https://github.com/AndriyKalashnykov/dapr-go-poly/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/AndriyKalashnykov/dapr-go-poly/actions/workflows/ci.yml)
[![Hits](https://hits.sh/github.com/AndriyKalashnykov/dapr-go-poly.svg?view=today-total&style=plastic)](https://hits.sh/github.com/AndriyKalashnykov/dapr-go-poly/)
[![Renovate enabled](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://app.renovatebot.com/dashboard#github/AndriyKalashnykov/dapr-go-poly)

# Dapr Go Poly

A polyglot microservices project using [Dapr](https://dapr.io/) with Go and .NET services, orchestrated via Docker Compose. Demonstrates service invocation, pub/sub messaging, and state management across multiple language runtimes.

## Quick Start

```bash
make deps          # verify required tools (go, dotnet, docker)
make build         # build all services
make test          # run tests
make compose-up    # start all services via Docker Compose
```

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [GNU Make](https://www.gnu.org/software/make/) | 3.81+ | Build orchestration |
| [Go](https://go.dev/dl/) | 1.26+ | Go services (basket-service, onboarding) |
| [.NET SDK](https://dotnet.microsoft.com/download) | 10.0+ | .NET services (order-service, product-service) |
| [Docker](https://www.docker.com/) | latest | Container builds and Compose orchestration |
| [Dapr CLI](https://docs.dapr.io/getting-started/install-dapr-cli/) | latest | Local Dapr runtime (optional) |

Install all required dependencies:

```bash
make deps
```

## Available Make Targets

Run `make help` to see all available targets.

### Build & Run

| Target | Description |
|--------|-------------|
| `make build` | Build all services |
| `make test` | Run tests |
| `make clean` | Remove build artifacts |
| `make run` | Run order-service via Dapr |
| `make update` | Update all dependencies to latest versions |

### Code Quality

| Target | Description |
|--------|-------------|
| `make format` | Auto-fix formatting (Go + .NET) |
| `make lint` | Run linters (go vet + dotnet format --verify + hadolint) |

### Docker

| Target | Description |
|--------|-------------|
| `make image-build` | Build Docker images |
| `make compose-up` | Start Docker Compose services (rebuild) |
| `make compose-down` | Stop and remove Docker Compose services |

### CI

| Target | Description |
|--------|-------------|
| `make ci` | Full local CI pipeline (format, lint, test, build) |
| `make ci-run` | Run GitHub Actions workflow locally via [act](https://github.com/nektos/act) |

### Utilities

| Target | Description |
|--------|-------------|
| `make help` | List available targets (default) |
| `make deps` | Verify required tools (idempotent) |
| `make deps-act` | Install act for local CI (idempotent) |
| `make deps-hadolint` | Install hadolint for Dockerfile linting |
| `make deps-prune` | Remove unused and redundant dependencies |
| `make deps-prune-check` | Verify no prunable dependencies (CI gate) |
| `make release` | Create and push a new tag |
| `make renovate-bootstrap` | Install nvm and npm for Renovate |
| `make renovate-validate` | Validate Renovate configuration |

## Project Structure

```
basket-service/        # Go service
onboarding/            # Go service
order-service/         # .NET service (with Dockerfile)
product-service/       # .NET service (with Dockerfile)
dapr-go-poly.sln       # .NET solution file
docker-compose.yml     # Local orchestration
global.json            # .NET SDK version pin
renovate.json          # Renovate dependency update configuration
```

## CI/CD

GitHub Actions runs on every push to `main`, tags `v*`, and pull requests.

| Job | Triggers | Steps |
|-----|----------|-------|
| **static-check** | push (main, tags), PR | Lint |
| **build** | after static-check passes | Build |
| **test** | after static-check passes | Unit Tests |
| **docker** | tag pushes only (`v*`) | Image Build |

A weekly cleanup workflow removes old workflow runs (retains 7 days, minimum 5 runs).

[Renovate](https://docs.renovatebot.com/) keeps dependencies up to date with platform automerge enabled.
