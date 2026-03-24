# Configuration

The core configuration for the planner is managed via the `planner.Config` struct.

```go
type Config struct {
	StateFile string
	Workspace string // Directory to hold workspaces
}
```

## Configuration File

By default, the planner looks for a YAML configuration file at `~/.planner/config.yml`. If this file does not exist, it falls back to a set of in-memory defaults.

An example `config.yml` looks like:

```yaml
llm:
  provider: "gemini"
  model: "gemini-3.1-flash-lite-preview"
  api_key: "YOUR_API_KEY_HERE" # Optional: Can also be passed via GEMINI_API_KEY env var
```

## CLI Flags

`plan-cli` exposes persistent flags through Cobra for configuration:

1. `--config`
   - **Type:** String
   - **Default:** `~/.planner/config.yml`
   - **Usage:** Specifies an alternate configuration file to load.

2. `--state`
   - **Type:** String
   - **Default:** `planner-state.json` (in the current directory)
   - **Usage:** Changes where the JSON persistence file is stored and loaded from. Crucial if you are running multiple independent plans in the same directory.
   - Example: `--state=my-plan.json`

2. `--workspace`
   - **Type:** String
   - **Default:** `./workspace`
   - **Usage:** Specifies the directory meant to hold the execution artifacts. *(Note: This was originally used in the `fractals` execution framework, but remains here as part of the domain terminology in case the generated plan references it).*
   - Example: `--workspace=/tmp/isolated-work`
