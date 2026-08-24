# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Audit all loading, success, error, sync, icon animations, and theme colors in this app.
- **SHOULD trigger** — Add accent-color settings and a proper light/dark/system token system.
- **SHOULD trigger** — Our UI changes state abruptly; design microinteractions and progress/status feedback.
- **SHOULD NOT trigger** — Redesign the information architecture of the whole CMS.
- **SHOULD NOT trigger** — Create a premium landing-page layout.
- **SHOULD NOT trigger** — Fix a single CSS media query.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
