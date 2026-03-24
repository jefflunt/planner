# Configuration

The core configuration for the planner is managed via the `planner.Config` struct.

```go
type Config struct {
	PlansDir  string
	StateFile string
	Workspace string // Directory to hold workspaces
}
```

## Configuration File

By default, the planner looks for a YAML configuration file at `~/.planner/config.yml`. If this file does not exist, it falls back to a set of in-memory defaults.

An example `config.yml` looks like:

```yaml
plans_dir: "~/.planner/plans"
llm:
  provider: "gemini"
  model: "gemini-3.1-flash-lite-preview"
  api_key: "YOUR_API_KEY_HERE" # Optional: Can also be passed via GEMINI_API_KEY env var
```

## CLI Behavior

`plan-tui` handles configuration and CLI flags:

1. **Config Path**
   - **Default:** `~/.planner/config.yml`

2. **Plans Directory**
   - **Default:** `~/.planner/plans/`
   - **Usage:** Stores all the individual plan JSON files (e.g., `~/.planner/plans/my-plan.json`).

3. **Plan Name**
   - **CLI Flag:** `-plan my-plan`
   - **Usage:** Loads or creates a plan with the given name. If not provided, the user will be prompted with an interactive menu to select an existing plan or create a new one.

4. **Workspace**
   - **Default:** `./workspace`
   - **Usage:** Specifies the directory meant to hold the execution artifacts. *(Note: This was originally used in the `fractals` execution framework, but remains here as part of the domain terminology in case the generated plan references it).*
