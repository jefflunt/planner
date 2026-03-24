package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"planner/pkg/config"
	"planner/pkg/planner"
)

type GeminiClient struct {
	client *genai.Client
	model  string
}

func NewGeminiClient(ctx context.Context, cfg *config.Config) (*GeminiClient, error) {
	apiKey := cfg.LLM.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("gemini api key is required in config or via GEMINI_API_KEY environment variable")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	model := cfg.LLM.Model
	if model == "" {
		model = "gemini-3.1-flash-lite-preview"
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiClient) AnalyzeTask(ctx context.Context, task string) (planner.LLMResponse, error) {
	model := g.client.GenerativeModel(g.model)

	// Force JSON output
	model.ResponseMIMEType = "application/json"

	prompt := fmt.Sprintf(`You are an expert agentic task orchestrator. Your job is to analyze a task and decide whether it is actionable, requires decomposition, or needs clarification from the user.

CRITICAL RULE (Actionable Heuristic): 
A task is ONLY "actionable" if it describes the creation, deletion, or editing of ONE SINGLE FILE on disk. 
- Example: "Refactor the authentication module" -> Not Actionable (Too vague, multiple files).
- Example: "Rename AuthUser to SessionUser in src/auth/models.go" -> Actionable (Single file operation).

If a task is too large or modifies multiple files (e.g. "Rename type X and all references"), you MUST decompose it into multiple actionable steps.

Analyze this task:
"""
%s
"""

Respond with a JSON object containing:
1. "action": Must be exactly one of "actionable", "decompose", or "ask_user".
2. "reasoning": A brief explanation of why you chose this action.
3. "subtasks": If action is "decompose", provide a JSON array of strings, where each string is a smaller, more specific subtask.
4. "question": If action is "ask_user", provide the clarification question you want to ask the user.

JSON Format:
{
  "action": "...",
  "reasoning": "...",
  "subtasks": [...],
  "question": "..."
}`, task)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return planner.LLMResponse{}, fmt.Errorf("gemini generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return planner.LLMResponse{}, fmt.Errorf("gemini returned an empty response")
	}

	// Extract the text part
	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return planner.LLMResponse{}, fmt.Errorf("expected text response from gemini, got %T", part)
	}

	var llmResp planner.LLMResponse
	if err := json.Unmarshal([]byte(text), &llmResp); err != nil {
		return planner.LLMResponse{}, fmt.Errorf("failed to unmarshal gemini json: %w\nResponse was: %s", err, text)
	}

	return llmResp, nil
}
