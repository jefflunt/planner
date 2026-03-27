package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"planner/pkg/config"
	"planner/pkg/planner"
)

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestClaudeClient_AnalyzeTask(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "claude"
	cfg.LLM.APIKey = "test-key"

	client, err := NewClaudeClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error creating client, got: %v", err)
	}

	// Mock the HTTP client
	client.client = &http.Client{
		Transport: &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				responseBody := `{
					"content": [
						{
							"type": "text",
							"text": "{\"action\": \"actionable\", \"reasoning\": \"mock\"}"
						}
					]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := planner.LLMRequest{
		Task: "Create a simple python hello world script in hello.py",
	}

	resp, err := client.AnalyzeTask(context.Background(), req)
	if err != nil {
		t.Fatalf("AnalyzeTask failed: %v", err)
	}

	if resp.Action != "actionable" {
		t.Fatalf("expected actionable, got %v", resp.Action)
	}
}

func TestClaudeClient_GeneratePlanName(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "claude"
	cfg.LLM.APIKey = "test-key"

	client, err := NewClaudeClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error creating client, got: %v", err)
	}

	// Mock the HTTP client
	client.client = &http.Client{
		Transport: &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				responseBody := `{
					"content": [
						{
							"type": "text",
							"text": "{\"filename\": \"add-user-login-feature\"}"
						}
					]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	name, err := client.GeneratePlanName(context.Background(), "Add user login feature")
	if err != nil {
		t.Fatalf("GeneratePlanName failed: %v", err)
	}

	if name != "add-user-login-feature" {
		t.Fatalf("expected add-user-login-feature, got %v", name)
	}
}

func TestClaudeClient_GetExecCommand(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "claude"
	cfg.LLM.APIKey = "test-key"

	client, err := NewClaudeClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error creating client, got: %v", err)
	}

	req := planner.ExecRequest{
		Task:          "Some task",
		Details:       "Task details",
		AsciiDiagram:  "diagram",
		PlanStructure: "plan",
	}

	cmd, err := client.GetExecCommand(context.Background(), req)
	if err != nil {
		t.Fatalf("GetExecCommand failed: %v", err)
	}

	if cmd == nil {
		t.Fatal("expected command to be returned, got nil")
	}

	if cmd.Path != "claude" && cmd.Args[0] != "claude" {
		t.Fatalf("expected command path to be claude, got %v", cmd.Path)
	}
}
