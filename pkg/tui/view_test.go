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
