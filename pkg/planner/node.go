package planner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// TaskType represents whether a task can be broken down further.
type TaskType string

const (
	TaskTypeAtomic    TaskType = "atomic"
	TaskTypeComposite TaskType = "composite"
)

// NodeStatus tracks the execution state of a node.
type NodeStatus string

const (
	StatusPending    NodeStatus = "pending"
	StatusActionable NodeStatus = "actionable"
	StatusComposite  NodeStatus = "composite"
	StatusNeedsInput NodeStatus = "needs_input"
	StatusError      NodeStatus = "error"
)

// Node represents a single task in the task tree.
type Node struct {
	ID           string     `json:"id"`
	ParentID     string     `json:"parent_id,omitempty"`
	Task         string     `json:"task"` // A high-level goal that can be up to a paragraph in length
	Title        string     `json:"title,omitempty"`
	Details      string     `json:"details,omitempty"`
	AsciiDiagram string     `json:"ascii_diagram,omitempty"`
	Type         TaskType   `json:"type"`
	Status       NodeStatus `json:"status"`
	Children     []*Node    `json:"children,omitempty"`
	Depth        int        `json:"depth"`
	Dependencies []string   `json:"dependencies,omitempty"`
}

// PlanAction represents the next step for a node according to the LLM.
type PlanAction string

const (
	ActionActionable PlanAction = "actionable"
	ActionDecompose  PlanAction = "decompose"
	ActionAskUser    PlanAction = "ask_user"
)

// LLMResponse is the structured output from the LLM analysis.
type LLMResponse struct {
	Action        PlanAction `json:"action"`
	Title         string     `json:"title,omitempty"`          // Short description (< 20 words) of the node
	Details       string     `json:"details,omitempty"`        // Multiple paragraphs explaining the task in depth
	AsciiDiagram  string     `json:"ascii_diagram,omitempty"`  // Text-based diagram of interactions
	Subtasks      []string   `json:"subtasks,omitempty"`       // Populated if Action == Decompose
	Question      string     `json:"question,omitempty"`       // Populated if Action == AskUser
	Reasoning     string     `json:"reasoning,omitempty"`      // Why the LLM made this choice
	RewrittenTask string     `json:"rewritten_task,omitempty"` // If the task contains clarification, this is the rewritten version
}

// LLMRequest contains the task to analyze and its context within the project tree.
type LLMRequest struct {
	Task           string
	Ancestry       []string // The chain of parent tasks, from Root down to the immediate parent
	IsVision       bool     // True if this is the root vision of the project
	FileSystemTree string   // List of files in the current working directory to provide codebase context
}

// ExecRequest contains the context of the current node and the overall plan structure.
type ExecRequest struct {
	Task          string
	Details       string
	AsciiDiagram  string
	PlanStructure string
}

// LLMClient represents an abstract interface for the LLM to classify and decompose tasks.
type LLMClient interface {
	// AnalyzeTask evaluates a task to determine if it's actionable (single file operation),
	// if it needs decomposition, or if it requires user clarification.
	AnalyzeTask(ctx context.Context, req LLMRequest) (LLMResponse, error)

	// GeneratePlanName creates a short, descriptive, unique filename (without extension) for a new plan based on the task description.
	GeneratePlanName(ctx context.Context, task string) (string, error)

	// GetExecCommand returns an un-started exec.Cmd that will execute the plan natively in the terminal.
	GetExecCommand(ctx context.Context, req ExecRequest) (*exec.Cmd, error)
}

// IsLeaf returns true if the node is atomic and has no children.
func (n *Node) IsLeaf() bool {
	return n.Type == TaskTypeAtomic && len(n.Children) == 0
}

// Find finds a node by ID anywhere in the tree under this node.
func (n *Node) Find(id string) *Node {
	if n.ID == id {
		return n
	}
	for _, child := range n.Children {
		if found := child.Find(id); found != nil {
			return found
		}
	}
	return nil
}

// LeafNodes returns all atomic nodes under this node.
func (n *Node) LeafNodes() []*Node {
	if n.IsLeaf() {
		return []*Node{n}
	}
	var leaves []*Node
	for _, child := range n.Children {
		leaves = append(leaves, child.LeafNodes()...)
	}
	return leaves
}

// FormatPlanStructure recursively serializes the entire plan tree into a formatted text representation.
func (n *Node) FormatPlanStructure(depth int) string {
	if n == nil {
		return ""
	}
	indent := strings.Repeat("  ", depth)
	res := fmt.Sprintf("%s- %s [%s]\n", indent, n.Task, n.Status)
	for _, child := range n.Children {
		res += child.FormatPlanStructure(depth + 1)
	}
	return res
}
