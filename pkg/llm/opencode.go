package llm

import (
	"bufio"
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

type opencodeEvent struct {
	Type string `json:"type"`
	Part struct {
		Text string `json:"text,omitempty"`
	} `json:"part"`
}

type OpencodeClient struct {
	model  string
	runner func(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
}

func NewOpencodeClient(ctx context.Context, cfg *config.Config) (*OpencodeClient, error) {
	// Verify that the opencode executable is available
	if _, err := exec.LookPath("opencode"); err != nil {
		return nil, fmt.Errorf("opencode command line interface not found in PATH: %w", err)
	}

	model := cfg.LLM.Model
	// If no model is specified, we can leave it empty to let the CLI choose the default

	return &OpencodeClient{
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

func (c *OpencodeClient) executePrompt(ctx context.Context, prompt string) (string, error) {
	logger.LogMsg("executePrompt called")
	// Use 'run' subcommand for messages
	args := []string{"run", prompt, "--format", "json"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	out, stderr, err := c.runner(ctx, "opencode", args...)
	if err != nil {
		return "", fmt.Errorf("opencode cli failed: %w\nstderr: %s", err, string(stderr))
	}
	logger.LogMsg(fmt.Sprintf("opencode output: %s", string(out)))
	logger.LogMsg(fmt.Sprintf("opencode stderr: %s", string(stderr)))

	// Parse stream line-by-line
	var fullText strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var event opencodeEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			if event.Type == "text" && event.Part.Text != "" {
				fullText.WriteString(event.Part.Text)
			}
		}
	}

	// Extract JSON block if opencode included other text
	return extractJSON(fullText.String()), nil
}

func (c *OpencodeClient) AnalyzeTask(ctx context.Context, req planner.LLMRequest) (planner.LLMResponse, error) {
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
		return planner.LLMResponse{}, fmt.Errorf("failed to unmarshal opencode json: %w\nResponse was: %s", err, output)
	}

	return llmResp, nil
}

func (c *OpencodeClient) GeneratePlanName(ctx context.Context, task string) (string, error) {
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
		return "", fmt.Errorf("failed to unmarshal opencode json: %w\nResponse was: %s", err, output)
	}

	if result.Filename == "" {
		return "", fmt.Errorf("opencode returned an empty filename")
	}

	return result.Filename, nil
}

func (c *OpencodeClient) GetExecCommand(ctx context.Context, req planner.ExecRequest) (*exec.Cmd, error) {
	prompt, err := prompts.Load("execute_plan", map[string]string{
		"TASK":           req.Task,
		"DETAILS":        req.Details,
		"ASCII_DIAGRAM":  req.AsciiDiagram,
		"PLAN_STRUCTURE": req.PlanStructure,
	})
	if err != nil {
		return nil, err
	}

	logger.LogMsg(fmt.Sprintf("opencode execution prompt: %s", prompt))

	args := []string{".", "--prompt", prompt}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	cmd := exec.CommandContext(ctx, "opencode", args...)
	return cmd, nil
}
