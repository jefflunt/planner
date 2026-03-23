# planner 

**A recursive agentic task orchestrator**

`planner` accepts a high-level task description and breaks it down iteratively using an LLM. Unlike execution-focused agents, `planner` focuses strictly on decomposition. It guarantees that the resulting leaf nodes represent *single-file operations* by recursively polling the user for clarification and the LLM for decomposition until every branch reaches an actionable state.

This project was inspired by the design of [TinyAGI/fractals](https://github.com/TinyAGI/fractals) but built entirely in Go and focused purely on building the task tree rather than executing it.

## Quick Start

You can build the two available binaries (a rich TUI or a standard CLI):

```bash
go build -o bin/plan-tui ./cmd/plan-tui
go build -o bin/plan-cli ./cmd/plan-cli
```

### Using the TUI

Launch the interactive Bubble Tea terminal UI to watch the tree build and answer LLM clarification questions in real-time:

```bash
./bin/plan-tui "Build a web scraper"
```

### Using the CLI

Run the standard CLI (useful for embedding or logging):

```bash
./bin/plan-cli plan "Build a web scraper"
```

List the current state of the plan:
```bash
./bin/plan-cli list
```

## Agent Documentation

This repository uses a Progressive Disclosure documentation pattern for AI Agents. 

If you are an AI agent working in this repository, **start by reading [`agent_docs/README.md`](agent_docs/README.md)**. 

The `agent_docs/` directory contains all architectural decisions, workflow details, and project conventions needed to successfully navigate and modify this codebase.
