# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Redesign our CMS so editors can find, preview, review, schedule, publish, and roll back articles safely.
- **SHOULD trigger** — Audit our content model, media library, permissions, SEO, drafts, and version history.
- **SHOULD trigger** — Our admin is full of cards and editors get lost—design the actual editorial workflow.
- **SHOULD NOT trigger** — Build only a Markdown live preview editor with no CMS workflow.
- **SHOULD NOT trigger** — Make the marketing site more editorial-looking.
- **SHOULD NOT trigger** — Audit Go heap allocations.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
