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

	"github.com/dapr/durabletask-go/workflow"
	"github.com/dapr/go-sdk/client"
)

// workflowClient captures the narrow slice of workflow.Client used by the
// HTTP handlers. Keeping the interface tiny makes unit tests straightforward
// and keeps production code decoupled from the Dapr workflow SDK shape.
type workflowClient interface {
	Schedule(ctx context.Context, orchestrator string, input any) (id string, err error)
	Raise(ctx context.Context, id, eventName string, payload any) error
	AwaitOutput(ctx context.Context, id string, out any) error
}

// daprWorkflowClient adapts *workflow.Client (from dapr/durabletask-go) to
// workflowClient. It owns the ScheduleWorkflow / RaiseEvent /
// WaitForWorkflowCompletion / FetchWorkflowMetadata fan-out the handlers
// used to do inline, plus the JSON-decode of the workflow output.
type daprWorkflowClient struct {
	c *workflow.Client
}

func (d *daprWorkflowClient) Schedule(ctx context.Context, orchestrator string, input any) (string, error) {
	return d.c.ScheduleWorkflow(ctx, orchestrator, workflow.WithInput(input))
}

func (d *daprWorkflowClient) Raise(ctx context.Context, id, eventName string, payload any) error {
	return d.c.RaiseEvent(ctx, id, eventName, workflow.WithEventPayload(payload))
}

func (d *daprWorkflowClient) AwaitOutput(ctx context.Context, id string, out any) error {
	if _, err := d.c.WaitForWorkflowCompletion(ctx, id); err != nil {
		return fmt.Errorf("wait completion: %w", err)
	}
	state, err := d.c.FetchWorkflowMetadata(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}
	if state == nil || state.Output == nil {
		return errors.New("workflow completed with no output")
	}
	return json.Unmarshal([]byte(state.Output.GetValue()), out)
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

	var fullname string
	if err := s.wfClient.AwaitOutput(r.Context(), id, &fullname); err != nil {
		http.Error(w, fmt.Sprintf("await workflow failed: %v", err), http.StatusBadGateway)
		return
	}

	_ = json.NewEncoder(w).Encode(fullname)
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
