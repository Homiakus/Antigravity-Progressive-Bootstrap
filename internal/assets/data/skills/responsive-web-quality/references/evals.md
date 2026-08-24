# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Audit every viewport from 320px to ultrawide and fix our fragile responsive layout.
- **SHOULD trigger** — Our dashboard breaks in split-screen and tables are unusable on tablet.
- **SHOULD trigger** — Create a responsive QA matrix including zoom, touch, keyboard, landscape, and cross-browser.
- **SHOULD NOT trigger** — Choose a brand color palette.
- **SHOULD NOT trigger** — Audit publishing permissions in our CMS.
- **SHOULD NOT trigger** — Redesign a terminal UI.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
