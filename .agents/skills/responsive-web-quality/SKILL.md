---
name: responsive-web-quality
description: >-
  Use this skill when auditing or fixing a website, web app, CMS, dashboard, or design system so it works intentionally across phones, tablets, split-screen, small laptops, desktop, ultrawide/4K, touch, mouse, keyboard, zoom, text scaling, orientation, and major browsers. Use it for intrinsic/fluid layout, container queries, responsive typography/media/tables/forms, accessibility, performance, and a rigorous QA matrix rather than breakpoint patching.
compatibility: >-
  Agent Skills open standard. Best with browser/screenshots or repository access for real viewport and component testing.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Responsive Web Quality

## Objective

Build resilient, content-driven responsive behavior that adapts to available space and capabilities, not a collection of device-specific patches.

## Activation boundary

### Use this skill when
- A site looks poor or breaks on some screen sizes.
- The user asks for mobile/tablet/ultrawide adaptation or cross-browser responsive QA.
- A dashboard/CMS has hardcoded widths, overflow, fragile breakpoints, tables, panels, or modals.

### Prefer another skill / do not use when
- Do not use only to choose visual art direction.
- Do not add breakpoints before identifying the intrinsic layout cause.
- Do not infer touch/hover solely from viewport width.

## Inputs to establish

- Critical pages/components
- Supported browsers/devices
- Existing breakpoints/tokens
- Real content extremes/locales
- Performance/accessibility requirements

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Continuously resize representative pages to find natural failure points; record issue, width/height, root cause, and affected component.
2. Fix primitives first: min-width, intrinsic sizing, grid/flex, overflow ownership, container bounds, logical properties, and content measures.
3. Centralize fluid type, spacing, gutters, size classes, and component container-query behavior.
4. Give complex components explicit responsive strategies: navigation, sidebars, tables, forms, charts, modals, editors, toolbars, filters, media.
5. Adapt to capabilities as well as size: hover, coarse pointer, safe areas, reduced motion, contrast, zoom/text scaling, virtual keyboard, orientation.
6. Optimize responsive media and rendering so compact devices do not pay for large-screen assets/effects unnecessarily.
7. Test representative fixed viewports plus continuous in-between widths, split-screen, 200–400% zoom where applicable, localization, and real content extremes.
8. Add visual/E2E/overflow regression checks for critical paths.

9. Compose with `web-visual-runtime-validation` whenever the app is runnable: use browser geometry/ARIA checks and screenshot evidence to discover actual failure intervals before adding breakpoints.

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
- `web-visual-runtime-validation` — Use `web-visual-runtime-validation` as the runtime evidence layer for overflow/overlap detection, pixel regression, and selective VLM review.

## Completion gates

- No unexplained page-level horizontal overflow.
- Breakpoints follow content failure points and components use intrinsic/container-driven behavior where suitable.
- Critical functions remain available on compact screens rather than being silently removed.
- Zoom, text scaling, keyboard/touch, safe areas, and virtual keyboard do not block operation.
- Performance and visual quality remain acceptable from compact mobile through ultrawide.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
