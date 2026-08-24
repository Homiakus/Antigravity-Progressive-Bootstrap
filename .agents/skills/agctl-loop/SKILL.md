---
name: agctl-loop
description: Inspect and configure autonomous completion loop mode without spaces via /agctl-loop or /agctlloop.
---

# agctl Loop Skill (Multi-Platform Autonomous Completion Loop)

Inspect and configure the Autonomous Completion Loop across all supported agentic environments.

## Supported Environments
- **Antigravity IDE / AGY CLI**: Invoked via slash command `/agctl-loop` or lifecycle hooks (`hooks.json`).
- **DeepSeek Harness (DSH)**: Executed via `pwsh` sidecar `agctl loop status` or through `agctl-dsh`.
- **Cursor Agent**: Run `agctl loop status` / `agctl loop enable <profile>` in Terminal or Cursor Agent commands.
- **Claude Code / Roo Code / CLI**: Run `agctl loop status` / `agctl loop enable <profile>`.

## Profiles & Usage
- `standard`: 25 max executions with guarded permission checks.
- `deep`: 50 max executions with guarded permission checks (Recommended).
- `until-done` / `unlimited`: 0 (unlimited) executions until Definition of Done is verified.
- `unrestricted`: 0 (unlimited) executions with unrestricted permissions (`--yes-i-accept-risk`).

## When `/agctl-loop` or `/agctlloop` is invoked:
1. Run `agctl loop status` (or `agctl loop enable <profile>` if arguments provided).
2. Present current loop profile, execution limits, Stop-hook policy, and verified completion contract.

