# Skill eval cases

## SHOULD trigger
- "After every frontend change, have Playwright render the page and let the agent inspect whether the layout really broke."
- "Add DOM geometry checks, screenshot pixel diffs, and a local VLM fallback to our coding harness."
- "Create a fast visual feedback loop for a DeepSeek coding agent without sending every screenshot to the cloud."
- "Map Playwright screenshot differences back to the DOM so the agent can fix the responsible component."

## SHOULD NOT trigger
- "Design a premium landing page from scratch" — use the design skill unless a runnable implementation also needs verification.
- "Audit Go allocations" — use the codebase audit skill.
- "Redesign a terminal UI" — use the TUI skill.

## Quality evals
- A deterministic overlap fixture must be detected without VLM.
- Intentional parent/child overlap must not be reported as a collision.
- Dynamic timestamps should be maskable without loosening the whole pixel threshold.
- A failed baseline must never be auto-updated.
- VLM output must name stable refs/regions and be labeled semantic/uncertain when not corroborated.
- The auto policy should skip VLM for deterministic failures and unchanged states.
