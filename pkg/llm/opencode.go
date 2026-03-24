package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"planner/pkg/config"
	"planner/pkg/planner"
)

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
	// Use 'run' subcommand for messages
	args := []string{"run", prompt, "--format", "json"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	out, stderr, err := c.runner(ctx, "opencode", args...)
	if err != nil {
		return "", fmt.Errorf("opencode cli failed: %w\nstderr: %s", err, string(stderr))
	}

	output := strings.TrimSpace(string(out))

	// Extract JSON block if opencode included other text
	output = extractJSON(output)

	return output, nil
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

	prompt := fmt.Sprintf(`You are an expert agentic task orchestrator. Your job is to analyze a task and decide whether it is actionable, requires decomposition, or needs clarification from the user.
	
CRITICAL RULE (Actionable Heuristic): 
A task is ONLY "actionable" if it describes the creation, deletion, or editing of ONE SINGLE FILE on disk. 
- Example: "Refactor the authentication module" -> Not Actionable (Too vague, multiple files).
- Example: "Rename AuthUser to SessionUser in src/auth/models.go" -> Actionable (Single file operation).

If a task is too large or modifies multiple files (e.g. "Rename type X and all references"), you MUST decompose it into multiple actionable steps.%s%s%s

Analyze this task:
"""
%s
"""

Respond with a JSON object containing:
1. "action": Must be exactly one of "actionable", "decompose", or "ask_user".
2. "reasoning": A brief explanation of why you chose this action.
3. "subtasks": If action is "decompose", provide a JSON array of strings, where each string is a smaller, more specific subtask.
4. "question": If action is "ask_user", provide the clarification question you want to ask the user.
5. "rewritten_task": If the task contains appended clarifications from the user (e.g. "[Clarification]: ..."), rewrite the entire task to incorporate the clarification into a single coherent task description, and provide it here. Otherwise, you can omit this field.

JSON Format:
{
  "action": "...",
  "reasoning": "...",
  "subtasks": [...],
  "question": "...",
  "rewritten_task": "..."
}`, visionRule, ancestryStr, fsStr, req.Task)

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
	prompt := fmt.Sprintf(`You are an assistant that creates short, descriptive, unique filenames for task plans.
Given the following task description, generate a short filename (kebab-case, max 5-6 words) without any file extension.

Task:
"""
%s
"""

Respond with a JSON object containing a single key "filename" with your chosen name.
Example: {"filename": "add-user-auth-system"}`, task)

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
