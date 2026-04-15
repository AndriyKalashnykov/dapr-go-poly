package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dapr/durabletask-go/api/protos"
)

func TestCreateUser_ConcatenatesFirstAndLastName(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(OnboardingRequest{
		Firstname: "Ada",
		Lastname:  "Lovelace",
		Email:     "ada@example.com",
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	ctx := activityCtx{input: input}
	out, err := CreateUser(ctx)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	got, ok := out.(string)
	if !ok {
		t.Fatalf("expected string output, got %T", out)
	}
	if want := "Ada Lovelace"; got != want {
		t.Errorf("fullname = %q, want %q", got, want)
	}
}

func TestCreateUser_ReturnsEmptyStringForEmptyInput(t *testing.T) {
	t.Parallel()

	input, _ := json.Marshal(OnboardingRequest{})

	ctx := activityCtx{input: input}
	out, err := CreateUser(ctx)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if got := out.(string); got != " " {
		t.Errorf("expected single-space fullname for empty input, got %q", got)
	}
}

// activityCtx is a minimal test double satisfying the workflow ActivityContext
// interface. CreateUser only exercises GetInput; the other methods return
// benign zero values.
type activityCtx struct {
	input []byte
}

func (a activityCtx) GetInput(out any) error                { return json.Unmarshal(a.input, out) }
func (a activityCtx) GetTaskID() int32                      { return 0 }
func (a activityCtx) GetTaskExecutionID() string            { return "test-execution-id" }
func (a activityCtx) Context() context.Context              { return context.Background() }
func (a activityCtx) GetTraceContext() *protos.TraceContext { return nil }
