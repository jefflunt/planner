package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"planner/pkg/config"
	"planner/pkg/llm"
	"planner/pkg/tui"
)

func main() {
	// Default configuration
	configFile := "planner.yaml"
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

	// Load or create default configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create an overarching context
	ctx := context.Background()

	// Instantiate the LLM based on the loaded config
	client, err := llm.NewClient(ctx, cfg)
	if err != nil {
		fmt.Printf("Failed to initialize LLM client: %v\n", err)
		os.Exit(1)
	}

	// Launch the TUI
	if err := tui.StartTUI(initialTask, stateFile, workspace, client); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
