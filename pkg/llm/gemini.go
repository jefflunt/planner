package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"planner/pkg/config"
	"planner/pkg/logger"
	"planner/pkg/planner"
	"planner/pkg/prompts"
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

func (g *GeminiClient) AnalyzeTask(ctx context.Context, req planner.LLMRequest) (planner.LLMResponse, error) {
	model := g.client.GenerativeModel(g.model)

	// Force JSON output
	model.ResponseMIMEType = "application/json"

	visionRule := ""
	if req.IsVision {
		visionRule = "\n\nCRITICAL: This task is the 'Vision' or 'Root' description of the project. It MUST be decomposed into smaller actionable steps, even if it seems simple. NEVER mark a root vision as 'actionable'."
	}

	var ancestryStr string
	if len(req.Ancestry) > 0 {
		ancestryStr = "\n\nCONTEXT:\nYou are working on a subtask of a larger project. Use the following ancestry to infer the programming language, framework, design patterns, and file structure. DO NOT ask the user for clarifications on details that can be reasonably inferred from this context.\n\nAncestry (Top-down):\n"
		for i, parent := range req.Ancestry {
			ancestryStr += fmt.Sprintf("%d. %s\n", i+1, parent)
		}
	}

	var fsStr string
	if req.FileSystemTree != "" {
		fsStr = fmt.Sprintf("\n\nFILE SYSTEM CONTEXT:\nYou are operating within an existing codebase. Here is the current file structure of the project:\n%s\n\nUse this to understand existing files, directories, and naming conventions. DO NOT ask the user for file paths or project structure if it can be inferred from this tree.", req.FileSystemTree)
	}

	prompt, err := prompts.Load("analyze_task", map[string]string{
		"VISION_RULE":  visionRule,
		"ANCESTRY_STR": ancestryStr,
		"FS_STR":       fsStr,
		"TASK":         req.Task,
	})
	if err != nil {
		return planner.LLMResponse{}, err
	}

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

func (c *GeminiClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
	model := c.client.GenerativeModel(c.model)
	model.ResponseMIMEType = "application/json"

	prompt, err := prompts.Load("generate_plan_name", map[string]string{
		"TASK": task,
	})
	if err != nil {
		return "", err
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))

	if err != nil {
		return "", fmt.Errorf("gemini generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned an empty response")
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return "", fmt.Errorf("expected text response from gemini, got %T", part)
	}

	var result struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal gemini json: %w\nResponse was: %s", err, text)
	}

	if result.Filename == "" {
		return "", fmt.Errorf("gemini returned an empty filename")
	}

	return result.Filename, nil
}

func (g *GeminiClient) ExecutePlan(ctx context.Context, plan string) (string, error) {
	model := g.client.GenerativeModel(g.model)
	// Force JSON output to improve stability, especially for complex prompts
	model.ResponseMIMEType = "application/json"

	prompt, err := prompts.Load("execute_plan", map[string]string{
		"PLAN": plan,
	})
	if err != nil {
		return "", err
	}

	logger.LogMsg(fmt.Sprintf("gemini execution prompt: %s", prompt))

	// DEBUG: Log the model configuration
	logger.LogMsg(fmt.Sprintf("DEBUG: Model config: %+v", model))

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		// Log the error with more context
		logger.LogMsg(fmt.Sprintf("gemini generation failed: %v", err))
		return "", fmt.Errorf("gemini generation failed: %w", err)
	}

	// Log raw response object to inspect structure
	logger.LogMsg(fmt.Sprintf("DEBUG: Raw Candidates: %+v", resp.Candidates))
	if len(resp.Candidates) > 0 {
		logger.LogMsg(fmt.Sprintf("DEBUG: Candidate 0: %+v", resp.Candidates[0]))
		if resp.Candidates[0].Content != nil {
			logger.LogMsg(fmt.Sprintf("DEBUG: Candidate 0 Content: %+v", resp.Candidates[0].Content))
		}
	}

	if len(resp.Candidates) == 0 {
		logger.LogMsg(fmt.Sprintf("gemini returned empty candidates: %+v, PromptFeedback: %+v", resp, resp.PromptFeedback))
		return "", fmt.Errorf("gemini returned an empty response (no candidates)")
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		candidate := resp.Candidates[0]
		// Log the entire response as JSON for debugging
		respJSON, _ := json.Marshal(resp)
		logger.LogMsg(fmt.Sprintf("gemini returned empty parts. Candidate: FinishReason=%v (%d), SafetyRatings=%+v, Index=%v. Full response: %s", candidate.FinishReason, int(candidate.FinishReason), candidate.SafetyRatings, candidate.Index, string(respJSON)))
		return "", fmt.Errorf("gemini returned an empty response (no parts)")
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return "", fmt.Errorf("expected text response from gemini, got %T", part)
	}

	return string(text), nil
}
