## Instructions
1. Implement the assigned task ONLY.
2. Create tests consistent with current testing patterns.
3. Run quality checks (tests, lint, etc.).
4. If checks pass, commit ALL of your changes with the task name.
5. Use `ponder_complete_task` to mark the task as finished, or `ponder_report_task_blocked` if you encounter an unresolvable issue.

## Tooling
These MCP tools are provided for task lifecycle management and should be used to interact with the Ponder system:
- `ponder_list_features`: List all features.
- `ponder_list_tasks`: List tasks with optional filters.
- `ponder_get_available_tasks`: Get tasks that are ready to work on.
- `ponder_complete_task`: Complete a task by setting its status to completed.
- `ponder_report_task_blocked`: Report a task as blocked and provide a reason.
