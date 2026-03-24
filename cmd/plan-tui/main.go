package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"planner/pkg/tui"
)

func main() {
	// Default configuration
	stateFile := "planner-state.json"
	workspace := "./workspace"

	var initialTask string

	// Check if data is piped to STDIN
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		// Read from STDIN
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			initialTask = strings.TrimSpace(string(data))
		}
	} else if len(os.Args) > 1 {
		// Optionally allow passing the task as arguments
		initialTask = strings.Join(os.Args[1:], " ")
	}

	// Launch the TUI
	if err := tui.StartTUI(initialTask, stateFile, workspace); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
