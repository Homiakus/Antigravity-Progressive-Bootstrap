# Master Prompt Skills — v2.1

Production-grade Agent Skills derived from every master prompt in the conversation.

## v2 architecture

Each skill now uses progressive disclosure:

```text
skill/
├── SKILL.md                       # concise activation + core workflow
├── references/
│   ├── full-spec.md               # complete original prompt, preserved
│   ├── practice-upgrades.md       # best-practice additions
│   ├── output-contract.md         # required deliverable structure
│   ├── evals.md                   # trigger/non-trigger + quality evals
│   └── sources.md                 # authoritative current references
└── scripts/
    └── validate_skill.py          # dependency-free structural validator
```

The main `SKILL.md` files are intentionally compact. No original master-prompt rules were deleted: the entire prior body is retained under `references/full-spec.md`.

## Included skills

### Visual runtime layer

- `web-visual-runtime-validation` — progressive real-browser validation using Playwright, DOM/ARIA geometry, visual diffing, and selective local VLM inspection.

### Core skills

- `premium-editorial-ui`
- `motion-icons-status-themes`
- `cms-editorial-workflow-ux`
- `responsive-web-quality`
- `go-tui-premium-ui`
- `obsidian-plugin-premium-ui-suite`
- `markdown-live-preview-editor`
- `deep-codebase-integrity-audit`

## Improvements

- More precise user-intent descriptions and negative activation boundaries.
- Progressive disclosure to reduce context cost.
- Explicit required inputs, ordered workflows, evidence rules, and completion gates.
- Dedicated output contracts so results are concrete and testable.
- Trigger/non-trigger eval cases for every skill.
- Original full specifications preserved for deep tasks.
- Domain-specific best-practice upgrades.
- Portable validators for Agent Skills naming/description/reference structure.
- No experimental `allowed-tools` restrictions, preserving cross-client compatibility.

## Validation

```bash
python tools/validate_all.py
```

If `skills-ref` is installed, also run:

```bash
skills-ref validate skills/<skill-name>
```

## Reference standards

- https://agentskills.io/specification
- https://agentskills.io/skill-creation/best-practices
- https://agentskills.io/skill-creation/optimizing-descriptions
- https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills
- https://openai.com/academy/skills/

## v2.1 visual-agent loop

Web-oriented skills can now compose with `web-visual-runtime-validation`:

```text
code → Playwright render → L0 runtime → L1 DOM/ARIA geometry → L2 pixel baseline → L3 local VLM when needed → evidence packet → targeted fix → re-test
```

The runtime layer is intentionally progressive: cheap deterministic checks run first, while VLM inference is reserved for visual semantics or ambiguous cases.
