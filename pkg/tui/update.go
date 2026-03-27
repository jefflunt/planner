package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"planner/pkg/planner"
)

type promptMsg planner.UserPrompt

type executionFinishedMsg struct {
	output string
	err    error
}

type planNameGeneratedMsg struct {
	name string
	err  error
	task string
}

func generatePlanNameCmd(ctx context.Context, client planner.LLMClient, task string) tea.Cmd {
	return func() tea.Msg {
		name, err := client.GeneratePlanName(ctx, task)
		return planNameGeneratedMsg{name: name, err: err, task: task}
	}
}

func listenForPrompt(p *planner.Planner) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return nil
		}
		prompt := <-p.Prompts
		return promptMsg(prompt)
	}
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.spinner.Tick, tickCmd(), textinput.Blink)
	if m.p != nil {
		cmds = append(cmds, listenForPrompt(m.p))
	}
	if m.state == stateGeneratingPlanName {
		cmds = append(cmds, generatePlanNameCmd(m.ctx, m.llmClient, m.initialTask))
	}
	return tea.Batch(cmds...)
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

func getFilteredPlans(m *model) []string {
	var filteredPlans []string
	for _, p := range m.plans {
		if m.searchQuery == "" || strings.Contains(strings.ToLower(p), strings.ToLower(m.searchQuery)) {
			filteredPlans = append(filteredPlans, p)
		}
	}
	return filteredPlans
}

func startPlanning(m *model) error {
	cfg := planner.Config{
		PlansDir:       m.cfg.PlansDir,
		StateFile:      filepath.Join(m.cfg.PlansDir, fmt.Sprintf("%s.json", m.planName)),
		Workspace:      m.workspaceDir,
		MaxConcurrency: m.cfg.MaxConcurrency,
		MaxRetries:     m.cfg.MaxRetries,
	}

	m.p = planner.NewPlanner(cfg, m.llmClient)

	// Try loading existing state
	loaded := m.p.Load() == nil && m.p.Root != nil

	if m.initialTask != "" {
		if !loaded || m.p.Root.Task != m.initialTask {
			m.state = statePlanning
			go m.p.Start(m.ctx, m.initialTask)
		} else {
			m.state = statePlanning
			go m.p.Plan(m.ctx, m.p.Root)
		}
	} else {
		if !loaded {
			m.state = stateAskTask
			m.textInput.SetValue("")
			m.textInput.Placeholder = "What task do you want to plan?"
			m.textInput.Focus()
		} else {
			m.state = statePlanning
			go m.p.Plan(m.ctx, m.p.Root)
		}
	}

	return nil
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
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 4 // Reserve space for status bar

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
		if m.p != nil {
			m.p.RLock()
			m.nodes = flattenTree(m.p.Root)
			m.p.RUnlock()
		}

		// Let the spinner update as well so it doesn't freeze when tickCmd returns
		var cmdSpinner tea.Cmd
		m.spinner, cmdSpinner = m.spinner.Update(msg)
		return m, tea.Batch(tickCmd(), cmdSpinner)

	case planNameGeneratedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}

		name := msg.name
		// Ensure uniqueness
		isUnique := false
		for !isUnique {
			isUnique = true
			for _, p := range m.plans {
				if p == name {
					isUnique = false
					break
				}
			}
			if !isUnique {
				// Append a random string or timestamp to make it unique
				name = fmt.Sprintf("%s-%d", msg.name, time.Now().UnixNano()%10000)
			}
		}

		m.planName = name
		m.initialTask = msg.task
		err := startPlanning(&m)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		return m, listenForPrompt(m.p)

	case executionFinishedMsg:
		m.state = statePlanning
		if msg.err != nil {
			m.state = stateExecuting
			m.executionOutput = fmt.Sprintf("Execution Error:\n\n%v", msg.err)
			m.viewport.SetContent(m.executionOutput)
		}
		return m, nil

	case tea.KeyMsg:
		if m.state == stateExecuting {
			if msg.Type == tea.KeyEsc {
				m.state = statePlanning
				return m, nil
			}
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		// Handle selection logic for a plan
		selectPlan := func(m *model) (tea.Model, tea.Cmd) {
			filteredPlans := getFilteredPlans(m)
			if m.planCursor == 0 {
				m.state = stateAskTask
				m.planName = ""
				m.initialTask = ""
				m.textInput.SetValue("")
				m.textInput.Placeholder = "What task do you want to plan?"
				m.textInput.Focus()
				return m, textinput.Blink
			} else if m.planCursor-1 < len(filteredPlans) {
				m.planName = filteredPlans[m.planCursor-1]
				m.initialTask = ""
				err := startPlanning(m)
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				return m, listenForPrompt(m.p)
			}
			return m, nil
		}

		if m.state == stateSelectPlan {
			if m.isSearching {
				switch msg.String() {
				case "esc":
					m.isSearching = false
					m.searchQuery = ""
					return m, nil
				case "enter":
					m.isSearching = false
					return selectPlan(&m)
				case "backspace":
					if len(m.searchQuery) > 0 {
						m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					}
					return m, nil
				case "ctrl+c":
					return m, tea.Quit
				default:
					if len(msg.String()) == 1 {
						m.searchQuery += msg.String()
					}
					return m, nil
				}
			}

			switch msg.String() {
			case "/":
				m.isSearching = true
				m.searchQuery = ""
				m.planCursor = 0
				return m, nil
			case "k":
				if m.planCursor > 0 {
					m.planCursor--
				}
			case "j":
				filteredPlans := getFilteredPlans(&m)
				if m.planCursor < len(filteredPlans) {
					m.planCursor++
				}
			case "D":
				filteredPlans := getFilteredPlans(&m)
				if m.planCursor > 0 && m.planCursor-1 < len(filteredPlans) {
					planName := filteredPlans[m.planCursor-1]
					err := planner.DeletePlan(m.cfg.PlansDir, planName)
					if err != nil {
						m.err = err
					} else {
						// Remove from m.plans
						for i, p := range m.plans {
							if p == planName {
								m.plans = append(m.plans[:i], m.plans[i+1:]...)
								break
							}
						}
						// Adjust cursor if needed
						if m.planCursor > len(getFilteredPlans(&m)) {
							m.planCursor--
						}
					}
					return m, nil
				}
				return m, nil
			case "enter":
				return selectPlan(&m)
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		if m.state == stateAskTask {
			switch msg.Type {
			case tea.KeyEnter:
				task := strings.TrimSpace(m.textInput.Value())
				if task != "" {
					if m.planName == "" {
						m.state = stateGeneratingPlanName
						return m, generatePlanNameCmd(m.ctx, m.llmClient, task)
					}
					m.state = statePlanning
					m.textInput.Blur()
					m.textInput.Placeholder = "Type your answer..."
					go m.p.Start(m.ctx, task)
				}
				return m, nil
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.state = stateSelectPlan
				m.textInput.Blur()
				return m, nil
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
		case "tab":
			if m.state == statePlanning {
				m.displayMode = (m.displayMode + 1) % 3
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.state == stateExecuting {
				m.state = statePlanning
			} else if m.state == statePlanning {
				m.state = stateSelectPlan
				m.p = nil

				// Refresh plans list
				if m.cfg != nil {
					plans, err := planner.ListPlans(m.cfg.PlansDir)
					if err == nil {
						m.plans = plans
					}
				}
			}
		case "x", "X":
			if m.state == statePlanning {
				plan := m.p.SerializePlan()

				cmd, err := m.llmClient.GetExecCommand(m.ctx, plan)
				if err != nil {
					m.state = stateExecuting
					m.executionOutput = fmt.Sprintf("Error preparing command:\n\n%v", err)
					m.viewport.SetContent(m.executionOutput)
					return m, nil
				}

				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return executionFinishedMsg{output: "Interactive execution finished natively.", err: err}
				})
			}
		case "up", "k":
			if m.cursorIndex > 0 {
				m.cursorIndex--
				if m.cursorIndex < m.viewportOffset {
					m.viewportOffset = m.cursorIndex
				}
			}
		case "down", "j":
			if m.cursorIndex < len(m.nodes)-1 {
				m.cursorIndex++
				// Rough estimate of visible items: ~half screen
				visibleLines := m.height / 2
				if m.cursorIndex >= m.viewportOffset+visibleLines {
					m.viewportOffset = m.cursorIndex - visibleLines + 1
				}
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
					m.state = stateAskTask
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
