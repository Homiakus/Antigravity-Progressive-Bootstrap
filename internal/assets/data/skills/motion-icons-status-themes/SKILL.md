---
name: motion-icons-status-themes
description: >-
  Use this skill when improving an existing interface through motion design, microinteractions, icon behavior, status/progress feedback, semantic colors, light/dark themes, accent-color customization, and accessibility. Use it for systematic feedback and appearance architecture rather than decorative animation. It complements broader UI redesign skills and should preserve product semantics, performance, reduced-motion behavior, and theme compatibility.
compatibility: >-
  Agent Skills open standard. Best with UI/repository access for auditing tokens, states, and component behavior.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Motion, Icons, Status, Color & Themes

## Objective

Make interfaces feel responsive and alive by using motion, iconography, status feedback, semantic color, and themes as information—not decoration.

## Activation boundary

### Use this skill when
- The user asks to add/improve animations, icon motion, status bars, progress, feedback, light/dark mode, or accent colors.
- An interface has abrupt state changes, unclear async feedback, inconsistent icons, or hardcoded colors.
- A design system needs semantic tokens for themes and state feedback.

### Prefer another skill / do not use when
- Do not use as the sole skill for a full information-architecture redesign.
- Do not add motion to a data-heavy tool before correctness, hierarchy, and responsiveness are stable.
- Do not introduce plugin-specific theme controls where the host application should own appearance.

## Inputs to establish

- Current component/state inventory
- Existing token/theme system
- Async/loading/error/success workflows
- Target platforms and input methods
- Accessibility constraints

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Audit all state transitions and identify where users lack feedback; prioritize state changes over decorative opportunities.
2. Create semantic color and motion tokens; separate accent colors from success/warning/danger/info semantics.
3. Unify icon family, optical size, stroke, labels, and contextual animation rules.
4. Specify component states: default, hover, focus-visible, active, selected, disabled, loading, success, error, offline/conflict where applicable.
5. Design progress/status patterns based on determinacy and scope: inline, local component, view-level, or global status.
6. Implement light/dark/system theme behavior and accent derivation without hardcoded component colors; prevent theme flash on load when relevant.
7. Add reduced-motion, forced-colors/high-contrast, touch/no-hover, and keyboard behavior.
8. Profile expensive animations and remove effects that cause layout/repaint churn or attention fatigue.

9. When motion/theme changes are implemented in a runnable web app, compose with `web-visual-runtime-validation`; capture deterministic theme states and disable animation only for static regression captures, while testing motion separately.

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
- `web-visual-runtime-validation` — Use `web-visual-runtime-validation` to verify real light/dark/accent states, loading/status layouts, and screenshot regressions.

## Completion gates

- Every animation has a trigger, purpose, duration/easing, and reduced-motion behavior.
- No important state is communicated by color alone.
- Accent never replaces semantic danger/warning/success meaning.
- Theme variants retain readable hierarchy and focus visibility.
- Motion does not block work, create layout shifts, or animate constantly without purpose.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
