package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"planner/pkg/config"
	"planner/pkg/planner"
)

func StartTUI(planName string, initialTask string, cfg *config.Config, workspace string, client planner.LLMClient) error {
	// Context for planning
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var state uiState
	var plans []string
	var err error
	plansDir := cfg.PlansDir

	if planName == "" {
		plans, err = planner.ListPlans(plansDir)
		if err != nil {
			return err
		}

		if initialTask != "" {
			state = stateGeneratingPlanName
		} else if len(plans) > 0 {
			state = stateSelectPlan
		} else {
			state = stateAskTask
		}
	} else {
		// Plan name provided, we can attempt to load it immediately
		state = statePlanning // startPlanning will change this if needed
	}

	m := initialModel(ctx, state, cfg, plans, planName, initialTask, workspace, client)

	// If we have a planName, try to load it right away
	if planName != "" {
		err := startPlanning(&m)
		if err != nil {
			return err
		}
	}

	program := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	if fm, ok := finalModel.(model); ok && fm.err != nil {
		return fm.err
	}

	return nil
}
