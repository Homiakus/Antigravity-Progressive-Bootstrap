---
name: premium-editorial-ui
description: >-
  Use this skill when the user wants to design, audit, or redesign a website, web app, dashboard, portfolio, SaaS, or desktop-like web UI with premium editorial, neo-Swiss, restrained-brutalist art direction inspired by high-end digital studios. Use it for visual hierarchy, typography, grid, whitespace, imagery, responsive composition, design tokens, states, and implementation guidance; do not use it as the primary skill for native Obsidian plugins or terminal TUIs.
compatibility: >-
  Agent Skills open standard. Best with screenshot/browser or repository access when auditing an existing product; still usable for greenfield design specifications.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Premium Editorial UI / Art Direction

## Objective

Create a distinctive, content-led, premium interface without copying a reference site or sacrificing usability, accessibility, responsiveness, or implementation quality.

## Activation boundary

### Use this skill when
- A site/app needs a premium editorial or neo-Swiss redesign.
- The user references ORGNZM, Awwwards-style editorial composition, restrained brutalism, or high-end digital-studio art direction.
- A dashboard or product UI needs stronger hierarchy, grid, type, whitespace, imagery, and visual rhythm.
- The task asks for a design system plus implementation/audit guidance.

### Prefer another skill / do not use when
- Native Obsidian plugin UI: use `obsidian-plugin-premium-ui-suite` as the primary skill.
- Terminal/CLI TUI: use `go-tui-premium-ui`.
- A narrowly scoped motion/theme-only pass: use `motion-icons-status-themes`.
- A purely responsive bug-fix pass with no broader art-direction goal: use `responsive-web-quality`.

## Inputs to establish

- Product purpose and audience
- Existing UI/repository/screenshots when available
- Primary user tasks
- Brand/content constraints
- Target platforms and accessibility/performance constraints

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Inspect before styling: reconstruct content hierarchy, primary tasks, existing design tokens/components, and constraints. Do not invent a new IA if the problem is only visual.
2. Define the art-direction thesis in 3–6 concrete principles. Separate reference-site principles from unique elements that must not be copied.
3. Establish semantic tokens, type scale, grid, spacing, surfaces, imagery rules, radii, and motion primitives before page-level polish.
4. Design critical screens and states using content-first hierarchy and controlled asymmetry. Use cards only when grouping/interaction semantics justify them.
5. Run a responsive pass based on content failure points and container size; preserve the editorial character on compact screens.
6. Run accessibility and interaction checks: focus, contrast, keyboard/touch, reduced motion, long content, localization, loading/error/empty states.
7. For implementation tasks, map each design change to components/files and preserve existing business logic unless a UX flaw requires a structural change.
8. Finish with visual-regression, performance, and remove-20%-decoration passes.

9. When a runnable browser implementation exists, compose with `web-visual-runtime-validation` so visual/art-direction decisions are checked against the actual rendered pages, not only source or mockups.

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
- `web-visual-runtime-validation` — Use `web-visual-runtime-validation` after layout-affecting implementation work to verify real rendering, screenshot regression, and selective semantic visual quality.

## Completion gates

- The result is recognizably premium without copying a branded layout or asset.
- Hierarchy still works in grayscale and without motion.
- No generic AI/SaaS card soup, gratuitous gradients, glass, or decoration.
- Critical flows remain obvious, accessible, responsive, and performant.
- Design decisions are expressed as reusable tokens/patterns rather than one-off CSS.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
