# onboarding

Go service that orchestrates user onboarding via a Dapr durable workflow.
The workflow waits on an external approval event; operators raise it with
`POST /onboardings/{id}/approve` or `POST /onboardings/{id}/deny`.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/onboarding` | Start a new workflow instance. Body: `{"firstname":"...","lastname":"...","email":"..."}`. Blocks until approved or denied. |
| POST | `/onboardings/{id}/approve` | Raise the `onboarding-approval` event on instance `{id}` with `Approved: true`. |
| POST | `/onboardings/{id}/deny` | Same, with `Approved: false`. Workflow completes with the `"was not approved"` error. |

## Local run

A Dapr sidecar is required (workflow + placement + scheduler).

```bash
# Control plane (placement + scheduler) — from the repo root
make compose-up

# Sidecar + service
cd onboarding
dapr run --app-id onboarding --app-port 8080 --dapr-grpc-port 50001 -- go run main.go
```

## Testing

- **Unit** (`go test ./...`): `CreateUser` activity + HTTP handler tests using a
  hand-written `workflowClient` fake — see [`handlers_test.go`](handlers_test.go).
- **Integration** (`go test -tags=integration ./...`): exercises the real
  Dapr workflow engine. **Auto-skips** when the sidecar isn't reachable on
  `localhost:50001` — run `dapr run` as above first, then invoke
  `make integration-test` from the repo root.

See the [repo README](../README.md) for the full three-layer test pyramid.
