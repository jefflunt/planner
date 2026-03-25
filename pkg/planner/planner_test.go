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

func (m *simpleMockClient) AnalyzeTask(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	if resp, ok := m.responses[req.Task]; ok {
		return resp, nil
	}
	return LLMResponse{}, fmt.Errorf("unexpected task: %s", req.Task)
}

func (m *simpleMockClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
	return "test-plan", nil
}

func (m *simpleMockClient) ExecutePlan(ctx context.Context, plan string) (string, error) {
	return "mock implementation", nil
}

func TestPlannerListPlans(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some dummy plan files
	os.WriteFile(filepath.Join(tempDir, "plan1.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tempDir, "plan2.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tempDir, "not-a-plan.txt"), []byte("txt"), 0644)

	plans, err := ListPlans(tempDir)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}

	if len(plans) != 2 {
		t.Errorf("Expected 2 plans, got %d", len(plans))
	}

	foundPlan1 := false
	foundPlan2 := false
	for _, p := range plans {
		if p == "plan1" {
			foundPlan1 = true
		}
		if p == "plan2" {
			foundPlan2 = true
		}
	}

	if !foundPlan1 || !foundPlan2 {
		t.Errorf("ListPlans didn't find the expected plans: %v", plans)
	}
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

func TestPlannerDeleteNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{
		ID:   "root",
		Task: "Root",
		Children: []*Node{
			{ID: "child-1", Task: "Child 1"},
			{ID: "child-2", Task: "Child 2"},
		},
	}

	// Delete a child
	if err := p.DeleteNode("child-1"); err != nil {
		t.Fatalf("Failed to delete node: %v", err)
	}

	if len(p.Root.Children) != 1 || p.Root.Children[0].ID != "child-2" {
		t.Fatalf("Expected 1 child 'child-2', got %v", p.Root.Children)
	}

	// Delete root
	if err := p.DeleteNode("root"); err != nil {
		t.Fatalf("Failed to delete root: %v", err)
	}

	if p.Root != nil {
		t.Fatalf("Expected root to be nil, got %v", p.Root)
	}
}

func TestPlannerAddChild(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{
		ID:     "root",
		Task:   "Root",
		Status: StatusActionable,
		Type:   TaskTypeAtomic,
	}

	node, err := p.AddChild("root", "New Child")
	if err != nil {
		t.Fatalf("Failed to add child: %v", err)
	}

	if p.Root.Type != TaskTypeComposite || p.Root.Status != StatusComposite {
		t.Errorf("Expected root to become composite, got type=%s status=%s", p.Root.Type, p.Root.Status)
	}

	if len(p.Root.Children) != 1 || p.Root.Children[0].ID != node.ID {
		t.Errorf("Expected root to have the new child")
	}

	if node.Task != "New Child" {
		t.Errorf("Expected child task to be 'New Child', got %q", node.Task)
	}
}

func TestPlannerAddSibling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{
		ID:   "root",
		Task: "Root",
		Children: []*Node{
			{ID: "child-1", Task: "Child 1"},
			{ID: "child-2", Task: "Child 2"},
		},
	}

	// Add sibling after child-1
	nodeAfter, err := p.AddSibling("child-1", "New Sibling After", false)
	if err != nil {
		t.Fatalf("Failed to add sibling after: %v", err)
	}

	if len(p.Root.Children) != 3 {
		t.Fatalf("Expected 3 children, got %d", len(p.Root.Children))
	}

	if p.Root.Children[1].ID != nodeAfter.ID {
		t.Errorf("Expected new sibling to be at index 1")
	}
	if p.Root.Children[2].ID != "child-2" {
		t.Errorf("Expected child-2 to be shifted to index 2")
	}

	// Add sibling before child-1
	nodeBefore, err := p.AddSibling("child-1", "New Sibling Before", true)
	if err != nil {
		t.Fatalf("Failed to add sibling before: %v", err)
	}

	if len(p.Root.Children) != 4 {
		t.Fatalf("Expected 4 children, got %d", len(p.Root.Children))
	}
	if p.Root.Children[0].ID != nodeBefore.ID {
		t.Errorf("Expected new sibling to be at index 0")
	}

	// Cannot add sibling to root
	_, err = p.AddSibling("root", "Root Sibling", false)
	if err == nil {
		t.Errorf("Expected error when adding sibling to root")
	}
}

func TestPlannerEditNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{
		ID:     "root",
		Task:   "Root",
		Status: StatusActionable,
		Type:   TaskTypeAtomic,
		Children: []*Node{
			{ID: "child-1", Task: "Child 1"},
		},
	}

	node, err := p.EditNode("root", "New Root")
	if err != nil {
		t.Fatalf("Failed to edit node: %v", err)
	}

	if node.Task != "New Root" {
		t.Errorf("Expected task to be 'New Root', got %q", node.Task)
	}
	if node.Status != StatusPending {
		t.Errorf("Expected status to be StatusPending, got %s", node.Status)
	}
	if node.Type != "" {
		t.Errorf("Expected type to be empty, got %s", node.Type)
	}
	if len(node.Children) != 0 {
		t.Errorf("Expected 0 children, got %d", len(node.Children))
	}
}

func TestPlannerReplanNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "planner-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	cfg := Config{StateFile: stateFile}

	p := NewPlanner(cfg, &simpleMockClient{})
	p.Root = &Node{
		ID:     "root",
		Task:   "Root",
		Status: StatusActionable,
		Type:   TaskTypeAtomic,
		Children: []*Node{
			{ID: "child-1", Task: "Child 1"},
		},
	}

	node, err := p.ReplanNode("root")
	if err != nil {
		t.Fatalf("Failed to replan node: %v", err)
	}

	if node.Task != "Root" {
		t.Errorf("Expected task to remain 'Root', got %q", node.Task)
	}
	if node.Status != StatusPending {
		t.Errorf("Expected status to be StatusPending, got %s", node.Status)
	}
	if node.Type != "" {
		t.Errorf("Expected type to be empty, got %s", node.Type)
	}
	if len(node.Children) != 0 {
		t.Errorf("Expected 0 children, got %d", len(node.Children))
	}
}
