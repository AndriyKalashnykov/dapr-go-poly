# KinD e2e scaffolding

This directory is the intended home for Kubernetes manifests used by
`make e2e-kind`. It is deliberately empty today — the `make e2e-kind` target
stands up a KinD cluster and installs Dapr via `dapr init -k`, then prints a
TODO list for the remaining work.

## Why KinD in addition to Docker Compose e2e

`make e2e` (Docker Compose) is the fast, CI-ready path and currently covers
CRUD, validation negatives, and the RabbitMQ → Postgres async pipeline. KinD
adds validation for K8s-specific concerns that compose can't exercise:

- `Deployment` / `Service` manifest wiring and readiness probes
- Dapr sidecar injector (vs. standalone `daprd` processes in compose)
- Service discovery through cluster DNS
- `LoadBalancer` / `Ingress` routing (via `cloud-provider-kind` — see below)
- Dapr components in cluster mode (secret store, pub/sub, state)

The target topology is the same shape as
[`../../docs/diagrams/c4-container.puml`](../../docs/diagrams/c4-container.puml) —
the C4 Container diagram rendered to PNG under [`../../docs/diagrams/out/c4-container.png`](../../docs/diagrams/out/c4-container.png) is the reference
map for "what needs to end up as manifests."

## When you're ready to land it

1. **Manifests under `e2e/k8s/`** — `Deployment` + `Service` for each of
   `product-service`, `order-service`, `postgres`, `rabbitmq`; Dapr sidecar
   injection via `dapr.io/enabled: "true"` + `dapr.io/app-id` pod annotations.
2. **LoadBalancer controller** — **default: `cloud-provider-kind`** (one
   `docker run` on the kind Docker network; kind-team maintained, zero
   in-cluster footprint). Opt into MetalLB instead only if prod parity with
   MetalLB matters or you need BGP/FRR behavior — document the rationale here
   if so. Rationale for the default lives in the `/makefile` skill §
   "Kubernetes Targets (KinD + cloud-provider-kind)".
3. **Components** — Dapr state/pubsub `Component` resources pointing at an
   in-cluster Redis (or whichever backend you pick), applied via
   `kubectl apply -f` or a Kustomization. The existing
   `.iac/dapr/local/components/` files can be migrated once they're pointed at
   in-cluster DNS names instead of `redis-master.dapr-go.svc.cluster.local`.
4. **Test script** — `e2e/k8s-test.sh` similar in shape to
   [`../e2e-test.sh`](../e2e-test.sh) but pointing at the LoadBalancer IP
   instead of `localhost`. The compose-based script is a good template —
   it already covers RabbitMQ publish → Postgres round-trip, which the KinD
   version should replicate against the Dapr pub/sub component.
5. **Wire into `make e2e-kind`** — replace the TODO echo block in the
   Makefile with the real `kubectl apply` + `wait` + run-assertions +
   teardown sequence.

## Starting points

- `/makefile` skill §"Kubernetes Targets (KinD + cloud-provider-kind)"
  documents the canonical `kind-create` / `kind-up` / `kind-down` shape and
  explains why `cloud-provider-kind` is preferred over MetalLB by default.
- `/test-coverage-analysis` skill §"Infrastructure Decision Tree" and
  §"E2E tests (with Dapr + K8s manifests)" give a Dapr-aware end-to-end
  template including the Dapr Helm chart install step.
- `../../docs/diagrams/c4-container.puml` captures the compose topology —
  translating each `Container(...)` node to a K8s `Deployment` + `Service`
  pair is the mechanical step.
