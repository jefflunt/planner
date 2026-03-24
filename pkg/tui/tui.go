package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"planner/pkg/llm"
	"planner/pkg/planner"
)

var (
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	statusActionableStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8E6CF"))
	statusNeedsInputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD3B6"))
	statusPendingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	selectedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8A65")).Bold(true)
)

type model struct {
	p           *planner.Planner
	cursorIndex int
	nodes       []*planner.Node
	err         error
	width       int
	height      int

	// Prompt handling
	currentPrompt *planner.UserPrompt
	textInput     textinput.Model
	askingForTask bool
	ctx           context.Context
}

func initialModel(p *planner.Planner, askingForTask bool, ctx context.Context) model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60

	if askingForTask {
		ti.Placeholder = "What task do you want to plan?"
	} else {
		ti.Placeholder = "Type your answer..."
	}

	return model{
		p:             p,
		textInput:     ti,
		askingForTask: askingForTask,
		ctx:           ctx,
	}
}

func StartTUI(task string, stateFile string, workspace string) error {
	cfg := planner.Config{
		StateFile: stateFile,
		Workspace: workspace,
	}

	client := &llm.MockClient{MaxSubtasks: 3}
	p := planner.NewPlanner(cfg, client)

	// Context for planning
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	askingForTask := false

	// Try loading existing state
	loaded := p.Load() == nil && p.Root != nil

	if task != "" {
		if !loaded || p.Root.Task != task {
			go p.Start(ctx, task)
		}
	} else {
		if !loaded {
			askingForTask = true
		}
	}

	m := initialModel(p, askingForTask, ctx)
	program := tea.NewProgram(m, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

type promptMsg planner.UserPrompt

func listenForPrompt(p *planner.Planner) tea.Cmd {
	return func() tea.Msg {
		prompt := <-p.Prompts
		return promptMsg(prompt)
	}
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), listenForPrompt(m.p), textinput.Blink)
}

func flattenTree(n *planner.Node) []*planner.Node {
	if n == nil {
		return nil
	}
	var flat []*planner.Node
	flat = append(flat, n)
	for _, c := range n.Children {
		flat = append(flat, flattenTree(c)...)
	}
	return flat
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case promptMsg:
		prompt := planner.UserPrompt(msg)
		m.currentPrompt = &prompt
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink

	case tickMsg:
		m.p.Load() // Reload state from disk to catch async updates
		m.nodes = flattenTree(m.p.Root)
		return m, tickCmd()

	case tea.KeyMsg:
		// If we are currently asking for the root task
		if m.askingForTask {
			switch msg.Type {
			case tea.KeyEnter:
				task := strings.TrimSpace(m.textInput.Value())
				if task != "" {
					m.askingForTask = false
					m.textInput.Blur()
					m.textInput.Placeholder = "Type your answer..."
					go m.p.Start(m.ctx, task)
				}
				return m, nil
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			}

			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// If we are currently answering a prompt
		if m.currentPrompt != nil {
			switch msg.Type {
			case tea.KeyEnter:
				// Send answer back
				m.currentPrompt.ReplyChan <- m.textInput.Value()
				m.currentPrompt = nil
				m.textInput.Blur()
				return m, listenForPrompt(m.p)
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			}

			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// Navigation mode
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursorIndex > 0 {
				m.cursorIndex--
			}
		case "down", "j":
			if m.cursorIndex < len(m.nodes)-1 {
				m.cursorIndex++
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	if m.askingForTask {
		b.WriteString(titleStyle.Render(" Planner "))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Enter the task you want to break down:"))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to submit)")
		return b.String()
	}

	if len(m.nodes) == 0 {
		return "Initializing plan..."
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Planner: %s ", m.p.Root.Task)))
	b.WriteString("\n\n")

	for i, n := range m.nodes {
		cursor := "  "
		if m.cursorIndex == i {
			cursor = "> "
		}

		indent := strings.Repeat("  ", n.Depth)

		statusStr := string(n.Status)
		var s lipgloss.Style
		switch n.Status {
		case planner.StatusActionable:
			s = statusActionableStyle
		case planner.StatusNeedsInput:
			s = statusNeedsInputStyle
		default:
			s = statusPendingStyle
		}

		line := fmt.Sprintf("%s%s[%s] %s (%s)", cursor, indent, n.Type, n.Task, s.Render(statusStr))

		if m.cursorIndex == i {
			b.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}

	// Render prompt if active
	if m.currentPrompt != nil {
		b.WriteString("\n\n" + strings.Repeat("─", m.width/2) + "\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Clarification Needed: "))
		b.WriteString(m.currentPrompt.Question + "\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to submit)")
	} else {
		b.WriteString("\n\nCommands: [j/k] navigate | [q] quit")
	}

	return b.String()
}
