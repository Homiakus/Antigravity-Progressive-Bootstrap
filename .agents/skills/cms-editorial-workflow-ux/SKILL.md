---
name: cms-editorial-workflow-ux
description: >-
  Use this skill when auditing, designing, or refactoring a CMS/admin/editorial system so authors, editors, reviewers, and administrators can create, find, edit, preview, relate, review, publish, roll back, and maintain content safely and quickly. Use it for content models, editor workflows, search/filtering, media, SEO, versioning, permissions, publishing, content health, mobile, accessibility, and operational safety—not just a visual admin redesign.
compatibility: >-
  Agent Skills open standard. Best with repository, schema, CMS screenshots, and representative editorial workflows when available.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# CMS Editorial Workflow UX

## Objective

Reduce navigation and cognitive overhead while making publication, rollback, permissions, versioning, and content relationships explicit and safe.

## Activation boundary

### Use this skill when
- A CMS/admin interface is difficult, slow, or risky for editors.
- The task involves content modeling, publishing workflows, previews, media libraries, SEO, versioning, or content health.
- A product needs a professional editorial workspace rather than a generic admin dashboard.

### Prefer another skill / do not use when
- For a standalone Markdown editor specification, prefer `markdown-live-preview-editor` and use this skill for the surrounding CMS lifecycle.
- For purely visual art direction with no editorial workflow problem, use a design-system skill instead.

## Inputs to establish

- Content types and relationships
- User roles and permissions
- Publishing lifecycle
- Existing CMS UI/schema/repository
- Representative content volumes and failure cases

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Reconstruct the content model, canonical source of truth, roles, publication lifecycle, and high-frequency editorial journeys before redesigning screens.
2. Map each major workflow with action count, navigation depth, wait states, error opportunities, and irreversible operations.
3. Simplify IA around editor intent: find/create/edit/preview/review/publish/maintain. Keep operational alerts actionable.
4. Design dense content lists, global search, progressive filters, saved views, bulk actions, and direct navigation to affected content.
5. Design editing around context: autosave, live preview, relationships, media, metadata, SEO, validation, draft-vs-live clarity, and history.
6. Make destructive and externally visible actions safe through versioning, optimistic concurrency, dependency/usage checks, audit logs, and rollback.
7. Design permissions, review, scheduling, localization, mobile emergency editing, and content-health workflows only to the complexity actually required.
8. Finish with task-based usability tests, permission tests, conflict/failure injection, accessibility, and performance checks.

9. For runnable web CMS work, compose with `web-visual-runtime-validation` on critical editorial states (lists, editor, preview, publish, errors, mobile) so the agent verifies the rendered result after changes.

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
- `web-visual-runtime-validation` — Use `web-visual-runtime-validation` for real-browser verification of critical CMS workflows and visual regressions.

## Completion gates

- Editors can tell saved, draft, review, scheduled, and live states apart at a glance.
- Canonical content and relationships are not duplicated invisibly across UI and storage.
- Dangerous actions explain scope and are recoverable where feasible.
- Search, preview, and quick edits minimize navigation overhead.
- Permissions and validation are enforced authoritatively server-side even when mirrored in UI.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
