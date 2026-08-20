---
name: agctlrun
description: Execute task with Adaptive DAG replanning and autonomous completion loop without spaces via /agctlrun <task>.
---

# agctl Run Skill

Execute a task with full autonomous lifecycle, adaptive replanning, and verification.

When `/agctlrun <prompt>` or `/agctl-run <prompt>` is invoked:
1. Run `agctl plan run --workspace . --prompt "<prompt>"` (or `.\agctl.exe plan run ...`).
2. Follow through `autonomous-completion-loop` to implement, test, and verify the outcome.
