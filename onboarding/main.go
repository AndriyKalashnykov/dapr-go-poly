package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/dapr/durabletask-go/api/protos"
	"github.com/dapr/durabletask-go/workflow"
	"github.com/dapr/go-sdk/client"
)

// workflowClient captures the narrow slice of workflow.Client used by the
// HTTP handlers. Keeping the interface tiny makes unit tests straightforward
// and keeps production code decoupled from the Dapr workflow SDK shape.
type workflowClient interface {
	Schedule(ctx context.Context, orchestrator string, input any) (id string, err error)
	Raise(ctx context.Context, id, eventName string, payload any) error
	GetState(ctx context.Context, id string) (*WorkflowState, error)
}

// WorkflowState is the externally visible projection of a Dapr workflow
// instance. It's the body of the 202 response from POST /onboarding and the
// body of GET /onboardings/{id}.
type WorkflowState struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// daprWorkflowClient adapts *workflow.Client (from dapr/durabletask-go) to
// workflowClient. It owns the ScheduleWorkflow / RaiseEvent /
// FetchWorkflowMetadata fan-out plus the JSON-decode of the workflow output
// and translation of the Dapr runtime status enum into the simple status
// string the HTTP API exposes.
type daprWorkflowClient struct {
	c *workflow.Client
}

func (d *daprWorkflowClient) Schedule(ctx context.Context, orchestrator string, input any) (string, error) {
	return d.c.ScheduleWorkflow(ctx, orchestrator, workflow.WithInput(input))
}

func (d *daprWorkflowClient) Raise(ctx context.Context, id, eventName string, payload any) error {
	return d.c.RaiseEvent(ctx, id, eventName, workflow.WithEventPayload(payload))
}

// GetState returns the current state of a workflow without blocking. For a
// running workflow it returns Status="Running" and empty Result/Error. For a
// completed workflow it decodes the output into Result. For a failed workflow
// (e.g., onboarding denied) it surfaces the FailureDetails error message.
func (d *daprWorkflowClient) GetState(ctx context.Context, id string) (*WorkflowState, error) {
	state, err := d.c.FetchWorkflowMetadata(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	if state == nil {
		return nil, errors.New("workflow instance not found")
	}

	out := &WorkflowState{
		ID:     id,
		Status: normalizeRuntimeStatus(state.RuntimeStatus),
	}
	if state.Output != nil && state.Output.GetValue() != "" {
		var result string
		if err := json.Unmarshal([]byte(state.Output.GetValue()), &result); err == nil {
			out.Result = result
		}
	}
	if state.FailureDetails != nil {
		out.Error = state.FailureDetails.GetErrorMessage()
	}
	return out, nil
}

// normalizeRuntimeStatus maps Dapr's protobuf-style status enum to a short
// PascalCase string. Unknown values fall through to the raw enum .String()
// form so we don't silently drop new statuses if Dapr adds them.
func normalizeRuntimeStatus(rs protos.OrchestrationStatus) string {
	switch rs {
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_RUNNING:
		return "Running"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_COMPLETED:
		return "Completed"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_FAILED:
		return "Failed"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_TERMINATED:
		return "Terminated"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_PENDING:
		return "Pending"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_SUSPENDED:
		return "Suspended"
	case protos.OrchestrationStatus_ORCHESTRATION_STATUS_CANCELED:
		return "Canceled"
	default:
		return rs.String()
	}
}

type Server struct {
	wfClient workflowClient
}

func NewServer(wfClient workflowClient) *Server {
	return &Server{wfClient: wfClient}
}

type OnboardingRequest struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Email     string `json:"email"`
}

type OnboardingApprovalRequest struct {
	Approved bool
}

func main() {
	wf, err := client.NewWorkflowClient()
	if err != nil {
		log.Fatalf("failed to initialise workflow client: %v", err)
	}

	r := workflow.NewRegistry()
	if err := r.AddWorkflow(OnboardingWorkflow); err != nil {
		log.Fatal(err)
	}
	if err := r.AddActivity(CreateUser); err != nil {
		log.Fatal(err)
	}

	if err := wf.StartWorker(context.Background(), r); err != nil {
		log.Fatal(err)
	}

	s := NewServer(&daprWorkflowClient{c: wf})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /onboarding", s.handleCreateOnboarding)
	mux.HandleFunc("GET /onboardings/{id}", s.handleGetOnboarding)
	mux.HandleFunc("POST /onboardings/{id}/approve", s.ApproveOnboarding)
	mux.HandleFunc("POST /onboardings/{id}/deny", s.DenyOnboarding)

	fmt.Println("Starting web server on http://localhost:8080")
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// handleCreateOnboarding schedules a new workflow instance and returns 202
// with the instance ID immediately — it does NOT wait for approval. Clients
// then raise the approval/denial event via the /approve or /deny endpoints
// and poll GET /onboardings/{id} for completion.
func (s *Server) handleCreateOnboarding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var request OnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "unable to parse body", http.StatusBadRequest)
		return
	}

	id, err := s.wfClient.Schedule(r.Context(), "OnboardingWorkflow", request)
	if err != nil {
		http.Error(w, fmt.Sprintf("schedule workflow failed: %v", err), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(WorkflowState{
		ID:     id,
		Status: "Running",
	})
}

// handleGetOnboarding returns the current state of a workflow instance.
// Non-blocking — returns Status="Running" while the workflow waits for an
// approval event. Once the workflow completes, Result holds the fullname;
// on failure (e.g., denied), Error holds the workflow error message.
func (s *Server) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing workflow id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	state, err := s.wfClient.GetState(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch state failed: %v", err), http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(state)
}

func (s *Server) ApproveOnboarding(w http.ResponseWriter, r *http.Request) {
	s.raiseApproval(w, r, true, "Approved")
}

func (s *Server) DenyOnboarding(w http.ResponseWriter, r *http.Request) {
	s.raiseApproval(w, r, false, "Denied")
}

func (s *Server) raiseApproval(w http.ResponseWriter, r *http.Request, approved bool, okMessage string) {
	runID := r.PathValue("id")
	if runID == "" {
		http.Error(w, "missing workflow id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	err := s.wfClient.Raise(r.Context(), runID, "onboarding-approval",
		OnboardingApprovalRequest{Approved: approved})
	if err != nil {
		http.Error(w, fmt.Sprintf("raise event failed: %v", err), http.StatusBadGateway)
		return
	}

	_ = json.NewEncoder(w).Encode(okMessage)
}

func OnboardingWorkflow(ctx *workflow.WorkflowContext) (any, error) {
	var request OnboardingRequest
	if err := ctx.GetInput(&request); err != nil {
		return nil, err
	}

	// Manual interaction
	var signal OnboardingApprovalRequest
	err := ctx.WaitForExternalEvent("onboarding-approval", time.Duration(math.MaxInt64)).Await(&signal)
	if err != nil {
		return nil, err
	}

	if !signal.Approved {
		return nil, errors.New("was not approved")
	}

	// Resume run

	// Create user
	var fullname string
	err = ctx.CallActivity(CreateUser, workflow.WithActivityInput(request)).Await(&fullname)

	return fullname, err
}

func CreateUser(ctx workflow.ActivityContext) (any, error) {
	var request OnboardingRequest
	if err := ctx.GetInput(&request); err != nil {
		return "", err
	}

	fullname := fmt.Sprintf("%s %s", request.Firstname, request.Lastname)

	return fullname, nil
}
