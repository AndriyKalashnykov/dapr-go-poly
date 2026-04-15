# onboarding

Go service that orchestrates user onboarding via a Dapr durable workflow.
POST creates a new workflow instance and returns its id immediately; an
operator approves or denies it, and the caller polls for the result.

## API

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/onboarding` | Schedule a new workflow. Body: `{"firstname":"...","lastname":"...","email":"..."}`. Returns **202 Accepted** with `{"id":"...","status":"Running"}`. |
| `GET`  | `/onboardings/{id}` | Return the current workflow state: `{"id","status","result?","error?"}`. `status` is one of `Running` / `Completed` / `Failed` / `Terminated` / `Canceled` / `Pending` / `Suspended`. On completion, `result` holds the full name. On denial, `error` holds the workflow error message. |
| `POST` | `/onboardings/{id}/approve` | Raise the `onboarding-approval` event on instance `{id}` with `Approved: true`. Returns `"Approved"`. |
| `POST` | `/onboardings/{id}/deny` | Same, with `Approved: false`. Workflow completes with error `"was not approved"`, surfaced via `GET /onboardings/{id}`'s `error` field. |

### End-to-end flow

```bash
# 1. Create — returns id immediately
id=$(curl -sf -X POST http://localhost:8080/onboarding \
       -H 'Content-Type: application/json' \
       -d '{"firstname":"Grace","lastname":"Hopper","email":"grace@example.com"}' \
     | jq -r .id)

# 2. Approve (or /deny)
curl -sf -X POST "http://localhost:8080/onboardings/$id/approve"

# 3. Poll state
curl -sf "http://localhost:8080/onboardings/$id"
# → {"id":"...","status":"Completed","result":"Grace Hopper"}
```

This shape replaces an earlier design where `POST /onboarding` blocked until
approval and never returned the run id — making the approve/deny endpoints
effectively unaddressable from any client that didn't already know the id
out of band. See [`../e2e/e2e-test.sh`](../e2e/e2e-test.sh) for the
canonical end-to-end exercise of all three paths.

## Local run

Requires a Dapr sidecar with workflow + placement + scheduler + an actor
state store. The simplest way is `make e2e` (brings up the full stack and
tears it down); for iterative development:

```bash
# Bring up Dapr control plane + Redis state store + component
# See e2e/docker-compose.e2e.yml + e2e/dapr/components/statestore.yaml
make compose-up

# In a separate terminal — sidecar + service
cd onboarding
dapr run --app-id onboarding \
  --app-port 8080 \
  --dapr-grpc-port 50001 \
  --resources-path ../e2e/dapr/components \
  -- go run .
```

## Testing

- **Unit** (`go test ./...`): `CreateUser` activity + HTTP handler tests using a
  hand-written `workflowClient` fake that satisfies the narrow three-method
  interface (`Schedule` / `Raise` / `GetState`). See
  [`handlers_test.go`](handlers_test.go) — 9 tests covering happy paths, the
  malformed-body and schedule-failure branches on POST, and the
  Running/Completed/Failed state projections on GET.
- **Integration** (`go test -tags=integration ./...`): currently empty. The
  previously-held sidecar-gated tests were removed once `make e2e` started
  exercising the same surface via HTTP against a real Dapr sidecar (with
  Redis actor state store) — duplicate coverage.
- **E2E** is where full workflow lifecycle is verified: `make e2e` runs
  `e2e/e2e-test.sh` which POSTs onboarding, approves, polls
  `GET /onboardings/{id}` until `status=Completed`, and asserts
  `result="E2E Widget"`. A parallel denial case asserts `status=Failed` +
  `error` contains `"not approved"`.

See the [repo README](../README.md) for the full three-layer test pyramid.

## Known upstream issue

Dapr 1.17.3's `backend.(*grpcExecutor).GetInstance` panics with a
nil-pointer dereference when asked about a workflow instance that was never
scheduled (durabletask-go). The sidecar becomes unreachable for the rest of
the process lifetime. The e2e suite avoids triggering this by only calling
`GET /onboardings/{id}` on ids returned from a prior `POST /onboarding`.
Track the bug in dapr/durabletask-go issues; remove the workaround once
1.17.4+ ships with a fix.
