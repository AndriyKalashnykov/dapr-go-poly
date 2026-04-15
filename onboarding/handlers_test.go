package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeWorkflowClient is a hand-written test double satisfying workflowClient.
// Each method returns the pre-configured value and captures the last call for
// assertion. Idiomatic for small narrow interfaces — avoids a mocking lib.
type fakeWorkflowClient struct {
	scheduleID  string
	scheduleErr error
	raiseErr    error
	stateReply  *WorkflowState
	stateErr    error

	// captured
	scheduledOrchestrator string
	scheduledInput        any
	raisedID, raisedEvent string
	raisedPayload         any
	fetchedID             string
}

func (f *fakeWorkflowClient) Schedule(_ context.Context, orchestrator string, input any) (string, error) {
	f.scheduledOrchestrator = orchestrator
	f.scheduledInput = input
	return f.scheduleID, f.scheduleErr
}

func (f *fakeWorkflowClient) Raise(_ context.Context, id, eventName string, payload any) error {
	f.raisedID = id
	f.raisedEvent = eventName
	f.raisedPayload = payload
	return f.raiseErr
}

func (f *fakeWorkflowClient) GetState(_ context.Context, id string) (*WorkflowState, error) {
	f.fetchedID = id
	return f.stateReply, f.stateErr
}

// newServeMux wires a fresh mux at every call so PathValue("id") works.
// http.HandleFunc on the default mux leaks between tests.
func newServeMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /onboarding", s.handleCreateOnboarding)
	mux.HandleFunc("GET /onboardings/{id}", s.handleGetOnboarding)
	mux.HandleFunc("POST /onboardings/{id}/approve", s.ApproveOnboarding)
	mux.HandleFunc("POST /onboardings/{id}/deny", s.DenyOnboarding)
	return mux
}

func do(t *testing.T, mux *http.ServeMux, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ------------------------------------------------------------------
// ApproveOnboarding
// ------------------------------------------------------------------

func TestApproveOnboarding_RaisesApprovalEventAndReturnsOK(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodPost, "/onboardings/run-123/approve", nil)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d (body: %s)", got, want, rec.Body.String())
	}
	if fake.raisedID != "run-123" {
		t.Errorf("raised ID = %q, want %q", fake.raisedID, "run-123")
	}
	if fake.raisedEvent != "onboarding-approval" {
		t.Errorf("event name = %q, want %q", fake.raisedEvent, "onboarding-approval")
	}
	payload, ok := fake.raisedPayload.(OnboardingApprovalRequest)
	if !ok {
		t.Fatalf("payload type = %T, want OnboardingApprovalRequest", fake.raisedPayload)
	}
	if !payload.Approved {
		t.Errorf("Approved = false, want true")
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `"Approved"`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestApproveOnboarding_ReturnsBadGatewayOnRaiseEventFailure(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{raiseErr: errors.New("sidecar unreachable")}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodPost, "/onboardings/run-xyz/approve", nil)

	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if !strings.Contains(rec.Body.String(), "sidecar unreachable") {
		t.Errorf("body = %q, want underlying error to be surfaced", rec.Body.String())
	}
}

// ------------------------------------------------------------------
// DenyOnboarding
// ------------------------------------------------------------------

func TestDenyOnboarding_RaisesDenialEventAndReturnsOK(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodPost, "/onboardings/run-456/deny", nil)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	payload, ok := fake.raisedPayload.(OnboardingApprovalRequest)
	if !ok || payload.Approved {
		t.Errorf("expected Approved=false, got payload=%+v (ok=%v)", fake.raisedPayload, ok)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `"Denied"`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

// ------------------------------------------------------------------
// handleCreateOnboarding — async 202 shape
// ------------------------------------------------------------------

func TestHandleCreateOnboarding_ReturnsAcceptedWithInstanceID(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{scheduleID: "wf-abc"}
	s := NewServer(fake)

	body := bytes.NewBufferString(`{"firstname":"Grace","lastname":"Hopper","email":"grace@example.com"}`)
	rec := do(t, newServeMux(s), http.MethodPost, "/onboarding", body)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Errorf("status = %d, want %d (body: %s)", got, want, rec.Body.String())
	}
	if fake.scheduledOrchestrator != "OnboardingWorkflow" {
		t.Errorf("orchestrator = %q, want %q", fake.scheduledOrchestrator, "OnboardingWorkflow")
	}
	input, ok := fake.scheduledInput.(OnboardingRequest)
	if !ok || input.Firstname != "Grace" || input.Lastname != "Hopper" {
		t.Errorf("scheduled input = %+v (type %T), want OnboardingRequest{Firstname: Grace, Lastname: Hopper}", fake.scheduledInput, fake.scheduledInput)
	}

	var got WorkflowState
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	if got.ID != "wf-abc" {
		t.Errorf("id = %q, want %q", got.ID, "wf-abc")
	}
	if got.Status != "Running" {
		t.Errorf("status = %q, want %q", got.Status, "Running")
	}
	if got.Result != "" {
		t.Errorf("Result should be empty on 202; got %q", got.Result)
	}
}

func TestHandleCreateOnboarding_MalformedBodyReturns400(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodPost, "/onboarding", strings.NewReader("not-json"))

	if got, want := rec.Code, http.StatusBadRequest; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if fake.scheduledOrchestrator != "" {
		t.Errorf("ScheduleWorkflow should not be called on malformed body, got orchestrator=%q", fake.scheduledOrchestrator)
	}
}

func TestHandleCreateOnboarding_ScheduleFailureReturns502(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{scheduleErr: errors.New("placement not reachable")}
	s := NewServer(fake)

	body := bytes.NewBufferString(`{"firstname":"X","lastname":"Y","email":"x@y.com"}`)
	rec := do(t, newServeMux(s), http.MethodPost, "/onboarding", body)

	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if !strings.Contains(rec.Body.String(), "placement not reachable") {
		t.Errorf("body = %q, want underlying error to be surfaced", rec.Body.String())
	}
}

// ------------------------------------------------------------------
// handleGetOnboarding — new GET /onboardings/{id}
// ------------------------------------------------------------------

func TestHandleGetOnboarding_ReturnsRunningState(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{stateReply: &WorkflowState{ID: "wf-1", Status: "Running"}}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodGet, "/onboardings/wf-1", nil)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if fake.fetchedID != "wf-1" {
		t.Errorf("fetched id = %q, want %q", fake.fetchedID, "wf-1")
	}
	var body WorkflowState
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "Running" {
		t.Errorf("status = %q, want Running", body.Status)
	}
}

func TestHandleGetOnboarding_ReturnsCompletedStateWithResult(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{stateReply: &WorkflowState{
		ID: "wf-2", Status: "Completed", Result: "Grace Hopper",
	}}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodGet, "/onboardings/wf-2", nil)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	var body WorkflowState
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "Completed" {
		t.Errorf("status = %q, want Completed", body.Status)
	}
	if body.Result != "Grace Hopper" {
		t.Errorf("result = %q, want Grace Hopper", body.Result)
	}
}

func TestHandleGetOnboarding_ReturnsFailedStateWithError(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{stateReply: &WorkflowState{
		ID: "wf-3", Status: "Failed", Error: "was not approved",
	}}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodGet, "/onboardings/wf-3", nil)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	var body WorkflowState
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "Failed" {
		t.Errorf("status = %q, want Failed", body.Status)
	}
	if body.Error != "was not approved" {
		t.Errorf("error = %q, want %q", body.Error, "was not approved")
	}
}

func TestHandleGetOnboarding_ReturnsBadGatewayWhenSidecarErrors(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{stateErr: errors.New("instance not found")}
	s := NewServer(fake)

	rec := do(t, newServeMux(s), http.MethodGet, "/onboardings/unknown", nil)

	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if !strings.Contains(rec.Body.String(), "instance not found") {
		t.Errorf("body = %q, want underlying error to be surfaced", rec.Body.String())
	}
}
