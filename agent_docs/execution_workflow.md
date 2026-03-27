# Execution Workflow

While **planner** is primarily focused on the recursive decomposition of tasks (Planning Mode), it also provides a bridge to actually execute the generated plan. This is known as **Execution Mode**.

Execution Mode works by gathering the selected node's detailed context (Task, Details, AsciiDiagram) along with a serialized representation of the overall plan tree, and passing it to an underlying, execution-capable agent (such as the Gemini CLI, GitHub Copilot CLI, or an Opencode agent) natively in the terminal.

## Sequence Diagram

```mermaid
sequenceDiagram
    actor User
    participant TUI as TUI (plan-tui)
    participant Planner as Planner (pkg/planner)
    participant Client as LLM Client (pkg/llm)
    participant Prompts as Prompts (pkg/prompts)
    participant OS as Operating System
    participant Agent as Underlying Agent (e.g., gemini)

    User->>TUI: Press 'x' or 'X' (Trigger Execution)
    TUI->>Planner: GetExecCommand(ctx, node.ID)
    Note over Planner: Gathers Task, Details, AsciiDiagram,<br/>and FormatPlanStructure()
    Planner->>Client: GetExecCommand(ctx, ExecRequest)
    
    Client->>Prompts: Load("execute_plan", data)
    Prompts-->>Client: string (Rendered Prompt)
    
    Client-->>Planner: exec.Cmd (Un-started OS Command)
    Planner-->>TUI: exec.Cmd
    
    TUI->>OS: tea.ExecProcess(cmd)
    Note over TUI, OS: TUI suspends rendering
    
    OS->>Agent: Run Command Natively (Interactive)
    Note over Agent: Agent modifies files, outputs to STDOUT/STDERR
    Agent-->>OS: Process Exits
    
    OS-->>TUI: Resume Execution (tea.Msg)
    Note over TUI: TUI restores rendering
    TUI-->>User: Display Execution Result
```

## How It Works

1. **Triggering Execution:** The user initiates execution from the TUI while in the Planning state (`statePlanning`) by selecting a node and pressing the `x` or `X` key.
2. **Context Gathering:** The TUI calls `Planner.GetExecCommand` for the selected node. The core orchestrator gathers the node's `Task`, `Details`, and `AsciiDiagram`, and generates a string representation of the current task tree using `FormatPlanStructure()`.
3. **Command Preparation:** This contextual data (`ExecRequest`) is passed to the active LLM client's `GetExecCommand` method.
   - The client loads the `execute_plan` prompt template from the `pkg/prompts` package, injecting the node's specific context and the overall plan structure.
   - The client constructs an un-started `exec.Cmd` tailored to its specific agent CLI tool (e.g., `gemini <prompt>`).
4. **Native Execution:** The TUI leverages Bubble Tea's `tea.ExecProcess` function. This temporarily suspends the TUI, clears the screen, and runs the generated `exec.Cmd` natively attached to the user's terminal. This allows interactive tools to prompt the user or stream output directly to standard out without TUI interference.
5. **Resumption:** Once the underlying agent process exits, the operating system returns control to the TUI. The TUI resumes rendering and displays an execution summary view (`stateExecuting`), indicating success or displaying any captured errors.
