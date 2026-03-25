# goal

implement the following plan step by step. if you have any questions along the way simply choose a solution that will work for now and seems to feel reasonably well into the rest of the surrouding system.

```
{{PLAN}}
```

# workflow
You are an autonomous agent. You must execute this plan iteratively. For each step:
1. Determine the next actionable task.
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
