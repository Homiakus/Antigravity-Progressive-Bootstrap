# Skill eval cases

Use these cases to test activation precision and output quality after editing the skill.

## Activation cases

- **SHOULD trigger** — Find multiple sources of truth, magic numbers, hidden defaults, overengineering, allocations, and algorithmic complexity across this repo.
- **SHOULD trigger** — Audit duplicated config precedence, retry multiplication, cache ownership, races, and give me a file-level fix plan.
- **SHOULD trigger** — I need a deep system audit, not a normal code review—prove the findings and benchmark hot paths.
- **SHOULD NOT trigger** — Review this 20-line function for naming and style.
- **SHOULD NOT trigger** — Design a responsive website.
- **SHOULD NOT trigger** — Write a Markdown editor UX specification.

## Output-quality checks

- Does the response follow the ordered workflow rather than jump straight to cosmetic/local fixes?
- Does it distinguish evidence and assumptions from recommendations?
- Does it load detailed references only when needed?
- Does it produce concrete, testable output rather than generic advice?
- Does it preserve relevant constraints from `full-spec.md`?
- Does it avoid work owned by a neighboring skill unless composition is explicitly useful?
