# KinD e2e scaffolding

This directory is the intended home for Kubernetes manifests used by
`make e2e-kind`. It is deliberately empty today — the `make e2e-kind` target
stands up a KinD cluster and installs Dapr, then prints a TODO list for the
remaining work.

## Why KinD in addition to Docker Compose e2e

`make e2e` (Docker Compose) is the fast CI-ready path. KinD adds validation
for K8s-specific concerns that compose can't exercise:

- Deployment / Service manifest wiring and readiness probes
- Dapr sidecar injector (vs. standalone `daprd` processes)
- Service discovery through cluster DNS
- LoadBalancer / Ingress routing (via MetalLB)
- Dapr components in cluster mode (secret store, pub/sub, state)

## When you're ready to land it

1. **Manifests under `e2e/k8s/`** — `Deployment` + `Service` for each of
   `product-service`, `order-service`, `postgres`, `rabbitmq`; Dapr sidecar
   injection via `dapr.io/enabled: "true"` + `dapr.io/app-id` annotations.
2. **MetalLB** — install a minimal config so `type: LoadBalancer` services get
   routable IPs from the KinD network.
3. **Components** — Dapr state/pubsub `Component` resources pointing at the
   in-cluster Redis (or whichever backend you pick), applied via
   `kubectl apply -f` or a Kustomization.
4. **Test script** — `e2e/k8s-test.sh` similar in shape to `e2e/e2e-test.sh`
   but pointing at the LoadBalancer IP instead of `localhost`.
5. **Wire into `make e2e-kind`** — replace the TODO echo block with the real
   apply + wait + run-assertions + teardown sequence.

## Starting points

- `/makefile` skill §"Kubernetes Targets" documents the canonical
  `kind-create` / `kind-setup` / `kind-deploy` / `kind-destroy` target shape.
- `/test-coverage-analysis` skill §"Infrastructure Decision Tree" and §"E2E
  tests (with Dapr + K8s manifests)" give a Dapr-aware end-to-end template.
- The existing `.iac/dapr/local/components/` files can be migrated to
  `e2e/k8s/components/` once they're pointed at an in-cluster Redis.
