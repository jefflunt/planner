package tui

import (
	"testing"

	"planner/pkg/planner"
)

func TestFlattenTree(t *testing.T) {
	root := &planner.Node{
		ID: "root",
		Children: []*planner.Node{
			{ID: "child-1"},
			{ID: "child-2", Children: []*planner.Node{
				{ID: "grandchild-1"},
			}},
		},
	}

	nodes := flattenTree(root)
	if len(nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(nodes))
	}
}

func TestGetFilteredPlans(t *testing.T) {
	m := &model{
		plans:       []string{"plan1", "plan2", "other"},
		searchQuery: "pl",
	}

	filtered := getFilteredPlans(m)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered plans, got %d", len(filtered))
	}
}
