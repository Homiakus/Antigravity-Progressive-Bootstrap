# Visual Runtime Validation — v2.1

The package now contains a cross-cutting skill named `web-visual-runtime-validation`.

## Fast loop

```text
frontend patch
  ↓
Playwright real render
  ↓
L0 runtime sanity
  ↓
L1 ARIA + DOM geometry + computed-style invariants
  ↓
L2 Playwright/pixelmatch visual regression when a trusted baseline exists
  ↓
L3 selective local VLM semantic inspection when needed
  ↓
VisualEvidencePacket with DOM refs, diff regions, confidence and artifact paths
  ↓
main coding agent applies targeted fix
  ↓
re-run affected state → final representative regression matrix
```

## Why this is progressive

The system avoids using a vision model when deterministic browser evidence can already prove the defect. VLM inference is reserved for visual semantics such as hierarchy, spacing rhythm, crowding, visual balance, or ambiguous screenshot differences.

## Current local VLM routing

The architecture is provider-neutral. A MiniCPM-V 4.6-class edge model is documented as a current fast local candidate; stronger local VLMs can be configured as escalation tiers.

## Integrated skills

- premium-editorial-ui
- motion-icons-status-themes
- cms-editorial-workflow-ux
- responsive-web-quality
- markdown-live-preview-editor

All previous full master specifications remain preserved.
