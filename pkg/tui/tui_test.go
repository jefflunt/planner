package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"planner/pkg/config"
	"planner/pkg/planner"
)

func TestUpdatePlansListOnEsc(t *testing.T) {
	// Setup temp directory
	tmpDir, err := os.MkdirTemp("", "plans-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy plan file
	planName := "new-plan"
	err = os.WriteFile(filepath.Join(tmpDir, planName+".json"), []byte("{}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	m := model{
		state: statePlanning,
		cfg:   &config.Config{PlansDir: tmpDir},
		plans: []string{"old-plan"},
	}

	// Trigger "esc"
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ := m.Update(msg)
	m = newModel.(model)

	// Verify plans list updated
	if len(m.plans) != 1 || m.plans[0] != planName {
		t.Errorf("Expected 1 plan (%s), got %v", planName, m.plans)
	}
}

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

func TestDetermineInitialState(t *testing.T) {
	tests := []struct {
		name        string
		planName    string
		initialTask string
		expected    uiState
	}{
		{
			name:        "Defaults to Select Plan view when no plan name or initial task",
			planName:    "",
			initialTask: "",
			expected:    stateSelectPlan,
		},
		{
			name:        "Starts with statePlanning if plan name is provided",
			planName:    "my-plan",
			initialTask: "",
			expected:    statePlanning,
		},
		{
			name:        "Starts with stateGeneratingPlanName if initial task is provided",
			planName:    "",
			initialTask: "build a web app",
			expected:    stateGeneratingPlanName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineInitialState(tt.planName, tt.initialTask)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
