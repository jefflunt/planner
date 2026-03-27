# goal

First, analyze the provided task, details, and overall plan structure. Create a consolidated, complete implementation plan that accounts for all TODOs, dependencies, and necessary code changes. This implementation plan MUST be declarative, avoid asking questions, and proactively infer technical details from the current codebase context.

Then, execute this implementation plan step-by-step. If you encounter unforeseen issues, adapt the plan iteratively to ensure the goal is met.

## Task
{{TASK}}

## Details
{{DETAILS}}

## Diagram
{{ASCII_DIAGRAM}}

## Overall Plan Structure
```
{{PLAN_STRUCTURE}}
```

# workflow
You are an autonomous agent. You must execute this plan iteratively. For each step:
1. Determine the next actionable task based on your consolidated implementation plan.
2. Execute the necessary commands or file operations to complete that task.
3. Observe the output of your actions.
4. If successful, move to the next task.
5. If unsuccessful, analyze the failure, adjust your approach, and try again.
6. Continue this loop until all steps in the plan are completed.

# cleanup and verification

after you believe you've achieved the goal, do the following verification steps:

- backfill any missing unit tests you feel are appropriate
- run all the tests and fix any failures
- if there is an `agent_docs/` folder in this repository, then update it with any changes
- Once all verification passes, provide a final confirmation that the goal is achieved.
