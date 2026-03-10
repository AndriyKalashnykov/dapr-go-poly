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

type Server struct {
	client   client.Client
	wfClient *workflow.Client
}

func NewServer(client client.Client, wfClient *workflow.Client) *Server {
	return &Server{
		client:   client,
		wfClient: wfClient,
	}
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
	wfClient, err := client.NewWorkflowClient()
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

	if err := wfClient.StartWorker(context.Background(), r); err != nil {
		log.Fatal(err)
	}

	c, err := client.NewClient()
	if err != nil {
		log.Fatalf("failed to intialise client: %v", err)
	}

	s := NewServer(c, wfClient)

	http.HandleFunc("POST /onboarding", s.handleCreateOnboarding)
	http.HandleFunc("POST /onboardings/{id}/approve", s.ApproveOnboarding)
	http.HandleFunc("POST /onboardings/{id}/deny", s.DenyOnboarding)

	fmt.Println("Starting web server on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) ApproveOnboarding(w http.ResponseWriter, r *http.Request) {
	runId := r.PathValue("id")

	req := OnboardingApprovalRequest{
		Approved: true,
	}

	if err := s.wfClient.RaiseEvent(context.Background(), runId, "onboarding-approval", workflow.WithEventPayload(req)); err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}

	json.NewEncoder(w).Encode("Approved")
}

func (s *Server) DenyOnboarding(w http.ResponseWriter, r *http.Request) {
	runId := r.PathValue("id")

	req := OnboardingApprovalRequest{
		Approved: false,
	}

	if err := s.wfClient.RaiseEvent(context.Background(), runId, "onboarding-approval", workflow.WithEventPayload(req)); err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}

	json.NewEncoder(w).Encode("Denied")
}

func (s *Server) handleCreateOnboarding(w http.ResponseWriter, r *http.Request) {
	log.Printf("onboarding request")
	w.Header().Set("Content-Type", "application/json")

	var request OnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		json.NewEncoder(w).Encode("Unable to parse body")
		return
	}

	id, err := s.wfClient.ScheduleWorkflow(context.Background(), "OnboardingWorkflow", workflow.WithInput(request))
	if err != nil {
		log.Fatalln("unable to start Workflow", err)
	}

	_, err = s.wfClient.WaitForWorkflowCompletion(r.Context(), id)
	if err != nil {
		log.Fatalln("unable to wait for workflow completion", err)
	}

	// Get the workflow state to retrieve results
	state, err := s.wfClient.FetchWorkflowMetadata(r.Context(), id)
	if err != nil {
		log.Fatalln("unable to fetch workflow metadata", err)
	}

	var fullname string
	if err := json.Unmarshal([]byte(state.Output.GetValue()), &fullname); err != nil {
		log.Fatalln("unable to unmarshal state data", err)
	}

	json.NewEncoder(w).Encode(fullname)
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
