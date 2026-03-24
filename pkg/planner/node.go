package planner

import (
	"context"
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
	Task         string     `json:"task"`
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
	Action    PlanAction `json:"action"`
	Subtasks  []string   `json:"subtasks,omitempty"`  // Populated if Action == Decompose
	Question  string     `json:"question,omitempty"`  // Populated if Action == AskUser
	Reasoning string     `json:"reasoning,omitempty"` // Why the LLM made this choice
}

// LLMClient represents an abstract interface for the LLM to classify and decompose tasks.
type LLMClient interface {
	// AnalyzeTask evaluates a task to determine if it's actionable (single file operation),
	// if it needs decomposition, or if it requires user clarification.
	// If isVision is true, the LLM will treat the task as a high-level project vision that MUST be decomposed.
	AnalyzeTask(ctx context.Context, task string, isVision bool) (LLMResponse, error)
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
