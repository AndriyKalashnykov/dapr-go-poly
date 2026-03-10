Run the full pre-commit checklist for this project. Execute each step sequentially and report results:

1. `cd basket-service && go vet ./...` — Go vet for basket-service
2. `cd onboarding && go vet ./...` — Go vet for onboarding
3. `make test` — Unit tests (Go services)
4. `make build` — Compile all services (Go + .NET)

After all steps, provide a summary table showing pass/fail status for each check. If any step fails, show the relevant error output and suggest a fix.
