package tui

import (
	"fmt"
	"strings"

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

func renderStatusBar(m model) string {
	termWidth := m.width
	if termWidth <= 0 {
		termWidth = 80
	}

	shortcuts := getShortcuts(m)
	shortcutStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Padding(0, 1)

	var b strings.Builder
	for i, s := range shortcuts {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(shortcutStyle.Render(s.Key))
		b.WriteString(" " + s.Description)
	}
	statusText := b.String()

	// Add pending count if in statePlanning
	var rightText string
	if m.state == statePlanning {
		pendingCount := 0
		for _, n := range m.nodes {
			if n.Status == planner.StatusPending {
				pendingCount++
			}
		}
		rightText = fmt.Sprintf(" pending:%d ", pendingCount)
	}

	leftPart := lipgloss.NewStyle().
		Foreground(lipgloss.Color("253")).
		Background(lipgloss.Color("235")).
		Render(statusText)

	rightPart := lipgloss.NewStyle().
		Foreground(lipgloss.Color("253")).
		Background(lipgloss.Color("235")).
		Width(termWidth - lipgloss.Width(leftPart)).
		Align(lipgloss.Right).
		Render(rightText)

	return lipgloss.JoinHorizontal(lipgloss.Left, leftPart, rightPart)
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

	if m.state == stateSelectPlan {
		if m.isSearching {
			b.WriteString(wrapStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render(fmt.Sprintf("Search: /%s", m.searchQuery))))
		} else {
			b.WriteString(wrapStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Select a plan or create a new one:")))
		}
		b.WriteString("\n\n")

		var filteredPlans []string
		for _, p := range m.plans {
			if m.searchQuery == "" || strings.Contains(strings.ToLower(p), strings.ToLower(m.searchQuery)) {
				filteredPlans = append(filteredPlans, p)
			}
		}

		// Adjust cursor if it's out of bounds
		if m.planCursor > len(filteredPlans) {
			m.planCursor = len(filteredPlans)
		}

		// Render [Create New Plan] (index 0)
		cursor := "  "
		if m.planCursor == 0 {
			cursor = "> "
		}
		createLine := fmt.Sprintf("%s[Create New Plan]", cursor)
		if m.planCursor == 0 {
			b.WriteString(selectedStyle.Render(createLine) + "\n")
		} else {
			b.WriteString(createLine + "\n")
		}

		// Render plans (index 1 to len(filteredPlans))
		for i, p := range filteredPlans {
			idx := i + 1
			cursor := "  "
			if m.planCursor == idx {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%s", cursor, p)
			if m.planCursor == idx {
				b.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}

		return wrapContentWithStatusBar(m, b.String())
	}

	if m.state == stateAskTask {
		var promptStr string
		if m.planName == "" {
			promptStr = "Enter the task you want to break down:"
		} else {
			promptStr = fmt.Sprintf("Plan: %s - Enter the task you want to break down:", m.planName)
		}
		b.WriteString(wrapStyle.Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render(promptStr)))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		return wrapContentWithStatusBar(m, b.String())
	}

	if len(m.nodes) == 0 {
		return "Initializing plan..."
	}

	if m.state == stateExecuting {
		// Re-render to update the viewport content
		content := fmt.Sprintf("Prompt:\n%s\n\nCommand:\n%s\n\nOutput:\n%s", m.executionPrompt, m.executionCommand, m.executionOutput)
		m.viewport.SetContent(content)
		return wrapContentWithStatusBar(m, m.viewport.View())
	}

	var tasksBuilder strings.Builder
	for i, n := range m.nodes {
		// Only render items within the viewport
		if i < m.viewportOffset || i >= m.viewportOffset+m.height {
			continue
		}

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
			indicator = "- "
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
			tasksBuilder.WriteString(selectedStyle.Render(renderedLine) + "\n")
		} else {
			tasksBuilder.WriteString(renderedLine + "\n")
		}
	}
	tasksList := tasksBuilder.String()

	// Build prompt/edit UI if active
	var promptBuilder strings.Builder
	hasPrompt := false
	if m.currentPrompt != nil {
		promptBuilder.WriteString(strings.Repeat("─", termWidth/2) + "\n")
		promptBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8A65")).Render("Clarification Needed: "))
		promptBuilder.WriteString("\n")
		promptBuilder.WriteString(wrapStyle.Render(m.currentPrompt.Question))
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(m.textInput.View())
		promptBuilder.WriteString("\n\n")
		hasPrompt = true
	} else if m.editingNode != nil {
		promptBuilder.WriteString(strings.Repeat("─", termWidth/2) + "\n")
		promptBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("Editing Task: "))
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(m.textInput.View())
		promptBuilder.WriteString("\n\n")
		hasPrompt = true
	} else if m.addingChildTo != nil {
		promptBuilder.WriteString(strings.Repeat("─", termWidth/2) + "\n")
		promptBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("Add Child Task: "))
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(m.textInput.View())
		promptBuilder.WriteString("\n\n")
		hasPrompt = true
	} else if m.addingSiblingTo != nil {
		promptBuilder.WriteString(strings.Repeat("─", termWidth/2) + "\n")
		var mode string
		if m.addingSiblingBefore {
			mode = "Before"
		} else {
			mode = "After"
		}
		promptBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(fmt.Sprintf("Add Sibling Task (%s): ", mode)))
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(m.textInput.View())
		promptBuilder.WriteString("\n\n")
		hasPrompt = true
	}

	if hasPrompt {
		b.WriteString(promptBuilder.String())
	}
	b.WriteString(tasksList)

	return wrapContentWithStatusBar(m, b.String())
}

func wrapContentWithStatusBar(m model, mainContent string) string {
	termWidth := m.width
	if termWidth <= 0 {
		termWidth = 80
	}

	statusBar := renderStatusBar(m)
	statusHeight := lipgloss.Height(statusBar)

	if m.height <= statusHeight {
		return mainContent
	}

	mainArea := lipgloss.NewStyle().
		Height(m.height - statusHeight).
		MaxHeight(m.height - statusHeight).
		Width(termWidth).
		MaxWidth(termWidth).
		Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBar)
}
