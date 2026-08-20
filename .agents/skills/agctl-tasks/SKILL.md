---
name: agctl-tasks
description: Inspect and manage autonomous task queue without spaces via /agctl-tasks or /agctltasks.
---

# agctl Tasks Skill

Inspect and manage autonomous task queue.

When `/agctl-tasks` or `/agctltasks` is invoked:
1. Run `agctl tasks list` (or `agctl tasks run-pending` if requested).
2. Present list of pending, running, completed, and failed tasks with status badges.
