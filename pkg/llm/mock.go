package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"planner/pkg/planner"
)

// MockClient is a dummy LLM client that generates mock tasks
type MockClient struct {
	MaxSubtasks int
}

func (m *MockClient) AnalyzeTask(ctx context.Context, task string) (planner.LLMResponse, error) {
	time.Sleep(200 * time.Millisecond)

	// In a real LLM prompt, you might use:
	// "Evaluate if this task describes editing, creating, or deleting a single file.
	// If yes, Action=Actionable.
	// If it's ambiguous or missing details, Action=AskUser.
	// Otherwise, Action=Decompose."

	if strings.Contains(strings.ToLower(task), "unclear") {
		return planner.LLMResponse{
			Action:   planner.ActionAskUser,
			Question: "I'm not sure which framework to use. Can you clarify?",
		}, nil
	}

	if strings.Contains(strings.ToLower(task), "step") || strings.Contains(strings.ToLower(task), "do") {
		return planner.LLMResponse{
			Action:    planner.ActionActionable,
			Reasoning: "Appears to be a single file operation.",
		}, nil
	}

	// Default: Break it down
	numTasks := m.MaxSubtasks
	if numTasks <= 0 {
		numTasks = 3
	}

	subTasks := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		subTasks[i] = fmt.Sprintf("Do step %d for: %s", i+1, task)
	}

	return planner.LLMResponse{
		Action:   planner.ActionDecompose,
		Subtasks: subTasks,
	}, nil
}
