---
name: agctlloop
description: Inspect and configure autonomous completion loop mode without spaces via /agctlloop.
---

# agctl Loop Skill

Inspect and configure autonomous completion loop mode.

When `/agctlloop` or `/agctl-loop` is invoked:
1. Run `agctl loop status` (or `agctl loop enable <profile>` if arguments provided).
2. Present current loop profile (`unrestricted`, `safe`, `review`), execution limits, and stop gate policies.
