package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"planner/pkg/config"
	"planner/pkg/planner"
)

func determineInitialState(planName string, initialTask string) uiState {
	if planName != "" {
		return statePlanning
	}
	if initialTask != "" {
		return stateGeneratingPlanName
	}
	return stateSelectPlan
}

func StartTUI(planName string, initialTask string, cfg *config.Config, workspace string, version string, client planner.LLMClient) error {
	// Context for planning
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var plans []string
	var err error
	plansDir := cfg.PlansDir

	if planName == "" {
		plans, err = planner.ListPlans(plansDir)
		if err != nil {
			return err
		}
	}

	state := determineInitialState(planName, initialTask)

	m := initialModel(ctx, state, cfg, plans, planName, initialTask, workspace, version, client)

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
