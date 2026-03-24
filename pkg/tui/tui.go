package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	currentPrompt       *planner.UserPrompt
	editingNode         *planner.Node
	addingChildTo       *planner.Node
	addingSiblingTo     *planner.Node
	addingSiblingBefore bool
	textInput           textinput.Model
	askingForTask       bool
	ctx                 context.Context

	// Spinner
	spinner spinner.Model
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

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		p:             p,
		textInput:     ti,
		askingForTask: askingForTask,
		ctx:           ctx,
		spinner:       s,
	}
}

func StartTUI(task string, stateFile string, workspace string, client planner.LLMClient) error {
	cfg := planner.Config{
		StateFile: stateFile,
		Workspace: workspace,
	}

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
		} else {
			go p.Plan(ctx, p.Root)
		}
	} else {
		if !loaded {
			askingForTask = true
		} else {
			go p.Plan(ctx, p.Root)
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
	return tea.Batch(m.spinner.Tick, tickCmd(), listenForPrompt(m.p), textinput.Blink)
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

		// Adjust text input width dynamically
		if m.width > 4 {
			m.textInput.Width = m.width - 4
		}

	case spinner.TickMsg:
		var cmdSpinner tea.Cmd
		m.spinner, cmdSpinner = m.spinner.Update(msg)
		return m, cmdSpinner

	case promptMsg:
		prompt := planner.UserPrompt(msg)
		m.currentPrompt = &prompt
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink

	case tickMsg:
		m.p.RLock()
		m.nodes = flattenTree(m.p.Root)
		m.p.RUnlock()

		// Let the spinner update as well so it doesn't freeze when tickCmd returns
		var cmdSpinner tea.Cmd
		m.spinner, cmdSpinner = m.spinner.Update(msg)
		return m, tea.Batch(tickCmd(), cmdSpinner)

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

		// If we are currently editing a node
		if m.editingNode != nil {
			switch msg.Type {
			case tea.KeyEnter:
				newTask := strings.TrimSpace(m.textInput.Value())
				if newTask != "" {
					node, err := m.p.EditNode(m.editingNode.ID, newTask)
					if err == nil {
						// Re-run planning on the edited node
						go m.p.Plan(m.ctx, node)
					}
				}
				m.editingNode = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyEsc:
				m.editingNode = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyCtrlC:
				return m, tea.Quit
			}

			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// If we are currently adding a child
		if m.addingChildTo != nil {
			switch msg.Type {
			case tea.KeyEnter:
				newTask := strings.TrimSpace(m.textInput.Value())
				if newTask != "" {
					node, err := m.p.AddChild(m.addingChildTo.ID, newTask)
					if err == nil {
						go m.p.Plan(m.ctx, node)
					}
				}
				m.addingChildTo = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyEsc:
				m.addingChildTo = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyCtrlC:
				return m, tea.Quit
			}

			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// If we are currently adding a sibling
		if m.addingSiblingTo != nil {
			switch msg.Type {
			case tea.KeyEnter:
				newTask := strings.TrimSpace(m.textInput.Value())
				if newTask != "" {
					node, err := m.p.AddSibling(m.addingSiblingTo.ID, newTask, m.addingSiblingBefore)
					if err == nil {
						go m.p.Plan(m.ctx, node)
					}
				}
				m.addingSiblingTo = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyEsc:
				m.addingSiblingTo = nil
				m.textInput.Blur()
				return m, nil
			case tea.KeyCtrlC:
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
		case "e":
			if m.cursorIndex >= 0 && m.cursorIndex < len(m.nodes) {
				m.editingNode = m.nodes[m.cursorIndex]
				m.textInput.SetValue(m.editingNode.Task)
				m.textInput.Placeholder = "Edit task..."
				m.textInput.Focus()
				return m, textinput.Blink
			}
		case "d":
			if m.cursorIndex >= 0 && m.cursorIndex < len(m.nodes) {
				node := m.nodes[m.cursorIndex]
				m.p.DeleteNode(node.ID)

				m.p.RLock()
				m.nodes = flattenTree(m.p.Root)
				m.p.RUnlock()

				if len(m.nodes) == 0 {
					m.askingForTask = true
					m.textInput.SetValue("")
					m.textInput.Placeholder = "What task do you want to plan?"
					m.textInput.Focus()
					return m, textinput.Blink
				}

				if m.cursorIndex >= len(m.nodes) {
					m.cursorIndex = len(m.nodes) - 1
				}
				if m.cursorIndex < 0 {
					m.cursorIndex = 0
				}
				return m, nil
			}
		case "R":
			if m.cursorIndex >= 0 && m.cursorIndex < len(m.nodes) {
				node := m.nodes[m.cursorIndex]
				replannedNode, err := m.p.ReplanNode(node.ID)
				if err == nil {
					go m.p.Plan(m.ctx, replannedNode)
				}

				m.p.RLock()
				m.nodes = flattenTree(m.p.Root)
				m.p.RUnlock()
				return m, nil
			}
		case "+":
			if m.cursorIndex >= 0 && m.cursorIndex < len(m.nodes) {
				m.addingChildTo = m.nodes[m.cursorIndex]
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Enter child task..."
				m.textInput.Focus()
				return m, textinput.Blink
			}
		case "[", "]":
			if m.cursorIndex >= 0 && m.cursorIndex < len(m.nodes) {
				node := m.nodes[m.cursorIndex]
				if m.p.Root != nil && m.p.Root.ID == node.ID {
					// Cannot add sibling to root
					return m, nil
				}

				m.addingSiblingTo = node
				m.addingSiblingBefore = (msg.String() == "[")
				m.textInput.SetValue("")

				if m.addingSiblingBefore {
					m.textInput.Placeholder = "Enter sibling task (before)..."
				} else {
					m.textInput.Placeholder = "Enter sibling task (after)..."
				}

				m.textInput.Focus()
				return m, textinput.Blink
			}
		}

	}

	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	termWidth := m.width
	if termWidth <= 0 {
		termWidth = 80
	}
	wrapStyle := lipgloss.NewStyle().Width(termWidth - 4)

	var b strings.Builder

	if m.askingForTask {
		b.WriteString(wrapStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Enter the task you want to break down:")))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to submit)")
		return b.String()
	}

	if len(m.nodes) == 0 {
		return "Initializing plan..."
	}

	for i, n := range m.nodes {
		cursor := "  "
		if m.cursorIndex == i {
			cursor = "> "
		}

		indent := strings.Repeat("  ", n.Depth)

		var s lipgloss.Style
		switch n.Status {
		case planner.StatusActionable:
			s = statusActionableStyle
		case planner.StatusNeedsInput:
			s = statusNeedsInputStyle
		default:
			s = statusPendingStyle
		}

		var indicator string
		if n.Status == planner.StatusNeedsInput {
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8A65")).Render("? ")
		} else if n.Type == planner.TaskTypeComposite {
			indicator = "+ "
		} else if n.Type == planner.TaskTypeAtomic {
			indicator = "- "
		} else {
			// Still evaluating
			indicator = m.spinner.View()
		}

		var line string
		if n.Status == planner.StatusNeedsInput {
			// Only show status string if it specifically needs input
			line = fmt.Sprintf("%s%s%s%s (%s)", cursor, indent, indicator, n.Task, s.Render(string(n.Status)))
		} else {
			line = fmt.Sprintf("%s%s%s%s", cursor, indent, indicator, n.Task)
		}

		// Optionally wrap long lines, but keep the indent structure
		lineWrapStyle := lipgloss.NewStyle().Width(termWidth - 4)
		renderedLine := lineWrapStyle.Render(line)

		if m.cursorIndex == i {
			b.WriteString(selectedStyle.Render(renderedLine) + "\n")
		} else {
			b.WriteString(renderedLine + "\n")
		}
	}

	// Render prompt if active
	if m.currentPrompt != nil {
		b.WriteString("\n\n" + strings.Repeat("─", termWidth/2) + "\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Clarification Needed: "))
		b.WriteString("\n")
		b.WriteString(wrapStyle.Render(m.currentPrompt.Question))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to submit)")
	} else if m.editingNode != nil {
		b.WriteString("\n\n" + strings.Repeat("─", termWidth/2) + "\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("Editing Task: "))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to save and re-plan | Esc to cancel)")
	} else if m.addingChildTo != nil {
		b.WriteString("\n\n" + strings.Repeat("─", termWidth/2) + "\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("Add Child Task: "))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to save and plan | Esc to cancel)")
	} else if m.addingSiblingTo != nil {
		b.WriteString("\n\n" + strings.Repeat("─", termWidth/2) + "\n")
		var mode string
		if m.addingSiblingBefore {
			mode = "Before"
		} else {
			mode = "After"
		}
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(fmt.Sprintf("Add Sibling Task (%s): ", mode)))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n(Press Enter to save and plan | Esc to cancel)")
	}

	mainContent := b.String()

	// Build the status bar
	statusText := " Commands: [j/k] nav | [e] edit | [d] del | [R] replan | [+] child | [[] before | []] after | [q] quit "
	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("253")). // Bright light gray/white text
		Background(lipgloss.Color("235")). // Slightly lighter dark gray background
		Width(termWidth).
		Render(statusText)

	// Since we are using an AltScreen, the height is fixed to m.height.
	// We want the main content to take up everything except the status bar height,
	// and we want the status bar pinned to the bottom.
	statusHeight := lipgloss.Height(statusBar)

	// If terminal is too small to render even the status bar, just return main content
	if m.height <= statusHeight {
		return mainContent
	}

	// Use Place to push the status bar to the very bottom
	mainArea := lipgloss.NewStyle().
		Height(m.height - statusHeight).
		MaxHeight(m.height - statusHeight).
		Width(termWidth).
		MaxWidth(termWidth).
		Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBar)
}
