---
name: markdown-live-preview-editor
description: >-
  Use this skill when defining, designing, or auditing an online Markdown article editor or CMS writing surface where Markdown remains the canonical source and live production-faithful preview is preferred. Use it for Markdown dialect, editor/AST/source maps, synchronized preview, paste/media/link/table/code workflows, metadata, autosave/recovery/versioning, validation, publishing, security, mobile, accessibility, performance, collaboration boundaries, and implementation planning.
compatibility: >-
  Agent Skills open standard. Best with editor/CMS repository access; library/API choices should be verified against current versions before implementation.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Markdown Live Preview Editor

## Objective

Preserve readable, portable Markdown while making authoring as frictionless as a modern visual editor through live preview, source mapping, smart insertion, and safe publishing.

## Activation boundary

### Use this skill when
- The user needs an online Markdown article editor, live preview, authoring workflow, or Markdown CMS editor.
- The task involves Markdown dialect, CodeMirror/Monaco-style editor choice, AST/source mapping, preview fidelity, paste/media, or publishing safety.
- A current editor needs functional/UX architecture rather than only visual styling.

### Prefer another skill / do not use when
- Do not turn Markdown into opaque proprietary blocks by default.
- Do not add MDX/realtime collaboration/AI unless product requirements justify the complexity.
- For the surrounding content-operations/admin system, pair with `cms-editorial-workflow-ux`.

## Inputs to establish

- Article/content types
- Required Markdown extensions
- Production renderer/styling
- CMS/publishing backend
- Collaboration/offline/security requirements

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Define users, article types, canonical source model, and the exact Markdown dialect before choosing editor components.
2. Design one document state with parser/AST/source ranges feeding validation, outline, links, transforms, and production-faithful preview.
3. Specify Editor/Split/Preview modes, source↔preview navigation, scroll anchoring, outline, problems panel, focus mode, and responsive behavior.
4. Optimize high-frequency authoring actions: paste rich text, paste/drag images, internal links, tables, code blocks, callouts, metadata, and keyboard commands.
5. Design autosave/local recovery, revisions, optimistic concurrency, draft-vs-live states, scheduling, publication validation, and rollback.
6. Secure rendering: sanitize untrusted HTML/URLs/embeds, use allowlists where appropriate, and keep preview interactions from performing production side effects.
7. Preserve round-trip source and clean diffs; avoid destructive reformatting or serializers that rewrite unrelated Markdown/frontmatter.
8. Benchmark long documents, many images/heavy blocks, IME/paste/mobile, preview latency, and test parser/render/security/source-map invariants.

9. For a runnable browser editor, compose with `web-visual-runtime-validation` to test real editor/split/preview rendering, toolbar/virtual-keyboard states, and preview visual regressions.

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
- `web-visual-runtime-validation` — Use `web-visual-runtime-validation` for actual browser verification of editor/preview states and responsive authoring layouts.

## Completion gates

- Markdown remains readable outside the CMS and is not silently normalized on unrelated edits.
- Preview and production use the same or provably equivalent rendering pipeline.
- Author input survives network loss, refresh, conflicts, and renderer failure.
- Paste, media, links, and tables minimize mode switches and manual syntax burden.
- Untrusted Markdown/HTML/embeds cannot turn preview into an XSS or side-effect surface.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
