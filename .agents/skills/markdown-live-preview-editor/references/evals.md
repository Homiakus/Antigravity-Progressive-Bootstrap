# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Define the full functionality and architecture of an online Markdown article editor with live preview.
- **SHOULD trigger** — Audit our CodeMirror Markdown CMS editor, paste behavior, AST/source mapping, autosave, and publishing.
- **SHOULD trigger** — Design source-to-preview sync, media insertion, internal links, tables, code blocks, and recovery.
- **SHOULD NOT trigger** — Redesign the entire CMS permission and media administration system.
- **SHOULD NOT trigger** — Make a static marketing page look premium.
- **SHOULD NOT trigger** — Audit Go memory allocations.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
