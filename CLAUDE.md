# CLAUDE.md

## Project Overview

**dapr-go-poly** is a polyglot microservices project using [Dapr](https://dapr.io/) with Go and .NET services, orchestrated via Docker Compose.

- **Repository**: `AndriyKalashnykov/dapr-go-poly`
- **Go services**: `basket-service`, `onboarding` (Go 1.26)
- **DotNet services**: `order-service`, `product-service` (.NET 10)

## Build & Test

```bash
make deps          # Verify required tools (go, dotnet, docker)
make build         # Build all services
make test          # Run Go tests
make lint          # Run linters (go vet + dotnet format)
make clean         # Remove build artifacts
make ci            # Full local CI pipeline (clean, build, lint, test)
make ci-run        # Run GitHub Actions locally via act
```

## Project Structure

```
basket-service/    # Go service
onboarding/        # Go service
order-service/     # .NET service
product-service/   # .NET service
docker-compose.yml # Local orchestration
global.json        # .NET SDK version pin
```

## Development Conventions

- Go services use standard `go build` / `go test` toolchain
- .NET services use `dotnet build` / `dotnet format`
- Docker images built with `docker buildx`
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
