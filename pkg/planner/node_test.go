package planner

import (
	"testing"
)

func TestNodeIsLeaf(t *testing.T) {
	t.Run("Actionable node with no children is a leaf", func(t *testing.T) {
		n := &Node{
			Type:     TaskTypeAtomic,
			Status:   StatusActionable,
			Children: []*Node{},
		}
		if !n.IsLeaf() {
			t.Errorf("Expected node to be a leaf")
		}
	})

	t.Run("Composite node is not a leaf", func(t *testing.T) {
		n := &Node{
			Type:     TaskTypeComposite,
			Children: []*Node{},
		}
		if n.IsLeaf() {
			t.Errorf("Expected composite node to NOT be a leaf")
		}
	})

	t.Run("Atomic node with children is not a leaf", func(t *testing.T) {
		n := &Node{
			Type: TaskTypeAtomic,
			Children: []*Node{
				{Type: TaskTypeAtomic},
			},
		}
		if n.IsLeaf() {
			t.Errorf("Expected atomic node with children to NOT be a leaf")
		}
	})
}

func TestNodeFind(t *testing.T) {
	child := &Node{ID: "child-1", Task: "Child Task"}
	root := &Node{
		ID:   "root-1",
		Task: "Root Task",
		Children: []*Node{
			{
				ID: "intermediate-1",
				Children: []*Node{
					child,
				},
			},
		},
	}

	t.Run("Finds itself", func(t *testing.T) {
		found := root.Find("root-1")
		if found != root {
			t.Errorf("Expected to find root node")
		}
	})

	t.Run("Finds deep child", func(t *testing.T) {
		found := root.Find("child-1")
		if found != child {
			t.Errorf("Expected to find child node")
		}
	})

	t.Run("Returns nil for non-existent ID", func(t *testing.T) {
		found := root.Find("does-not-exist")
		if found != nil {
			t.Errorf("Expected nil when searching for non-existent ID")
		}
	})
}

func TestNodeLeafNodes(t *testing.T) {
	leaf1 := &Node{ID: "leaf-1", Type: TaskTypeAtomic}
	leaf2 := &Node{ID: "leaf-2", Type: TaskTypeAtomic}
	intermediate := &Node{
		ID:       "inter",
		Type:     TaskTypeComposite,
		Children: []*Node{leaf2},
	}
	root := &Node{
		ID:       "root",
		Type:     TaskTypeComposite,
		Children: []*Node{leaf1, intermediate},
	}

	t.Run("Returns all leaves from composite root", func(t *testing.T) {
		leaves := root.LeafNodes()
		if len(leaves) != 2 {
			t.Fatalf("Expected 2 leaves, got %d", len(leaves))
		}
		if leaves[0].ID != "leaf-1" && leaves[1].ID != "leaf-1" {
			t.Errorf("Expected leaf-1 in result")
		}
		if leaves[0].ID != "leaf-2" && leaves[1].ID != "leaf-2" {
			t.Errorf("Expected leaf-2 in result")
		}
	})

	t.Run("Returns itself if called on leaf", func(t *testing.T) {
		leaves := leaf1.LeafNodes()
		if len(leaves) != 1 || leaves[0].ID != "leaf-1" {
			t.Errorf("Expected leaf to return itself")
		}
	})
}
