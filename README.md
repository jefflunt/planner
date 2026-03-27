See [agent_docs/README.md](agent_docs/README.md) for full documentation.

### Configuration
`plan-tui` can be configured using a YAML file located at `~/.planner/config.yml`.

Example `~/.planner/config.yml` for Gemini:

```yaml
plans_dir: "~/.planner/plans"
llm:
  provider: "gemini"
  model: "gemini-3.1-flash-lite-preview"
  api_key: "YOUR_API_KEY_HERE"
```

Example `~/.planner/config.yml` for GitHub Copilot:

```yaml
plans_dir: "~/.planner/plans"
llm:
  provider: "copilot"
  model: "claude-sonnet-4.5"
```

Example `~/.planner/config.yml` for opencode:

```yaml
plans_dir: "~/.planner/plans"
llm:
  provider: "opencode"
  model: "google/gemini-3.1-pro-preview"
```

### Atlassian Integration
If configured, the planner automatically fetches content from Jira or Confluence URLs found in tasks or details.

```yaml
atlassian:
  base_url: "https://your-atlassian-instance.atlassian.net"
  user: "your-email@example.com"
  api_key: "YOUR_API_TOKEN_HERE"
```

Example usage:
```bash
./bin/plan-tui "Implement task from https://your-atlassian-instance.atlassian.net/browse/PROJ-123"
```

See [agent_docs/config.md](agent_docs/config.md) for full configuration options.
