---
name: agctltasks
description: Inspect and manage autonomous task queue without spaces via /agctltasks.
---

# agctl Tasks Skill

Inspect and manage autonomous task queue.

When `/agctltasks` or `/agctl-tasks` is invoked:
1. Run `agctl tasks list` (or `agctl tasks run-pending` if requested).
2. Present list of pending, running, completed, and failed tasks with status badges.
