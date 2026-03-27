package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"planner/pkg/config"
	"planner/pkg/logger"
	"planner/pkg/planner"
	"planner/prompts"
)

type ClaudeClient struct {
	model  string
	apiKey string
	client *http.Client
}

func NewClaudeClient(ctx context.Context, cfg *config.Config) (*ClaudeClient, error) {
	apiKey := cfg.LLM.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("anthropic api key is required in config or via ANTHROPIC_API_KEY environment variable")
	}

	model := cfg.LLM.Model
	if model == "" {
		model = "claude-3-5-sonnet-latest" // Sensible default
	}

	return &ClaudeClient{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{},
	}, nil
}

func (c *ClaudeClient) callAPI(ctx context.Context, prompt string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	payload := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode anthropic response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from anthropic api")
	}

	return result.Content[0].Text, nil
}

func (c *ClaudeClient) AnalyzeTask(ctx context.Context, req planner.LLMRequest) (planner.LLMResponse, error) {
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

	output, err := c.callAPI(ctx, prompt)
	if err != nil {
		return planner.LLMResponse{}, err
	}

	output = extractJSON(output)

	var llmResp planner.LLMResponse
	if err := json.Unmarshal([]byte(output), &llmResp); err != nil {
		return planner.LLMResponse{}, fmt.Errorf("failed to unmarshal claude json: %w\nResponse was: %s", err, output)
	}

	return llmResp, nil
}

func (c *ClaudeClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
	prompt, err := prompts.Load("generate_plan_name", map[string]string{
		"TASK": task,
	})
	if err != nil {
		return "", err
	}

	output, err := c.callAPI(ctx, prompt)
	if err != nil {
		return "", err
	}

	output = extractJSON(output)

	var result struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal claude json: %w\nResponse was: %s", err, output)
	}

	if result.Filename == "" {
		return "", fmt.Errorf("claude returned an empty filename")
	}

	return result.Filename, nil
}

func (c *ClaudeClient) GetExecCommand(ctx context.Context, req planner.ExecRequest) (*exec.Cmd, error) {
	prompt, err := prompts.Load("execute_plan", map[string]string{
		"TASK":           req.Task,
		"DETAILS":        req.Details,
		"ASCII_DIAGRAM":  req.AsciiDiagram,
		"PLAN_STRUCTURE": req.PlanStructure,
	})
	if err != nil {
		return nil, err
	}

	logger.LogMsg(fmt.Sprintf("claude execution prompt: %s", prompt))

	cmdStr := "claude"
	args := []string{prompt}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	cmd := exec.CommandContext(ctx, cmdStr, args...)
	return cmd, nil
}
