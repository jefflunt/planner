package llm

import (
	"context"
	"fmt"

	"planner/pkg/config"
	"planner/pkg/planner"
)

// NewClient returns a configured LLMClient based on the config provider.
func NewClient(ctx context.Context, cfg *config.Config) (planner.LLMClient, error) {
	switch cfg.LLM.Provider {
	case "gemini":
		return NewGeminiClient(ctx, cfg)
	case "copilot":
		return NewCopilotClient(ctx, cfg)
	case "opencode":
		return NewOpencodeClient(ctx, cfg)
	case "mock":
		// Mock primarily used for tests, but can be forced via config
		return &MockClient{}, nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.LLM.Provider)
	}
}

// MockClient is included for fallback and testing
type MockClient struct{}

func (m *MockClient) AnalyzeTask(ctx context.Context, req planner.LLMRequest) (planner.LLMResponse, error) {
	return planner.LLMResponse{
		Action: planner.ActionActionable,
	}, nil
}

func (m *MockClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
	return "mock-plan-name", nil
}

func (m *MockClient) ExecutePlan(ctx context.Context, plan string) (string, error) {
	return "mock implementation of plan", nil
}
