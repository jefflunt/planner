package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"planner/pkg/config"
	"planner/pkg/llm"
	"planner/pkg/tui"
)

func isGitRepo() bool {
	cmd := exec.Command("git", "status")
	err := cmd.Run()
	return err == nil
}

func main() {
	if !isGitRepo() {
		fmt.Println("Error: planner must be run from inside a Git repository. This ensures accurate codebase context via .gitignore.")
		os.Exit(1)
	}

	// Default configuration
	configFile := config.DefaultPath()
	workspace := "./workspace"

	var planName string
	flag.StringVar(&planName, "plan", "", "Name of the plan to load or create")
	flag.Parse()

	var initialTask string

	// Check if data is piped to STDIN
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		// Read from STDIN
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			initialTask = strings.TrimSpace(string(data))
		}
	} else if flag.NArg() > 0 {
		// Optionally allow passing the task as arguments
		initialTask = strings.Join(flag.Args(), " ")
	}

	// Load or create default configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	plansDir := cfg.PlansDir
	if plansDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			plansDir = home + "/.planner/plans"
		} else {
			plansDir = "plans"
		}
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
	if err := tui.StartTUI(planName, initialTask, plansDir, workspace, client); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
