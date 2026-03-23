# Planning Workflow

The core mechanic of **planner** is its recursive interaction loop, defined in `pkg/planner/planner.go` within the `Plan(ctx context.Context, node *Node) error` method.

## The Goal
Unlike execution-focused agents that try to *do* the work immediately, this project strictly focuses on decomposition until a specific actionable threshold is met.

**The Actionable Heuristic:** A leaf node is actionable *if and only if* it describes the creation, deletion, or editing of a single file on disk. 
- Example: "Refactor the authentication module" -> *Not Actionable* (Multiple files, too vague)
- Example: "Rename `AuthUser` to `SessionUser` in `src/auth/models.go`" -> *Actionable* (Single file operation)

This ensures that whatever execution harness eventually consumes the leaf nodes can do so with extreme predictability.

---

## The Loop (`Plan()` Method)

When `p.Plan(ctx, rootNode)` is called, the orchestrator begins a recursive loop over the tree.

1. **Ask LLM**: `AnalyzeTask(ctx, task)`
   The LLM is prompted with the current task description and the actionable heuristic. It must return a structured `LLMResponse`:

```go
type PlanAction string
const (
	ActionActionable PlanAction = "actionable"
	ActionDecompose  PlanAction = "decompose"
	ActionAskUser    PlanAction = "ask_user"
)

type LLMResponse struct {
	Action    PlanAction `json:"action"`
	Subtasks  []string   `json:"subtasks,omitempty"` // Populated if Action == Decompose
	Question  string     `json:"question,omitempty"` // Populated if Action == AskUser
	Reasoning string     `json:"reasoning,omitempty"` 
}
```

2. **Handle the LLM Response**:

   - **`ActionActionable`**: The LLM determines this is a single file operation. The node's status is set to `actionable`, and this branch terminates successfully.
   
   - **`ActionDecompose`**: The LLM determines the task is too broad. The node's status is set to `composite`, and it generates `N` subtasks. The planner creates child `Node`s for each subtask and recursively calls `Plan()` on each child.
   
   - **`ActionAskUser`**: The LLM determines the task is ambiguous (e.g., "Build a web scraper" -> "Which language?"). 

3. **Yielding to User (`ActionAskUser`)**:
   When the LLM requests clarification, the `Plan()` function blocks.
   - It constructs a `UserPrompt` object containing the task, the LLM's question, and a `ReplyChan`.
   - It sends this prompt to the `p.Prompts` channel.
   - The UI (CLI or TUI) receives the prompt, displays it, and waits for user input.
   - The UI sends the user's string response back via the `ReplyChan`.
   - The planner appends the answer directly into the node's `Task` context string: `Task = Task + "\n\n[Clarification]: " + answer`.
   - The `for` loop restarts, feeding the newly augmented task string back into step 1 (`AnalyzeTask`).

This infinite loop guarantees that no branch stops growing until it is perfectly clarified and distilled into single-file actionable nodes.
