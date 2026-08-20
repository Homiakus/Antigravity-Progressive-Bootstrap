# Changelog

## 3.2.1

- Audited against current Antigravity 2.0/CLI/IDE docs and MCP 2026-07-28.
- Fixed custom-agent frontmatter and flat/folder/plugin-agent discovery.
- Reworked MCP probe for stateless 2026-07-28 plus 2025-11-25 fallback and modern result validation.
- Fixed headless success semantics: terminal result status and soft permission denial are authoritative, not process exit code alone.
- Split desktop `/goal` composition from headless until-done prompting.
- Required `fullyIdle` at verified Stop completion.
- Registered managed CLI hooks as an Antigravity CLI plugin.
- Hardened skill mirrors/provenance to preserve and hash auxiliary resources.
- Removed undocumented workspace-rule activation frontmatter.
- Made experimental `gopls mcp` explicit opt-in, including project profiles.
- Added compatibility diagnostics for legacy MCP URL fields and invalid agent schemas.

## 3.2.0

- Added bounded Adaptive DAG Replanning.
- Added machine-readable agent replan proposal contract with evidence/confidence/risk gates.
- Added automatic final-failure recovery chain: diagnose -> repair -> reverify.
- Added `superseded` task state and downstream dependency rewiring.
- Added no-progress detector using failure signatures and repair-depth/revision budgets.
- Added plan revision/status/dynamic-node history.
- Added optional parallel Git worktree lanes for independent write actions discovered by read-only nodes, plus integration node.
- Added `agctl replan status|config|enable|disable|apply|run|inbox`.
- Added `agctl plan history` and `--static` escape hatch for plan/task execution.
- Dashboard and doctor now expose adaptive replanning state, revisions and dynamic nodes.
- Extended doctor self-test with synthetic failed-node recovery and dependency rewiring.
- Added 3.1 -> 3.2 migration launcher.
- Preserved 3.1 task/plan/config compatibility.

## 3.1.0

- Added deterministic multi-agent DAG planner (`agctl plan create/list/show/enqueue/run`).
- Added capability relevance ranking (`agctl capabilities rank`).
- Added dependency-aware, resource-aware task scheduling with CPU/build/browser budgets.
- Added inter-process task claim locks to prevent duplicate execution by concurrent supervisors.
- Added automatic bounded retries for failed headless DAG nodes.
- Added downstream `blocked` state when a dependency fails permanently.
- Added security governance score for installed MCP/skills/plugins/provenance (`agctl security audit`).
- Added MCP Registry pre-install security scoring and low-score installation gate.
- Added local observability dashboard with JSON APIs and Prometheus-compatible `/metrics`.
- Added task/plan lifecycle telemetry events.
- Updated MCP probe to current protocol `2026-07-28` with fallback to `2025-11-25`.
- Fixed project profiler false-positive `ai` detection caused by `.agents` and other generated directories.
- Expanded `doctor --self-test` with planner DAG, destructive MCP denial and security-score checks.
- TUI exposes DAG planning, resource budgets/retries, capability ranking, security audit and dashboard.

## 3.0.0

- Plugin Manager with global/workspace scope and provenance locks.
- Official MCP Registry REST client with search/detail/install plans.
- Live MCP protocol probe: initialize, tools/list, resources/list, prompts/list.
- Capability Registry covering skills, MCP, agents, plugins, workflows and native capabilities.
- Custom Agent/Subagent Orchestrator with architecture, implementation, testing, security, review and research roles.
- Project Profiler and `project init` generating AGENTS.md, rules, workflows, agents and project MCP.
- Native `/goal` command composition plus verified completion gate.
- Semantic MCP/tool risk engine with configurable policy.
- Persistent headless Task Supervisor with concurrency and watchdog.
- Git Worktree Manager.
- Workflow Manager.
- Supply-chain provenance locks and verification.
- Local hook/task telemetry.
- Expanded Doctor with optional live MCP probe.
- Memory MCP removed from stable full defaults.
- Google Gemini Skills moved from recommended global pack to opt-in.
- TUI expanded to expose Control Plane 3.0 functionality.
- Dedicated `migrate v2` path.
