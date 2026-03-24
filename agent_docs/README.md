# planner — Agent Documentation Index

**planner** is a recursive agentic task orchestrator written in Go. It accepts a high-level task description and breaks it down iteratively using an LLM. Unlike execution-focused agents, `planner` focuses strictly on decomposition. It guarantees that the resulting leaf nodes represent *single-file operations* by recursively polling the user for clarification and the LLM for decomposition until every branch reaches an actionable state.

This project was inspired by the design of [TinyAGI/fractals](https://github.com/TinyAGI/fractals) but built entirely in Go and focused purely on building the task tree rather than executing it.

## Quick Start

You can build the `plan-tui` binary using the included build script:

```bash
./script/build
```

Launch the interactive Bubble Tea terminal UI to watch the tree build and answer LLM clarification questions in real-time:

```bash
./bin/plan-tui "Build a web scraper"
```

Or pass a larger prompt via STDIN:

```bash
cat prompt.txt | ./bin/plan-tui
```

---

## How to Use This Documentation

This folder follows **Progressive Disclosure** principles — show what exists and its retrieval cost first, let the agent decide what to fetch based on relevance and need. Start here, then read only the detail files relevant to your task.

> **Maintaining these docs:** When you add, remove, or significantly change code in this repo, update the relevant file(s) below. Keep each file focused on its layer. If a detail file grows beyond ~200 lines, split it. Always update this index when adding new files.

| File | What it covers | Read when… |
|------|---------------|------------|
| **This file** | Repo overview, file map, key facts | Always — start here |
| [`architecture.md`](architecture.md) | The generic task tree, TUI vs CLI separation, and state management | Changing core logic, adding new UI components |
| [`planning_workflow.md`](planning_workflow.md) | Step-by-step walkthrough of how tasks are analyzed, decomposed, and how the planner yields for user input | Changing the LLM interaction loop or Actionable heuristic |
| [`building.md`](building.md) | Build process and commands | Building the binaries |
| [`config.md`](config.md) | Configuration options (state files, workspaces) | Changing CLI flags or configuration options |
| [`plans/`](plans/) | Design plans for future or in-progress features | Starting a significant new feature (e.g. real LLM integration) |

---

## Repo at a Glance

```
planner/
├── bin/                       ← Compiled output directory
├── cmd/
│   └── plan-tui/              ← Interactive Terminal UI executable
├── pkg/
│   ├── config/                ← YAML Configuration parsing
│   ├── llm/                   ← Gemini LLM Client
│   ├── planner/               ← Core orchestrator logic (tree, node, loop)
│   ├── version/               ← Binary version definitions
│   └── tui/                   ← Bubble Tea UI components
├── script/                    ← Build, test, and automation scripts
├── agent_docs/                ← this documentation tree
└── planner-state.json         ← default state persistence (gitignored)
```

**Module:** `planner`  
**Go version:** 1.22+  

---

## Key Facts

- **Actionable Heuristic:** A leaf node is *only* actionable if it describes the creation, deletion, or editing of a single file on disk. The LLM must enforce this.
- **No Max Depth:** The planner does not rely on arbitrary depth limits. It continues to decompose infinitely until the LLM returns `Actionable` for all branches.
- **Yielding to User:** If a task is unclear, the LLM returns `AskUser`. The planner halts execution for that branch, bubbles a prompt up to the UI (via Go channels), waits for user input, appends the answer to the task's context, and retries.
- **Interface:** The logic is encapsulated in `pkg/planner`, driven by a rich interactive Bubble Tea interface (`plan-tui`).
- **Persistence:** The entire task tree is saved as a structured JSON file after every state mutation, allowing planning sessions to be resumed seamlessly.
