package llm

import (
	"context"
	"os/exec"
	"testing"

	"planner/pkg/config"
	"planner/pkg/planner"
)

func TestCopilotClient_AnalyzeTask(t *testing.T) {
	// Skip the test if the copilot CLI is not installed
	if _, err := exec.LookPath("copilot"); err != nil {
		t.Skip("copilot CLI not found in PATH; skipping test")
	}

	cfg := &config.Config{}
	cfg.LLM.Provider = "copilot"
	// Don't set model so it uses the default

	client, err := NewCopilotClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error creating client, got: %v", err)
	}

	req := planner.LLMRequest{
		Task: "Create a simple python hello world script in hello.py",
	}

	resp, err := client.AnalyzeTask(context.Background(), req)
	if err != nil {
		t.Fatalf("AnalyzeTask failed: %v", err)
	}

	// For a single file script, it should either be actionable or decompose.
	// As long as it parsed successfully and returned an action, it's working.
	if resp.Action == "" {
		t.Fatalf("expected an action, got empty")
	}
}

func TestCopilotClient_GeneratePlanName(t *testing.T) {
	if _, err := exec.LookPath("copilot"); err != nil {
		t.Skip("copilot CLI not found in PATH; skipping test")
	}

	cfg := &config.Config{}
	cfg.LLM.Provider = "copilot"

	client, err := NewCopilotClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error creating client, got: %v", err)
	}

	name, err := client.GeneratePlanName(context.Background(), "Add user login feature")
	if err != nil {
		t.Fatalf("GeneratePlanName failed: %v", err)
	}

	if name == "" {
		t.Fatalf("expected a plan name, got empty")
	}
}
