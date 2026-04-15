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
	awaitOutput string
	awaitErr    error

	// captured
	scheduledOrchestrator string
	scheduledInput        any
	raisedID, raisedEvent string
	raisedPayload         any
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

func (f *fakeWorkflowClient) AwaitOutput(_ context.Context, _ string, out any) error {
	if f.awaitErr != nil {
		return f.awaitErr
	}
	return json.Unmarshal([]byte(f.awaitOutput), out)
}

// newServeMux wires a fresh mux at every call so PathValue("id") works.
// http.HandleFunc on the default mux leaks between tests.
func newServeMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /onboarding", s.handleCreateOnboarding)
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
// handleCreateOnboarding — happy + error paths
// ------------------------------------------------------------------

func TestHandleCreateOnboarding_ScheduleAwaitAndReturnFullname(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{
		scheduleID:  "wf-abc",
		awaitOutput: `"Grace Hopper"`,
	}
	s := NewServer(fake)

	body := bytes.NewBufferString(`{"firstname":"Grace","lastname":"Hopper","email":"grace@example.com"}`)
	rec := do(t, newServeMux(s), http.MethodPost, "/onboarding", body)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d (body: %s)", got, want, rec.Body.String())
	}
	if fake.scheduledOrchestrator != "OnboardingWorkflow" {
		t.Errorf("orchestrator = %q, want %q", fake.scheduledOrchestrator, "OnboardingWorkflow")
	}
	input, ok := fake.scheduledInput.(OnboardingRequest)
	if !ok || input.Firstname != "Grace" || input.Lastname != "Hopper" {
		t.Errorf("scheduled input = %+v (type %T), want OnboardingRequest{Firstname: Grace, Lastname: Hopper}", fake.scheduledInput, fake.scheduledInput)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `"Grace Hopper"`; got != want {
		t.Errorf("body = %q, want %q", got, want)
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

func TestHandleCreateOnboarding_AwaitFailureReturns502(t *testing.T) {
	t.Parallel()
	fake := &fakeWorkflowClient{
		scheduleID: "wf-xyz",
		awaitErr:   errors.New("workflow timed out"),
	}
	s := NewServer(fake)

	body := bytes.NewBufferString(`{"firstname":"X","lastname":"Y","email":"x@y.com"}`)
	rec := do(t, newServeMux(s), http.MethodPost, "/onboarding", body)

	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}
