package planner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// simpleMockClient replaces the complex MockClient from pkg/llm for predictable testing
type simpleMockClient struct {
	responses map[string]LLMResponse
}

func (m *simpleMockClient) AnalyzeTask(ctx context.Context, task string, isVision bool) (LLMResponse, error) {
	if resp, ok := m.responses[task]; ok {
		return resp, nil
	}
	return LLMResponse{}, fmt.Errorf("unexpected task: %s", task)
}

func TestPlannerPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{ID: "test-id", Task: "Testing Save"}

	// Test Save
	if err := p.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatalf("State file was not created")
	}

	// Test Load into a new planner instance
	p2 := NewPlanner(cfg, nil)
	if err := p2.Load(); err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if p2.Root == nil || p2.Root.ID != "test-id" {
		t.Errorf("Loaded state does not match saved state")
	}
}

func TestPlannerStart(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	client := &simpleMockClient{
		responses: map[string]LLMResponse{
			"Do a simple task": {Action: ActionActionable},
		},
	}

	p := NewPlanner(cfg, client)
	ctx := context.Background()

	// Starting should initialize the root node and trigger Plan()
	err = p.Start(ctx, "Do a simple task")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if p.Root == nil {
		t.Fatalf("Root node was not created")
	}

	if p.Root.Task != "Do a simple task" {
		t.Errorf("Expected root task to be 'Do a simple task'")
	}

	if p.Root.Status != StatusActionable {
		t.Errorf("Expected root status to be Actionable")
	}
}

func TestPlannerPlanDecomposition(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	client := &simpleMockClient{
		responses: map[string]LLMResponse{
			"Complex Task": {
				Action:   ActionDecompose,
				Subtasks: []string{"Subtask 1", "Subtask 2"},
			},
			"Subtask 1": {Action: ActionActionable},
			"Subtask 2": {Action: ActionActionable},
		},
	}

	p := NewPlanner(cfg, client)
	p.Root = &Node{ID: "root", Task: "Complex Task", Status: StatusPending}

	err = p.Plan(context.Background(), p.Root)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if len(p.Root.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(p.Root.Children))
	}

	if p.Root.Children[0].Status != StatusActionable || p.Root.Children[1].Status != StatusActionable {
		t.Errorf("Expected children to be actionable")
	}
}

func TestPlannerPlanAskUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	client := &simpleMockClient{
		responses: map[string]LLMResponse{
			"Ambiguous task": {
				Action:   ActionAskUser,
				Question: "What do you mean?",
			},
			"Ambiguous task\n\n[Clarification]: I mean X": {
				Action: ActionActionable, // After clarification, it becomes actionable
			},
		},
	}

	p := NewPlanner(cfg, client)
	p.Root = &Node{ID: "root", Task: "Ambiguous task", Status: StatusPending}

	// Run planning in a separate goroutine because it will block on asking the user
	errChan := make(chan error, 1)
	go func() {
		errChan <- p.Plan(context.Background(), p.Root)
	}()

	// Wait for the prompt
	select {
	case prompt := <-p.Prompts:
		if prompt.Question != "What do you mean?" {
			t.Errorf("Unexpected question: %s", prompt.Question)
		}
		// Send reply
		prompt.ReplyChan <- "I mean X"
	case <-time.After(1 * time.Second):
		t.Fatalf("Timed out waiting for prompt")
	}

	// Wait for planning to finish
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Timed out waiting for Plan to finish")
	}

	// Verify the node's task string was updated
	expectedTask := "Ambiguous task\n\n[Clarification]: I mean X"
	if p.Root.Task != expectedTask {
		t.Errorf("Expected task %q, got %q", expectedTask, p.Root.Task)
	}

	if p.Root.Status != StatusActionable {
		t.Errorf("Expected status to be actionable, got %s", p.Root.Status)
	}
}
