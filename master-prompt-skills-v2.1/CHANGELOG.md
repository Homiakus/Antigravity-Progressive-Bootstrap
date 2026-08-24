# Changelog

## 2.1.0 — Visual Runtime Validation

- Added `web-visual-runtime-validation` as a ninth skill.
- Added progressive Playwright runtime validation: L0 runtime, L1 DOM/ARIA geometry, L2 screenshot/pixel regression, L3 local VLM, optional L4 escalation.
- Added model-neutral local VLM routing with MiniCPM-V 4.6-class edge inference as a current candidate.
- Added visual evidence packet schema and Cordis/deepseek-harness tool contract.
- Added baseline governance, screenshot determinism, diff-region → DOM mapping, viewport escalation, false-positive controls, and privacy rules.
- Composed the visual layer into premium editorial, motion/themes, CMS, responsive, and Markdown-editor skills.
- Preserved all existing full specifications.


## 2.0.0 — 2026-08-23

- Re-architected all eight skills for Agent Skills progressive disclosure.
- Rewrote activation descriptions around user intent and added negative activation boundaries.
- Reduced active `SKILL.md` files to ~80 lines each while preserving every v1 body verbatim in `references/full-spec.md`.
- Added explicit inputs, ordered workflows, evidence rules, completion gates, and cross-skill routing.
- Added domain-specific `practice-upgrades.md` using current world-class engineering/design practices.
- Added `output-contract.md` to make substantial deliverables concrete and testable.
- Added should-trigger / should-not-trigger eval cases and output-quality gates.
- Added authoritative `sources.md` for version-sensitive standards, security, host APIs, accessibility, and tooling.
- Added dependency-free validation scripts for each standalone skill and the complete bundle.
- Preserved cross-client portability by avoiding experimental tool restrictions.
