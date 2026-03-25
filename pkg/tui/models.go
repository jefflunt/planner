package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"planner/pkg/config"
	"planner/pkg/planner"
)

type uiState int

const (
	stateSelectPlan uiState = iota
	stateAskTask
	stateGeneratingPlanName
	statePlanning
	stateExecuting
)

type model struct {
	p              *planner.Planner
	cursorIndex    int
	viewportOffset int
	nodes          []*planner.Node
	err            error
	width          int
	height         int

	state        uiState
	cfg          *config.Config
	plans        []string
	planCursor   int
	isSearching  bool
	searchQuery  string
	planName     string
	initialTask  string
	llmClient    planner.LLMClient
	workspaceDir string

	// Prompt handling
	currentPrompt       *planner.UserPrompt
	editingNode         *planner.Node
	addingChildTo       *planner.Node
	addingSiblingTo     *planner.Node
	addingSiblingBefore bool
	textInput           textinput.Model
	askingForTask       bool
	ctx                 context.Context

	// Execution
	executionOutput  string
	viewport         viewport.Model
	executionPrompt  string
	executionCommand string

	// Spinner
	spinner spinner.Model
}

type shortcut struct {
	Key, Description string
}

func getShortcuts(m model) []shortcut {
	switch m.state {
	case stateSelectPlan:
		return []shortcut{
			{"j/k", "nav"},
			{"/", "search"},
			{"enter", "select"},
			{"D", "delete"},
			{"q", "quit"},
		}
	case stateAskTask:
		return []shortcut{
			{"enter", "submit"},
			{"esc", "back"},
			{"ctrl+c", "quit"},
		}
	case statePlanning:
		return []shortcut{
			{"j/k", "nav"},
			{"esc", "back"},
			{"e", "edit"},
			{"d", "del"},
			{"R", "replan"},
			{"+", "child"},
			{"[", "before"},
			{"]", "after"},
			{"X", "exec"},
			{"q", "quit"},
		}
	case stateExecuting:
		return []shortcut{
			{"esc", "return"},
		}
	default:
		return nil
	}
}

func initialModel(ctx context.Context, state uiState, cfg *config.Config, plans []string, planName string, initialTask string, workspace string, client planner.LLMClient) model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60

	if state == stateAskTask {
		ti.Placeholder = "What task do you want to plan?"
	} else {
		ti.Placeholder = "Type your answer..."
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(0, 0)

	return model{
		state:        state,
		cfg:          cfg,
		plans:        plans,
		planName:     planName,
		initialTask:  initialTask,
		workspaceDir: workspace,
		llmClient:    client,
		textInput:    ti,
		ctx:          ctx,
		spinner:      s,
		viewport:     vp,
		isSearching:  false,
		searchQuery:  "",
	}
}
