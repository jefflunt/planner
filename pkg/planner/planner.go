package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"planner/pkg/atlassian"
	"planner/pkg/logger"
)

// Config represents the planner configuration
type Config struct {
	PlansDir       string
	StateFile      string
	Workspace      string // Directory to hold workspaces
	MaxConcurrency int
	MaxRetries     int
}

// ListPlans returns a list of available plan files in the PlansDir, without the .json extension.
func ListPlans(plansDir string) ([]string, error) {
	var plans []string

	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return plans, nil
		}
		return nil, fmt.Errorf("failed to read plans directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			name := strings.TrimSuffix(entry.Name(), ".json")
			plans = append(plans, name)
		}
	}

	return plans, nil
}

// DeletePlan removes the plan file from disk.
func DeletePlan(plansDir string, name string) error {
	path := filepath.Join(plansDir, name+".json")
	return os.Remove(path)
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
	mu           sync.RWMutex
	saveMu       sync.Mutex
	Root         *Node           `json:"root"`
	Config       Config          `json:"config"`
	LLM          LLMClient       `json:"-"`
	Prompts      chan UserPrompt `json:"-"` // Channel to request user input
	llmSemaphore chan struct{}   `json:"-"`
}

// NewPlanner creates a new planner instance
func NewPlanner(cfg Config, llm LLMClient) *Planner {
	if cfg.MaxConcurrency == 0 {
		cfg.MaxConcurrency = 4
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	return &Planner{
		Config:       cfg,
		LLM:          llm,
		Prompts:      make(chan UserPrompt),
		llmSemaphore: make(chan struct{}, cfg.MaxConcurrency),
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

// SerializePlan returns the plan as a string representation.
func (p *Planner) SerializePlan() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serializeNode(p.Root, 0)
}

func (p *Planner) serializeNode(n *Node, depth int) string {
	if n == nil {
		return ""
	}
	indent := strings.Repeat("  ", depth)
	res := fmt.Sprintf("%s- %s [%s]\n", indent, n.Task, n.Status)
	for _, child := range n.Children {
		res += p.serializeNode(child, depth+1)
	}
	return res
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

	p.saveMu.Lock()
	defer p.saveMu.Unlock()
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

	err := p.Plan(ctx, p.Root)
	p.Save()
	return err
}

// analyzeTaskWithRetry wraps the LLM call with a semaphore and retry logic.
func (p *Planner) analyzeTaskWithRetry(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	p.llmSemaphore <- struct{}{}
	defer func() { <-p.llmSemaphore }()

	var lastErr error
	for i := 0; i <= p.Config.MaxRetries; i++ {
		resp, err := p.LLM.AnalyzeTask(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond) // Simple backoff
	}
	return LLMResponse{}, fmt.Errorf("failed after %d retries: %w", p.Config.MaxRetries, lastErr)
}

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

// ReplanNode clears a node's children, resets its status, and saves.
func (p *Planner) ReplanNode(id string) (*Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	node := p.Root.Find(id)
	if node == nil {
		return nil, fmt.Errorf("node not found")
	}

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

// AddSibling adds a new node immediately before or after the specified sibling.
func (p *Planner) AddSibling(siblingID string, task string, before bool) (*Node, error) {
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

	// Insert before or after siblingIdx
	newChildren := make([]*Node, 0, len(parent.Children)+1)
	if before {
		newChildren = append(newChildren, parent.Children[:siblingIdx]...)
		newChildren = append(newChildren, newNode)
		newChildren = append(newChildren, parent.Children[siblingIdx:]...)
	} else {
		newChildren = append(newChildren, parent.Children[:siblingIdx+1]...)
		newChildren = append(newChildren, newNode)
		newChildren = append(newChildren, parent.Children[siblingIdx+1:]...)
	}
	parent.Children = newChildren

	if err := p.saveUnlocked(); err != nil {
		return nil, err
	}

	return newNode, nil
}

// InsertParent inserts a new node directly above the target node.
// The target node becomes the only child of the new node.
func (p *Planner) InsertParent(targetID string, task string) (*Node, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	target := p.Root.Find(targetID)
	if target == nil {
		return nil, fmt.Errorf("target node not found")
	}

	newNode := &Node{
		ID:       uuid.New().String(),
		Task:     task,
		Status:   StatusComposite,
		Type:     TaskTypeComposite,
		Children: []*Node{target},
	}

	parent := p.findParent(p.Root, targetID)
	if parent == nil {
		// target is root
		if p.Root.ID != targetID {
			return nil, fmt.Errorf("internal error: target is not root but has no parent")
		}
		newNode.Depth = 0
		target.ParentID = newNode.ID
		p.Root = newNode
	} else {
		// target is not root
		newNode.ParentID = parent.ID
		newNode.Depth = parent.Depth + 1

		// replace target in parent.Children with newNode
		for i, child := range parent.Children {
			if child.ID == targetID {
				parent.Children[i] = newNode
				break
			}
		}
		target.ParentID = newNode.ID
	}

	// update depths recursively
	p.incrementDepth(target)

	if err := p.saveUnlocked(); err != nil {
		return nil, err
	}

	return newNode, nil
}

func (p *Planner) incrementDepth(n *Node) {
	if n == nil {
		return
	}
	n.Depth++
	for _, child := range n.Children {
		p.incrementDepth(child)
	}
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

	// 1. Fetch Atlassian Content if URLs present
	if p.Config.Atlassian.BaseURL != "" && p.Config.Atlassian.APIKey != "" {
		client := atlassian.NewClient(p.Config.Atlassian.BaseURL, p.Config.Atlassian.User, p.Config.Atlassian.APIKey)

		textToScan := node.Task + "\n" + node.Details

		// Simple URL detection
		lines := strings.Split(textToScan, " ")
		for _, token := range lines {
			if strings.Contains(token, p.Config.Atlassian.BaseURL) {
				// Clean URL (remove trailing punctuation)
				url := strings.TrimRight(token, ".,!?)")
				content, err := client.Fetch(url)
				if err == nil {
					p.mu.Lock()
					node.Details = fmt.Sprintf("%s\n\n[Atlassian Context from %s]:\n%s", node.Details, url, content)
					p.mu.Unlock()
				}
			}
		}
	}

	// If already fully analyzed, just skip or recurse
	if status == StatusActionable {
		return nil
	}
	if status == StatusComposite {
		g, gCtx := errgroup.WithContext(ctx)
		for _, child := range children {
			c := child
			g.Go(func() error {
				if err := p.Plan(gCtx, c); err != nil {
					p.mu.Lock()
					c.Status = StatusError
					c.Task = "!" + c.Task
					p.mu.Unlock()
					_ = logger.Log(err)
				}
				return nil
			})
		}
		return g.Wait()
	}

	pwd, _ := os.Getwd()
	fsTree := GetFileSystemTree(pwd)

	for {
		p.mu.Lock()
		node.Status = StatusPending
		p.mu.Unlock()

		ancestry := p.GetAncestry(node)

		req := LLMRequest{
			Task:           node.Task,
			Ancestry:       ancestry,
			IsVision:       isRoot,
			FileSystemTree: fsTree,
		}

		// Ask LLM what to do
		resp, err := p.analyzeTaskWithRetry(ctx, req)
		if err != nil {
			p.mu.Lock()
			node.Status = StatusError
			node.Task = "!" + node.Task
			p.mu.Unlock()
			_ = logger.Log(err)
			return fmt.Errorf("failed to analyze task %q: %w", node.Task, err)
		}

		p.mu.Lock()
		if resp.RewrittenTask != "" && resp.RewrittenTask != node.Task {
			node.Task = resp.RewrittenTask
		}
		if resp.Title != "" {
			node.Title = resp.Title
		}
		if resp.Details != "" {
			node.Details = resp.Details
		}
		if resp.AsciiDiagram != "" {
			node.AsciiDiagram = resp.AsciiDiagram
		}
		p.mu.Unlock()

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
			g, gCtx := errgroup.WithContext(ctx)
			p.mu.RLock()
			currentChildren := node.Children
			p.mu.RUnlock()
			for _, child := range currentChildren {
				c := child
				g.Go(func() error {
					if err := p.Plan(gCtx, c); err != nil {
						p.mu.Lock()
						c.Status = StatusError
						c.Task = "!" + c.Task
						p.mu.Unlock()
						_ = logger.Log(err)
					}
					return nil
				})
			}
			return g.Wait()

		case ActionAskUser:
			p.mu.Lock()
			node.Status = StatusNeedsInput
			p.mu.Unlock()

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

// GetExecCommand gathers the node's context and the overall plan structure,
// then returns an un-started exec.Cmd that will execute the plan natively.
func (p *Planner) GetExecCommand(ctx context.Context, nodeID string) (*exec.Cmd, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Root == nil {
		return nil, fmt.Errorf("no active plan")
	}

	node := p.Root.Find(nodeID)
	if node == nil {
		return nil, fmt.Errorf("node not found")
	}

	req := ExecRequest{
		Task:          node.Task,
		Details:       node.Details,
		AsciiDiagram:  node.AsciiDiagram,
		PlanStructure: p.Root.FormatPlanStructure(0),
	}

	return p.LLM.GetExecCommand(ctx, req)
}
