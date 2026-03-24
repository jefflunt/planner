# Architecture

The **planner** application is fundamentally a tree data structure orchestrator. It is built to recursively decompose a root task into an N-ary tree of subtasks.

## Core Concepts

### `planner.Planner`
The `Planner` struct is the central orchestrator. It manages the `Root` node, configuration state, file persistence, and the communication channel (`Prompts`) to yield for user input.

```go
type Planner struct {
	mu      sync.RWMutex
	Root    *Node
	Config  Config
	LLM     LLMClient
	Prompts chan UserPrompt
}
```

### `planner.Node`
A node represents a single task in the task tree.
- `ID`: UUID for tracking.
- `Task`: A string describing the work. This string grows as user clarifications are appended to it.
- `Type`: EITHER `TaskTypeAtomic` (a leaf node) or `TaskTypeComposite` (a node with children).
- `Status`: 
  - `pending` (waiting to be analyzed)
  - `composite` (analyzed, has children)
  - `actionable` (analyzed, single file operation)
  - `needs_input` (waiting for user clarification)

### TUI

The core planner is entirely decoupled from the user interface, but the primary consumer is the `plan-tui` binary.

**`plan-tui`**: Uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) to render an interactive application. It listens for `UserPrompt` messages asynchronously in its `Update` loop. When a prompt is received, it swaps the view to display a [Bubbles `textinput`](https://github.com/charmbracelet/bubbles/tree/master/textinput) component inline, capturing the user's answer and sending it back to the channel.

### State Persistence

The tree is saved to `plans/<plan-name>.json` (by default) after *every* mutation. The `Planner.Save()` method uses `sync.RWMutex` to ensure thread-safe writes. This directory structure enables managing multiple concurrent plans effectively. Each file acts as a single source of truth for its specific plan, meaning a planning session can be interrupted and resumed identically simply by selecting the plan again in the TUI or providing it via CLI arguments.
