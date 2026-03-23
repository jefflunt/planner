package main

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"planner/pkg/planner"
	"planner/pkg/tui"
)

var (
	stateFile string
	workspace string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "plan-tui [task]",
		Short: "A recursive agentic task orchestrator (TUI)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// We launch the TUI
			return tui.StartTUI(args[0], stateFile, workspace)
		},
	}

	rootCmd.PersistentFlags().StringVar(&stateFile, "state", "planner-state.json", "Path to save planner state")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "./workspace", "Workspace directory")

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List tasks from the current plan state",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := planner.Config{StateFile: stateFile}
			p := planner.NewPlanner(cfg, nil)
			if err := p.Load(); err != nil {
				return err
			}

			if p.Root == nil {
				fmt.Println("No active plan.")
				return nil
			}

			printTree(p.Root, "")
			return nil
		},
	}

	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func printTree(node *planner.Node, prefix string) {
	fmt.Printf("%s[%s] %s (%s) %s\n", prefix, node.Type, node.Task, node.Status, node.ID)
	for _, child := range node.Children {
		printTree(child, prefix+"  ")
	}
}
