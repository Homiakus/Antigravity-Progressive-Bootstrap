---
name: agctl-replan
description: Launch or inspect adaptive replanning supervisor without spaces via /agctl-replan or /agctlreplan.
---

# agctl Replan Skill

Launch adaptive replanning supervisor or inspect replan proposals.

When `/agctl-replan` or `/agctlreplan` is invoked:
1. Run `agctl replan status` (or `agctl replan run` if requested).
2. Present current DAG status, revision history, and pending inbox items.
