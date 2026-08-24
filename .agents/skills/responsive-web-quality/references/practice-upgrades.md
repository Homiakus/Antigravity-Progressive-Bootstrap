# World-class practice upgrades

## Modern CSS architecture
- Prefer intrinsic sizing, logical properties, `minmax(0, 1fr)`, `min-width: 0`, `clamp()`, container queries, and subgrid where valuable before viewport patches.
- Use `svh`/`dvh` deliberately and account for safe-area insets on fixed edge controls.
- Use hover/pointer capabilities instead of inferring input from width.
- Keep DOM reading/focus order logical; visual reordering must not break accessibility.

## Accessibility quality bar
- Use WCAG 2.2 AA as a baseline; test focus not obscured by sticky UI, touch targets, reflow/zoom, and text spacing.
- Test 200% zoom and relevant 400% reflow scenarios; do not suppress mobile zoom.
- Verify long translations and increased text size against fixed-height assumptions.

## Complex components and performance
- Give tables/charts/editors explicit compact strategies rather than generic overflow.
- Preserve key columns and expose hidden detail intentionally.
- Use accurate responsive image `sizes`; prevent CLS through reserved dimensions/font metrics.
- Progressive-enhance heavy scroll effects instead of making them core.

## Automated QA
- Add page-level overflow assertions, representative visual snapshots, and E2E critical flows.
- Test continuous widths, split-screen, landscape phones, small-height laptops, and ultrawide.
