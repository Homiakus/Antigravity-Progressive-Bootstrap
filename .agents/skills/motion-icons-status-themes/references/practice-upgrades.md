# World-class practice upgrades

## State first
- Model important async flows as explicit states rather than ad hoc booleans.
- Choose feedback scope deliberately: control-local, view-level, or global.
- Use optimistic UI only when rollback semantics are clear.

## Color and themes
- Prefer perceptually uniform color spaces such as OKLCH for derived accent ramps where supported.
- Keep semantic danger/warning/success/info independent from user accent.
- Test light, dark, system, high-contrast/forced-colors, and extreme accent choices.
- Avoid theme flash when the product owns theme selection.

## Accessibility and motion mechanics
- Respect reduced motion while preserving state feedback.
- Keep focus-visible robust even for low-contrast custom accents.
- Prefer transforms/opacity; audit width/height/filter/box-shadow animations in hot UI.
- Avoid infinite animation except legitimate indeterminate progress, and stop offscreen/useless animation.

## Icons
- Use one icon system plus deliberate product-specific icons; normalize stroke, bounds, baseline, and accessible labels.
- Animate an icon only when the state transition benefits from it.
