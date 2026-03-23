# Configuration

The core configuration for the planner is managed via the `planner.Config` struct.

```go
type Config struct {
	StateFile string
	Workspace string // Directory to hold workspaces
}
```

## CLI Flags

Both `plan-tui` and `plan-cli` expose persistent flags through Cobra for configuration:

1. `--state`
   - **Type:** String
   - **Default:** `planner-state.json` (in the current directory)
   - **Usage:** Changes where the JSON persistence file is stored and loaded from. Crucial if you are running multiple independent plans in the same directory.
   - Example: `--state=my-plan.json`

2. `--workspace`
   - **Type:** String
   - **Default:** `./workspace`
   - **Usage:** Specifies the directory meant to hold the execution artifacts. *(Note: This was originally used in the `fractals` execution framework, but remains here as part of the domain terminology in case the generated plan references it).*
   - Example: `--workspace=/tmp/isolated-work`
