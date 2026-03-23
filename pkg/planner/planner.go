package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// Config represents the planner configuration
type Config struct {
	StateFile string
	Workspace string // Directory to hold workspaces
}

// UserPrompt allows the planner to bubble up a question to the UI and wait for a reply
type UserPrompt struct {
	NodeID    string
	Task      string
	Question  string
	ReplyChan chan string
}

// Planner is the central orchestrator for the task tree
type Planner struct {
	mu      sync.RWMutex
	Root    *Node           `json:"root"`
	Config  Config          `json:"config"`
	LLM     LLMClient       `json:"-"`
	Prompts chan UserPrompt `json:"-"` // Channel to request user input
}

// NewPlanner creates a new planner instance
func NewPlanner(cfg Config, llm LLMClient) *Planner {
	return &Planner{
		Config:  cfg,
		LLM:     llm,
		Prompts: make(chan UserPrompt),
	}
}

// Load loads the planner state from disk
func (p *Planner) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := os.ReadFile(p.Config.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing state
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	return json.Unmarshal(data, p)
}

// Save persists the planner state to disk
func (p *Planner) Save() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dir := filepath.Dir(p.Config.StateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return os.WriteFile(p.Config.StateFile, data, 0644)
}

// Start initiates the planning process for a root task
func (p *Planner) Start(ctx context.Context, task string) error {
	p.mu.Lock()
	p.Root = &Node{
		ID:     uuid.New().String(),
		Task:   task,
		Status: StatusPending,
		Depth:  0,
	}
	p.mu.Unlock()

	if err := p.Save(); err != nil {
		return err
	}

	return p.Plan(ctx, p.Root)
}

// Plan recursively decomposes a node, polling the LLM and user until Actionable
func (p *Planner) Plan(ctx context.Context, node *Node) error {
	for {
		// Ask LLM what to do
		resp, err := p.LLM.AnalyzeTask(ctx, node.Task)
		if err != nil {
			p.mu.Lock()
			node.Status = StatusError
			p.mu.Unlock()
			p.Save()
			return fmt.Errorf("failed to analyze task %q: %w", node.Task, err)
		}

		switch resp.Action {
		case ActionActionable:
			p.mu.Lock()
			node.Type = TaskTypeAtomic
			node.Status = StatusActionable
			p.mu.Unlock()
			p.Save()
			return nil // Branch terminates successfully

		case ActionDecompose:
			p.mu.Lock()
			node.Type = TaskTypeComposite
			node.Status = StatusComposite
			for _, st := range resp.Subtasks {
				child := &Node{
					ID:       uuid.New().String(),
					ParentID: node.ID,
					Task:     st,
					Status:   StatusPending,
					Depth:    node.Depth + 1,
				}
				node.Children = append(node.Children, child)
			}
			p.mu.Unlock()
			p.Save()

			// Recursively plan children
			for _, child := range node.Children {
				if err := p.Plan(ctx, child); err != nil {
					return err
				}
			}
			return nil // Finished decomposing and planning children

		case ActionAskUser:
			p.mu.Lock()
			node.Status = StatusNeedsInput
			p.mu.Unlock()
			p.Save()

			// Block and ask user for clarification
			replyChan := make(chan string)
			prompt := UserPrompt{
				NodeID:    node.ID,
				Task:      node.Task,
				Question:  resp.Question,
				ReplyChan: replyChan,
			}

			// Send the prompt to the UI (blocking until the UI reads it)
			select {
			case p.Prompts <- prompt:
			case <-ctx.Done():
				return ctx.Err()
			}

			// Wait for UI to reply
			var answer string
			select {
			case answer = <-replyChan:
			case <-ctx.Done():
				return ctx.Err()
			}

			// Append context and retry loop
			p.mu.Lock()
			node.Task = fmt.Sprintf("%s\n\n[Clarification]: %s", node.Task, answer)
			node.Status = StatusPending
			p.mu.Unlock()
			p.Save()
			// The loop will continue, passing the augmented task back to the LLM
		}
	}
}

// Find finds a node by its ID
func (p *Planner) Find(id string) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Root == nil {
		return nil
	}
	return p.Root.Find(id)
}
