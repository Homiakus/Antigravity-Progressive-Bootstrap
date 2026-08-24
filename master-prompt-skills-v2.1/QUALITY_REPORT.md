# Quality report — v2.1.0

## Automated checks

- 8/8 skills pass the bundled structural validator.
- 8/8 `SKILL.md` frontmatter blocks parse as valid YAML.
- 8/8 skill names match their parent directory and Agent Skills naming constraints.
- 8/8 descriptions are below the 1024-character specification limit.
- 8/8 compatibility fields are below the 500-character specification limit.
- Every active `SKILL.md` is far below the recommended 500-line threshold.
- Every reference named by `SKILL.md` exists.
- Every v1 master-prompt body was compared with `references/full-spec.md` and preserved byte-for-byte after the preservation header.

## Skill sizes

- `cms-editorial-workflow-ux` — SKILL.md 83 lines; full spec 642 lines; 5 reference files.
- `deep-codebase-integrity-audit` — SKILL.md 84 lines; full spec 936 lines; 5 reference files.
- `go-tui-premium-ui` — SKILL.md 84 lines; full spec 760 lines; 5 reference files.
- `markdown-live-preview-editor` — SKILL.md 84 lines; full spec 823 lines; 5 reference files.
- `motion-icons-status-themes` — SKILL.md 84 lines; full spec 504 lines; 5 reference files.
- `obsidian-plugin-premium-ui-suite` — SKILL.md 84 lines; full spec 809 lines; 5 reference files.
- `premium-editorial-ui` — SKILL.md 86 lines; full spec 505 lines; 5 reference files.
- `responsive-web-quality` — SKILL.md 84 lines; full spec 682 lines; 5 reference files.

## External validator

The Agent Skills specification recommends `skills-ref validate`. The execution environment had no DNS access to PyPI, so `uvx skills-ref` could not be downloaded. This package therefore does **not** claim that external validator was run. The bundled validator plus PyYAML parsing cover the relevant structural constraints; run `skills-ref validate skills/<name>` in a networked environment for an additional independent check.

## Preservation guarantee

The original v1 prompt bodies are retained under each skill as `references/full-spec.md`. The v2 redesign changes how instructions are loaded, not the underlying detailed requirements.

## Methodology sources

- https://agentskills.io/specification
- https://agentskills.io/skill-creation/best-practices
- https://agentskills.io/skill-creation/optimizing-descriptions
- https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills
- https://openai.com/academy/skills/

## Visual runtime validation additions

- Real-render verification is now a reusable skill rather than duplicated prose.
- Deterministic checks precede VLM calls.
- Screenshot regression uses Playwright native assertions by default; direct pixelmatch is reserved for custom localization.
- Local VLM output is structured, grounded with element refs/crops, and never treated as sole proof.
- Dynamic baseline noise, environment pinning, privacy, and failed-baseline governance are explicit.
