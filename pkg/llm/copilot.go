package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"planner/pkg/config"
	"planner/pkg/logger"
	"planner/pkg/planner"
	"planner/prompts"
)

type CopilotClient struct {
	model  string
	runner func(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
}

func NewCopilotClient(ctx context.Context, cfg *config.Config) (*CopilotClient, error) {
	// Verify that the copilot executable is available
	if _, err := exec.LookPath("copilot"); err != nil {
		return nil, fmt.Errorf("copilot command line interface not found in PATH: %w", err)
	}

	model := cfg.LLM.Model
	// If no model is specified, we can leave it empty to let the CLI choose the default

	return &CopilotClient{
		model: model,
		runner: func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			return out.Bytes(), stderr.Bytes(), err
		},
	}, nil
}

func (c *CopilotClient) executePrompt(ctx context.Context, prompt string) (string, error) {
	args := []string{"-s", "-p", prompt, "--excluded-tools=*"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	out, stderr, err := c.runner(ctx, "copilot", args...)
	if err != nil {
		return "", fmt.Errorf("copilot cli failed: %w\nstderr: %s", err, string(stderr))
	}

	output := strings.TrimSpace(string(out))

	// Extract JSON block if copilot included other text
	output = extractJSON(output)

	return output, nil
}

func (c *CopilotClient) AnalyzeTask(ctx context.Context, req planner.LLMRequest) (planner.LLMResponse, error) {
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

	output, err := c.executePrompt(ctx, prompt)

	if err != nil {
		return planner.LLMResponse{}, err
	}

	var llmResp planner.LLMResponse
	if err := json.Unmarshal([]byte(output), &llmResp); err != nil {
		return planner.LLMResponse{}, fmt.Errorf("failed to unmarshal copilot json: %w\nResponse was: %s", err, output)
	}

	return llmResp, nil
}

func (c *CopilotClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
	prompt, err := prompts.Load("generate_plan_name", map[string]string{
		"TASK": task,
	})
	if err != nil {
		return "", err
	}

	output, err := c.executePrompt(ctx, prompt)
	if err != nil {
		return "", err
	}

	var result struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal copilot json: %w\nResponse was: %s", err, output)
	}

	if result.Filename == "" {
		return "", fmt.Errorf("copilot returned an empty filename")
	}

	return result.Filename, nil
}

func (c *CopilotClient) GetExecCommand(ctx context.Context, plan string) (*exec.Cmd, error) {
	prompt, err := prompts.Load("execute_plan", map[string]string{
		"PLAN": plan,
	})
	if err != nil {
		return nil, err
	}

	logger.LogMsg(fmt.Sprintf("copilot execution prompt: %s", prompt))

	args := []string{"-s", "-p", prompt}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	cmd := exec.CommandContext(ctx, "copilot", args...)
	return cmd, nil
}
