---
name: obsidian-plugin-premium-ui-suite
description: >-
  Use this skill when auditing, designing, or refactoring UI/UX for an Obsidian plugin. Use it for native-first workspace/editor/ribbon/commands/context menus/settings/modals/status UI, Obsidian CSS variables and community-theme compatibility, container-responsive panes, desktop/mobile behavior, knowledge-management workflows, motion, accessibility, lifecycle cleanup, and implementation planning. Prefer this over generic web UI skills whenever the target is an Obsidian plugin.
compatibility: >-
  Agent Skills open standard. Best with Obsidian plugin repository access. Guidance assumes current Obsidian plugin APIs and should be verified against official docs when API details are version-sensitive.
metadata:
  version: "2.0.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Premium UI/UX Suite for Obsidian Plugins

## Objective

Make a plugin feel like a polished native Obsidian capability with a restrained premium character, without creating a foreign web app inside the host.

## Activation boundary

### Use this skill when
- The target is an Obsidian community/private plugin.
- The task involves ItemView, WorkspaceLeaf, ribbon, commands, settings, modals, status bar, editor extensions, mobile, or theme compatibility.
- A plugin dashboard/CRM/People OS/project manager/knowledge tool needs UX and visual-system work.

### Prefer another skill / do not use when
- Do not apply generic web UI patterns that ignore Obsidian host behavior.
- Do not globally restyle Obsidian or other plugins.
- Do not duplicate Obsidian light/dark/accent controls unless a plugin-owned preview genuinely needs independent appearance.

## Inputs to establish

- Plugin repository and manifest
- Obsidian surfaces used
- Core user workflows
- Desktop/mobile support target
- Data model and vault integration

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Inventory all Obsidian surfaces and lifecycle hooks: Plugin, commands, ribbon, ItemView/leaf, editor extensions, post-processors, settings, modals, menus, status, events, intervals, observers.
2. Map user journeys from the note/editor outward. Prefer instant/contextual interactions before dedicated workspace views.
3. Reuse host conventions and business actions: command-first architecture, native menus/modals/settings/icons, scoped CSS, CSS variables, and community-theme compatibility.
4. Design custom views with container-driven responsive modes because leaf width—not monitor width—is the primary constraint.
5. Keep editor integrations minimal, low-latency, layout-stable, and compatible with keyboard/touch/mobile.
6. For knowledge/CMS-style plugins, keep note/frontmatter/links/Bases integration explicit and avoid hidden duplicate stores unless architecture truly requires one.
7. Audit desktop/mobile API differences, event/DOM/framework cleanup, startup cost, large-vault behavior, and plugin disable/enable lifecycle.
8. Finish with theme torture tests, focus/accessibility checks, large-vault performance, split-pane/mobile scenarios, and native-vs-brand visual review.

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

- The plugin looks coherent across Default and materially different community themes.
- All plugin CSS is scoped and uses host semantics unless a data visualization requires owned colors.
- No frequent workflow needlessly pulls users out of the note/editor context.
- Views/events/observers/framework roots are cleaned up on unload.
- Desktop/mobile/split-pane/large-vault behavior is intentional and tested.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
