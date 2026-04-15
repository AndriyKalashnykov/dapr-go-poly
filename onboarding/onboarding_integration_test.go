//go:build integration

// Integration test suite for the onboarding service.
//
// These tests exercise the real Dapr Workflow runtime and therefore require a
// Dapr sidecar to be reachable. They're gated behind the `integration` build
// tag so `make test` skips them. Run via `make integration-test` with a sidecar
// running (e.g., `dapr run --app-id onboarding -- sleep 3600` in a separate
// terminal) — without one, the test is skipped rather than failed so local
// `make integration-test` runs stay green when the developer hasn't spun up
// Dapr.

package main

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dapr/durabletask-go/workflow"
	"github.com/dapr/go-sdk/client"
)

func TestOnboardingWorkflow_ApprovedPath_ProducesFullName(t *testing.T) {
	if !daprSidecarReachable(t) {
		t.Skip("Dapr sidecar not reachable on the default gRPC port; skipping")
	}

	wfClient, err := newWorkflowClientWithRetry(t, 5*time.Second)
	if err != nil {
		t.Fatalf("workflow client: %v", err)
	}

	// Register the workflow/activity as the real server would.
	registry := workflow.NewRegistry()
	if err := registry.AddWorkflow(OnboardingWorkflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}
	if err := registry.AddActivity(CreateUser); err != nil {
		t.Fatalf("register activity: %v", err)
	}
	if err := wfClient.StartWorker(t.Context(), registry); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	input := OnboardingRequest{Firstname: "Grace", Lastname: "Hopper", Email: "grace@example.com"}
	runID, err := wfClient.ScheduleWorkflow(t.Context(), "OnboardingWorkflow", workflow.WithInput(input))
	if err != nil {
		t.Fatalf("schedule workflow: %v", err)
	}

	approval := OnboardingApprovalRequest{Approved: true}
	if err := wfClient.RaiseEvent(t.Context(), runID, "onboarding-approval", workflow.WithEventPayload(approval)); err != nil {
		t.Fatalf("raise approval event: %v", err)
	}

	state, err := wfClient.WaitForWorkflowCompletion(t.Context(), runID)
	if err != nil {
		t.Fatalf("wait for completion: %v", err)
	}

	var fullname string
	if err := json.Unmarshal([]byte(state.Output.GetValue()), &fullname); err != nil {
		t.Fatalf("decode workflow output: %v", err)
	}
	if want := "Grace Hopper"; fullname != want {
		t.Errorf("fullname = %q, want %q", fullname, want)
	}
}

// After ScheduleWorkflow but BEFORE raising the approval event, the workflow
// must be in RUNNING state waiting for the external event. Regressions here
// would mean the workflow exits prematurely (e.g., WaitForExternalEvent is
// wired wrong) — something the happy-path test above doesn't catch.
func TestOnboardingWorkflow_RemainsRunningUntilApprovalEvent(t *testing.T) {
	if !daprSidecarReachable(t) {
		t.Skip("Dapr sidecar not reachable on the default gRPC port; skipping")
	}

	wfClient, err := newWorkflowClientWithRetry(t, 5*time.Second)
	if err != nil {
		t.Fatalf("workflow client: %v", err)
	}

	registry := workflow.NewRegistry()
	_ = registry.AddWorkflow(OnboardingWorkflow)
	_ = registry.AddActivity(CreateUser)
	if err := wfClient.StartWorker(t.Context(), registry); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	runID, err := wfClient.ScheduleWorkflow(t.Context(), "OnboardingWorkflow",
		workflow.WithInput(OnboardingRequest{Firstname: "Waiting", Lastname: "Case"}))
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}

	// Wait for the workflow to enter the waiting state, then confirm it's
	// not prematurely completed.
	if _, err := wfClient.WaitForWorkflowStart(t.Context(), runID); err != nil {
		t.Fatalf("wait for start: %v", err)
	}

	state, err := wfClient.FetchWorkflowMetadata(t.Context(), runID)
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if workflow.WorkflowMetadataIsComplete(state) {
		t.Errorf("workflow is already complete before approval event; status=%s", state.RuntimeStatus)
	}

	// Clean up by raising approval so the worker doesn't leak.
	_ = wfClient.RaiseEvent(t.Context(), runID, "onboarding-approval",
		workflow.WithEventPayload(OnboardingApprovalRequest{Approved: true}))
}

// RaiseEvent on a workflow instance ID that was never scheduled must not
// cause the server to crash. Dapr's workflow runtime returns an error; the
// production path (ApproveOnboarding/DenyOnboarding handlers) surfaces it as
// 502 — exercised by the handler unit tests. This integration test verifies
// the Dapr runtime itself actually returns a non-nil error.
func TestRaiseEvent_OnUnknownInstance_ReturnsError(t *testing.T) {
	if !daprSidecarReachable(t) {
		t.Skip("Dapr sidecar not reachable on the default gRPC port; skipping")
	}

	wfClient, err := newWorkflowClientWithRetry(t, 5*time.Second)
	if err != nil {
		t.Fatalf("workflow client: %v", err)
	}

	err = wfClient.RaiseEvent(t.Context(), "no-such-instance-"+time.Now().Format("20060102150405"),
		"onboarding-approval",
		workflow.WithEventPayload(OnboardingApprovalRequest{Approved: true}))
	if err == nil {
		t.Error("expected error raising event on unknown instance, got nil")
	}
}

func TestOnboardingWorkflow_DeniedPath_FailsWithNotApproved(t *testing.T) {
	if !daprSidecarReachable(t) {
		t.Skip("Dapr sidecar not reachable on the default gRPC port; skipping")
	}

	wfClient, err := newWorkflowClientWithRetry(t, 5*time.Second)
	if err != nil {
		t.Fatalf("workflow client: %v", err)
	}

	registry := workflow.NewRegistry()
	_ = registry.AddWorkflow(OnboardingWorkflow)
	_ = registry.AddActivity(CreateUser)
	if err := wfClient.StartWorker(t.Context(), registry); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	runID, err := wfClient.ScheduleWorkflow(t.Context(), "OnboardingWorkflow",
		workflow.WithInput(OnboardingRequest{Firstname: "Denied", Lastname: "Case"}))
	if err != nil {
		t.Fatalf("schedule workflow: %v", err)
	}

	if err := wfClient.RaiseEvent(t.Context(), runID, "onboarding-approval",
		workflow.WithEventPayload(OnboardingApprovalRequest{Approved: false})); err != nil {
		t.Fatalf("raise denial event: %v", err)
	}

	state, err := wfClient.WaitForWorkflowCompletion(t.Context(), runID)
	if err != nil {
		t.Fatalf("wait for completion: %v", err)
	}

	// Denied path returns an error from the workflow, which surfaces as a failure state.
	if state.RuntimeStatus.String() == "COMPLETED" {
		t.Errorf("expected workflow to fail on denial, got status %s", state.RuntimeStatus)
	}
}

// daprSidecarReachable attempts a tiny TCP dial to the Dapr gRPC port.
// Avoids forcing local devs to install Dapr just to run `make integration-test`.
func daprSidecarReachable(t *testing.T) bool {
	t.Helper()

	port := os.Getenv("DAPR_GRPC_PORT")
	if port == "" {
		port = "50001"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func newWorkflowClientWithRetry(t *testing.T, budget time.Duration) (*workflow.Client, error) {
	t.Helper()

	deadline := time.Now().Add(budget)
	var lastErr error
	for time.Now().Before(deadline) {
		c, err := client.NewWorkflowClient()
		if err == nil {
			return c, nil
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("unknown error creating workflow client")
	}
	return nil, lastErr
}
