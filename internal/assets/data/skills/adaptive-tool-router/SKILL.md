---
name: adaptive-tool-router
description: Selects the smallest sufficient set of Agent Skills, MCP servers, native tools, verification methods, and subagents before non-trivial work. Use for coding, debugging, research, browser, GitHub, documentation, editorial and multi-domain tasks where capability selection matters.
---

# Adaptive Tool Router

This skill is the capability planner. It does not replace domain skills.

## Route contract

Before substantive work on a non-trivial request:

1. classify the requested outcome and constraints;
2. decide whether external/current state is required;
3. select the smallest sufficient set of specialized skills;
4. select MCP only when it adds a needed capability;
5. select verification appropriate to the risk and task;
6. re-route automatically if a selected capability is unavailable or ineffective.

An empty skill or MCP list is valid.

## Preferred routing

- Current/version-specific library docs -> Context7 when configured.
- Browser user flows/E2E -> Playwright when configured.
- Console/network/runtime/performance diagnosis -> Chrome DevTools when configured.
- Remote GitHub repository/issue/PR state -> GitHub MCP when configured.
- Editing/rewrite/proofreading/publication -> `editorial-quality-director`.
- Delegated implementation/debug/refactor -> `autonomous-completion-loop` when enabled, plus focused engineering skills.
- Local file/code work -> native editor/workspace/terminal tools first.

## Progressive disclosure

Use skill names/descriptions for discovery. Read only the selected SKILL.md files. Do not preload every skill.

## Visibility

The Go PreInvocation router hook supplies the active visibility mode. In transparent mode, output exactly one concise route line before substantive prose:

`Route: skills=[...]; MCP=[...]; verification=[...]`

## Security

Routing never overrides explicit user constraints, permission denials, credentials, or safety boundaries.
