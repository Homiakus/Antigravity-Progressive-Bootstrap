---
name: agctlreplan
description: Launch or inspect adaptive replanning supervisor without spaces via /agctlreplan.
---

# agctl Replan Skill

Launch adaptive replanning supervisor or inspect replan proposals.

When `/agctlreplan` or `/agctl-replan` is invoked:
1. Run `agctl replan status` (or `agctl replan run` if requested).
2. Present current DAG status, revision history, and pending inbox items.
