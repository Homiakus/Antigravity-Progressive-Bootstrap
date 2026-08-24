# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Redesign this Bubble Tea app with better keyboard navigation, status bar, responsive panes, and logs.
- **SHOULD trigger** — Audit our Go TUI state model, keymaps, async jobs, resize behavior, and SSH performance.
- **SHOULD trigger** — Make this Charm TUI look modern without requiring Nerd Fonts.
- **SHOULD NOT trigger** — Redesign a React dashboard.
- **SHOULD NOT trigger** — Make an Obsidian plugin sidebar.
- **SHOULD NOT trigger** — Audit REST API schema duplication only.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
