---
name: agctl
description: Antigravity Control Plane assistant and slash-command handler. Handles commands like /agctl, /agctl-doctor, /agctldoctor, /agctl-run, /agctlrun, /agctl-replan, /agctlreplan, /agctl-dashboard, /agctldashboard, /agctl-loop, /agctlloop, /agctl-tasks, /agctltasks, /agctl-mcp, /agctlmcp, /agctl-security, /agctlsecurity.
---

# agctl — Antigravity Control Plane Skill

This skill integrates `agctl` directly into the Antigravity assistant conversation and handles all unified slash-commands without spaces (e.g. `/agctl-doctor`, `/agctldoctor`, `/agctl-run`, `/agctlrun`, `/agctl-replan`, `/agctlreplan`, `/agctl-dashboard`, `/agctldashboard`, `/agctl-loop`, `/agctlloop`, `/agctl-tasks`, `/agctltasks`).

When any of these slash commands are triggered, execute the corresponding `agctl` CLI command using `run_command` in the workspace and present a structured, actionable response.

---

## Single-Word & Hyphenated Slash Commands

| Slash Command (Without Spaces) | Legacy Variant | Action | Command to Run |
| :--- | :--- | :--- | :--- |
| **`/agctl-doctor`** / **`/agctldoctor`** | `/agctl doctor` | Full health diagnostic with MCP probe | `agctl doctor --probe-mcp` |
| **`/agctl-run <task>`** / **`/agctlrun <task>`** | `/agctl run <task>` | Run task with Adaptive DAG replanning | `agctl plan run --workspace . --prompt "<task>"` |
| **`/agctl-replan`** / **`/agctlreplan`** | `/agctl replan` | Launch adaptive replanning supervisor | `agctl replan run` |
| **`/agctl-dashboard`** / **`/agctldashboard`** | `/agctl dashboard` | Start local observability web dashboard | `agctl dashboard serve --listen 127.0.0.1:8787` |
| **`/agctl-loop`** / **`/agctlloop`** | `/agctl loop` | Check or set autonomous loop mode | `agctl loop status` |
| **`/agctl-tasks`** / **`/agctltasks`** | `/agctl tasks` | List pending and running tasks in queue | `agctl tasks list` |
| **`/agctl-mcp`** / **`/agctlmcp`** | `/agctl mcp` | Inspect and verify MCP servers | `agctl mcp list` |
| **`/agctl-security`** / **`/agctlsecurity`** | `/agctl security` | Run security governance audit | `agctl security audit` |
| **`/agctl-status`** / **`/agctlstatus`** | `/agctl status` | Quick status & health diagnosis | `agctl doctor` |
| **`/agctl-plan`** / **`/agctlplan`** | `/agctl plan` | Show current execution plan / DAG | `agctl plan list` |
| **`/agctl-init`** / **`/agctlinit`** | `/agctl init` | Initialize agctl in current workspace | `agctl project init` |

---

## Behavior Guidelines

1. **Locate Binary**: Use `agctl` directly if installed in PATH, or `./agctl.exe` in the workspace directory.
2. **Execute & Parse**: Run the required subcommand via `run_command` and format the output cleanly with GitHub Markdown alerts (`[!NOTE]`, `[!TIP]`, `[!WARNING]`), status tables, and next recommended actions.
3. **Autonomous Execution (`/agctlrun` / `/agctl-run <prompt>`)**:
   - Run the plan via `agctl plan run --workspace . --prompt "<prompt>"`.
   - In interactive mode, continue through `autonomous-completion-loop` until verified.
4. **Dashboard (`/agctldashboard` / `/agctl-dashboard`)**:
   - Start the background dashboard server with `agctl dashboard serve --listen 127.0.0.1:8787` (daemon mode) and provide the clickable link `http://127.0.0.1:8787`.
