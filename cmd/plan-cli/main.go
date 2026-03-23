package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"planner/pkg/llm"
	"planner/pkg/planner"
)

var (
	stateFile string
	workspace string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "plan-cli",
		Short: "A recursive agentic task orchestrator (CLI)",
	}

	rootCmd.PersistentFlags().StringVar(&stateFile, "state", "planner-state.json", "Path to save planner state")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "./workspace", "Workspace directory")

	var planCmd = &cobra.Command{
		Use:   "plan [task]",
		Short: "Start planning a task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := planner.Config{
				StateFile: stateFile,
				Workspace: workspace,
			}

			// For now, use a mock LLM.
			client := &llm.MockClient{MaxSubtasks: 3}
			p := planner.NewPlanner(cfg, client)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			task := args[0]
			fmt.Printf("Starting CLI planning for: %q\n", task)

			// Start planning in a goroutine so we can handle prompts on the main thread
			errChan := make(chan error, 1)
			go func() {
				errChan <- p.Start(ctx, task)
			}()

			// Listen for prompts or completion
			for {
				select {
				case err := <-errChan:
					if err != nil {
						return err
					}
					fmt.Println("Planning completed. State saved to:", stateFile)
					return nil
				case prompt := <-p.Prompts:
					fmt.Printf("\n[Clarification Needed] Node: %s\n", prompt.NodeID)
					fmt.Printf("Task: %s\n", prompt.Task)
					fmt.Printf("Question: %s\n", prompt.Question)
					fmt.Print("Your Answer: ")

					reader := bufio.NewReader(os.Stdin)
					answer, _ := reader.ReadString('\n')
					prompt.ReplyChan <- strings.TrimSpace(answer)
				}
			}
		},
	}

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

	rootCmd.AddCommand(planCmd, listCmd)

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
