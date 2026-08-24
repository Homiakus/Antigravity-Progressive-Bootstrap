---
name: go-tui-premium-ui
description: >-
  Use this skill when designing, auditing, or refactoring a Go terminal UI—especially Bubble Tea, Lip Gloss, Bubbles, or other Charm-style applications. Use it for keyboard-first workflows, command/keymap architecture, responsive terminal layouts, themes/no-color fallbacks, status/progress/jobs/logs, forms, async state, terminal capability handling, SSH/tmux/Windows behavior, performance, and tests. Do not use it for ordinary web/GUI interfaces.
compatibility: >-
  Agent Skills open standard. Best with Go repository access and ability to run tests/benchmarks; framework-neutral where possible, with Bubble Tea/Charm guidance when present.
metadata:
  version: "2.0.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Premium Go TUI UI/UX

## Objective

Create a fast, calm, keyboard-first professional terminal application that exploits terminal strengths instead of imitating a GUI badly.

## Activation boundary

### Use this skill when
- The project is a Go TUI or CLI+TUI application.
- The user mentions Bubble Tea, Lip Gloss, Bubbles, Charm, terminal UX, SSH/tmux, or responsive terminal panes.
- A TUI needs a redesign, state/keymap refactor, status/progress/jobs/logs UX, or performance work.

### Prefer another skill / do not use when
- Do not use for a web dashboard or native GUI.
- Do not require Nerd Fonts, TrueColor, mouse, or 120×40 terminals for core usability.
- Do not optimize visual effects before state ownership and keyboard flow are correct.

## Inputs to establish

- Go repository/framework
- Core terminal workflows
- Minimum/supported terminal sizes
- Target OS/SSH/tmux environments
- Data volumes and async operations

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Map domain state, UI state, transient state, screens, commands, focus model, keymap, and async operations before changing styles.
2. Design keyboard-first workflows and a command/action registry so shortcuts, help, palettes, and UI actions reuse the same semantics.
3. Define responsive width/height classes from content needs and preserve selection/input state through resize.
4. Create a restrained semantic theme with no-color/16/256/truecolor degradation and Unicode/ASCII fallbacks where required.
5. Implement lists/tables/master-detail/forms/modals/status/jobs/logs with explicit focus and contextual key hints.
6. Ensure async work runs outside the update loop, is cancellable where appropriate, and reports deterministic/indeterminate progress correctly.
7. Audit terminal lifecycle: TTY detection, alt screen, raw mode, mouse opt-in, panic/exit restoration, SSH/tmux latency, Windows paths/ANSI/Unicode.
8. Benchmark/render-test large lists/logs, run state/keymap tests, and add golden tests for representative terminal sizes.

## Evidence and decision rules

- Separate **observed fact**, **inference**, and **recommendation**.
- Prefer the smallest change that solves the demonstrated problem.
- Preserve working business/domain behavior unless the requested UX/architecture change explicitly requires a behavior change.
- Do not claim verification that was not actually performed.
- For version-sensitive libraries, APIs, standards, browser/host behavior, or framework capabilities, verify current documentation when external access is available.
- When the task is implementation rather than advice, map recommendations to concrete components/files/contracts and include tests.
- Do not load every reference automatically. Use the routing section below to keep context focused.

## Progressive-disclosure routing

- `references/full-spec.md` — Read for the complete original master specification and edge-case requirements.
- `references/practice-upgrades.md` — Read when making architecture/implementation decisions or setting the final quality bar.
- `references/output-contract.md` — Read before producing a formal audit, specification, or implementation plan.
- `references/evals.md` — Read only when evaluating or tuning this skill itself.
- `references/sources.md` — Read when a recommendation depends on current external standards, APIs, browser/host behavior, or security guidance.

## Completion gates

- Core workflows are possible without a mouse and remain understandable without color.
- Focus and selection are distinct and always visible.
- Resize never discards editing/selection/domain state.
- Async operations never block the UI loop and clean up correctly.
- The TUI remains usable on realistic small terminals and remote/limited-color environments.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
