---
allowed-tools: Bash(gh issue view*), Bash(gh api*), Bash(git checkout*), Bash(git branch*), Bash(git log*), Bash(git diff*), Bash(git status*), Bash(make *), Read, Write, Edit, Glob, Grep, Agent
---

Analyze GitHub issue #$ARGUMENTS and implement the proposed changes.

## Phase 1: Research & Analysis

1. Run `gh issue view $ARGUMENTS --json number,title,state,body,labels,comments,assignees` to fetch the full issue details
2. Read the issue title, description, labels, and all comments to understand the requirements
3. Explore the codebase to identify all files relevant to the issue
4. Analyze the impact: what needs to change, what tests are affected, what risks exist

## Phase 2: Save Research

1. Create directory `docs/research/research-$ARGUMENTS/`
2. Write `docs/research/research-$ARGUMENTS/analysis.md` containing:
   - **Issue Summary**: Title, link, key requirements extracted from the issue
   - **Affected Files**: List of files that need changes with explanations
   - **Proposed Changes**: Detailed description of each change
   - **Implementation Plan**: Step-by-step plan with phases
   - **Test Strategy**: What tests to add/modify (unit, handler, benchmark, E2E)
   - **Risks & Considerations**: Edge cases, breaking changes, backwards compatibility
   - **Estimated Complexity**: Low / Medium / High

## Phase 3: Present Plan & Get Approval

1. Present a concise summary of the analysis to the user
2. Highlight the key changes, risks, and test strategy
3. **Ask the user to approve or reject the plan before proceeding**
4. If rejected, stop and ask for feedback

## Phase 4: Implementation (only after user approval)

1. Create and switch to branch `issue-$ARGUMENTS` from the current branch:
   ```
   git checkout -b issue-$ARGUMENTS
   ```
2. Implement the changes following the plan from Phase 2
3. Follow project conventions:
   - Idiomatic Go patterns for Go services, idiomatic C# for .NET services
   - Table-driven tests for Go
   - Proper error handling
   - Input validation

## Phase 5: Verification

Run each Makefile task sequentially and fix any failures before proceeding to the next:

1. `cd basket-service && go vet ./...` — Go vet for basket-service
2. `cd onboarding && go vet ./...` — Go vet for onboarding
3. `make test` — Unit tests
4. `make build` — Compile all services

If any step fails, fix the issue and re-run that step before continuing. After all steps pass, report the final status to the user.
