package tui

import (
	"strings"
	"testing"

	"planner/pkg/planner"
)

func TestRenderStatusBar_Version(t *testing.T) {
	m := model{
		state:   stateSelectPlan,
		version: "v1.2.3",
		width:   80,
	}

	result := renderStatusBar(m)

	if !strings.Contains(result, " v1.2.3 ") {
		t.Errorf("Expected status bar to contain ' v1.2.3 ', got: %s", result)
	}

	if strings.Contains(result, "version:") {
		t.Errorf("Expected status bar to NOT contain 'version:', got: %s", result)
	}
}

func TestRenderStatusBar_Pending(t *testing.T) {
	m := model{
		state: statePlanning,
		nodes: []*planner.Node{
			{Status: planner.StatusPending},
			{Status: planner.StatusPending},
			{Status: planner.StatusActionable},
		},
		width: 80,
	}

	result := renderStatusBar(m)

	if !strings.Contains(result, " pending:2 ") {
		t.Errorf("Expected status bar to contain ' pending:2 ', got: %s", result)
	}
}

func TestView_EmptyPlansList(t *testing.T) {
	m := model{
		state: stateSelectPlan,
		plans: []string{}, // Empty plans list
		width: 80,
	}

	result := m.View()

	if !strings.Contains(result, "No plans found") {
		t.Errorf("Expected view to contain 'No plans found', got:\n%s", result)
	}

	if !strings.Contains(result, "[Create New Plan]") {
		t.Errorf("Expected view to still contain '[Create New Plan]', got:\n%s", result)
	}
}

func TestView_PlansListWithItems(t *testing.T) {
	m := model{
		state: stateSelectPlan,
		plans: []string{"plan-A", "plan-B"},
		width: 80,
	}

	result := m.View()

	if strings.Contains(result, "No plans found") {
		t.Errorf("Expected view to NOT contain 'No plans found', got:\n%s", result)
	}

	if !strings.Contains(result, "plan-A") || !strings.Contains(result, "plan-B") {
		t.Errorf("Expected view to contain plan names, got:\n%s", result)
	}
}

func TestView_PlansListSearchEmpty(t *testing.T) {
	m := model{
		state:       stateSelectPlan,
		plans:       []string{"plan-A", "plan-B"},
		isSearching: true,
		searchQuery: "xyz",
		width:       80,
	}

	result := m.View()

	if !strings.Contains(result, "No plans match your search") {
		t.Errorf("Expected view to contain 'No plans match your search', got:\n%s", result)
	}
}
