Guide me through creating a new release for this project:

1. Show the latest git tag with `git describe --tags --abbrev=0` and recent tags with `git tag --sort=-v:refname | head -5`
2. Ask what the new version should be (suggest next patch, minor, and major versions)
3. Run the full pre-commit checklist:
   - `cd basket-service && go vet ./...`
   - `cd onboarding && go vet ./...`
   - `make test`
   - `make build`
4. If all checks pass, confirm the new version with me before proceeding
5. Show the exact git commands that will be run (tag, push tag) and ask for final confirmation
6. Only after my explicit approval, execute the release commands

Do NOT push or tag without my explicit confirmation at each step.
