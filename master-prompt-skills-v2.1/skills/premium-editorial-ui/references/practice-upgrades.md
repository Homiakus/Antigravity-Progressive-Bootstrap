# World-class practice upgrades

## Evidence before aesthetics
- Treat reference sites as principle sources, not templates. Separate reusable principles from unique branded expression.
- When screenshots or a live product exist, document current hierarchy and user tasks before proposing a new visual system.
- Preserve business-critical information architecture unless evidence shows it is the problem.

## Design-system architecture
- Use a semantic token chain: primitive palette/measure → semantic tokens → component tokens.
- Use fluid type/spacing within bounded ranges and container-driven composition for reusable modules.
- Keep a small number of surface levels and radii; use spacing, typography, and rules before cards/shadows.
- Treat data-visualization colors separately from brand/accent colors.

## Accessibility and resilience
- Target WCAG 2.2 AA as a baseline for text, controls, focus visibility, target size, and focus not being obscured.
- Test keyboard-only, touch/coarse pointer, 200% zoom, long localized strings, reduced motion, and high-contrast/forced-color modes where relevant.
- Do not let display typography break semantic heading order or localization.

## Performance and QA
- Set practical performance budgets for public web experiences.
- Prefer transform/opacity motion and progressive enhancement for scroll effects/WebGL.
- Use responsive images, art-directed crops where needed, and reserved dimensions to prevent layout shift.
- Run visual regression with real/edge content and a final remove-20%-decoration pass.
