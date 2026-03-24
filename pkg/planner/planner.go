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

func (p *Planner) RLock() {
	p.mu.RLock()
}

func (p *Planner) RUnlock() {
	p.mu.RUnlock()
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
	return p.saveUnlocked()
}

func (p *Planner) saveUnlocked() error {
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

// EditNode updates a node's task string, clears its children, and resets its status.
func (p *Planner) EditNode(id string, newTask string) (*Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	node := p.Root.Find(id)
	if node == nil {
		return nil, fmt.Errorf("node not found")
	}

	node.Task = newTask
	node.Status = StatusPending
	node.Type = ""
	node.Children = nil

	if err := p.saveUnlocked(); err != nil {
		return nil, err
	}

	return node, nil
}

// AddChild adds a new child node to the specified parent.
func (p *Planner) AddChild(parentID string, task string) (*Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	parent := p.Root.Find(parentID)
	if parent == nil {
		return nil, fmt.Errorf("parent node not found")
	}

	parent.Type = TaskTypeComposite
	parent.Status = StatusComposite

	child := &Node{
		ID:       uuid.New().String(),
		ParentID: parent.ID,
		Task:     task,
		Status:   StatusPending,
		Depth:    parent.Depth + 1,
	}

	parent.Children = append(parent.Children, child)

	if err := p.saveUnlocked(); err != nil {
		return nil, err
	}

	return child, nil
}

// AddSibling adds a new node immediately after the specified sibling.
func (p *Planner) AddSibling(siblingID string, task string) (*Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	if p.Root.ID == siblingID {
		return nil, fmt.Errorf("cannot add sibling to root node")
	}

	parent := p.findParent(p.Root, siblingID)
	if parent == nil {
		return nil, fmt.Errorf("parent not found for sibling")
	}

	siblingIdx := -1
	for i, child := range parent.Children {
		if child.ID == siblingID {
			siblingIdx = i
			break
		}
	}

	if siblingIdx == -1 {
		return nil, fmt.Errorf("sibling not found in parent's children")
	}

	newNode := &Node{
		ID:       uuid.New().String(),
		ParentID: parent.ID,
		Task:     task,
		Status:   StatusPending,
		Depth:    parent.Depth + 1,
	}

	// Insert after siblingIdx
	parent.Children = append(parent.Children[:siblingIdx+1], append([]*Node{newNode}, parent.Children[siblingIdx+1:]...)...)

	if err := p.saveUnlocked(); err != nil {
		return nil, err
	}

	return newNode, nil
}

// DeleteNode removes a node and all its children from the tree.
func (p *Planner) DeleteNode(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil
	}

	if p.Root.ID == id {
		p.Root = nil
		return p.saveUnlocked() // Call save unlocked to prevent deadlock
	}

	parent := p.findParent(p.Root, id)
	if parent != nil {
		for i, child := range parent.Children {
			if child.ID == id {
				parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
				break
			}
		}
	}
	return p.saveUnlocked()
}

func (p *Planner) findParent(current *Node, childID string) *Node {
	for _, child := range current.Children {
		if child.ID == childID {
			return current
		}
		if found := p.findParent(child, childID); found != nil {
			return found
		}
	}
	return nil
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

// GetAncestry returns a list of parent tasks, from Root down to the immediate parent of the given node.
func (p *Planner) GetAncestry(node *Node) []string {
	var ancestry []string
	current := node

	for current != nil && current.ParentID != "" {
		parent := p.Find(current.ParentID)
		if parent == nil {
			break
		}
		// Prepend
		ancestry = append([]string{parent.Task}, ancestry...)
		current = parent
	}
	return ancestry
}

// Plan recursively decomposes a node, polling the LLM and user until Actionable
func (p *Planner) Plan(ctx context.Context, node *Node) error {
	p.mu.RLock()
	status := node.Status
	children := node.Children
	isRoot := p.Root != nil && node.ID == p.Root.ID
	p.mu.RUnlock()

	// If already fully analyzed, just skip or recurse
	if status == StatusActionable {
		return nil
	}
	if status == StatusComposite {
		for _, child := range children {
			if err := p.Plan(ctx, child); err != nil {
				return err
			}
		}
		return nil
	}

	pwd, _ := os.Getwd()
	fsTree := GetFileSystemTree(pwd)

	for {
		p.mu.Lock()
		node.Status = StatusPending
		p.mu.Unlock()
		p.Save()

		ancestry := p.GetAncestry(node)

		req := LLMRequest{
			Task:           node.Task,
			Ancestry:       ancestry,
			IsVision:       isRoot,
			FileSystemTree: fsTree,
		}

		// Ask LLM what to do
		resp, err := p.LLM.AnalyzeTask(ctx, req)
		if err != nil {
			p.mu.Lock()
			node.Status = StatusError
			p.mu.Unlock()
			p.Save()
			return fmt.Errorf("failed to analyze task %q: %w", node.Task, err)
		}

		if resp.RewrittenTask != "" && resp.RewrittenTask != node.Task {
			p.mu.Lock()
			node.Task = resp.RewrittenTask
			p.mu.Unlock()
			p.Save()
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
			p.mu.RLock()
			currentChildren := node.Children
			p.mu.RUnlock()
			for _, child := range currentChildren {
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
