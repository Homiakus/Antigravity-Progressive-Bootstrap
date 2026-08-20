---
name: agctl-loop
description: Inspect and configure autonomous completion loop mode without spaces via /agctl-loop or /agctlloop.
---

# agctl Loop Skill

Inspect and configure autonomous completion loop mode.

When `/agctl-loop` or `/agctlloop` is invoked:
1. Run `agctl loop status` (or `agctl loop enable <profile>` if arguments provided).
2. Present current loop profile (`unrestricted`, `safe`, `review`), execution limits, and stop gate policies.
