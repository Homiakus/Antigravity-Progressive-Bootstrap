# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Motion, Icons, Status System, Color & Themes

Act as a **Senior Product Designer, Motion Designer, Design Systems Architect, Interaction Designer, and Frontend UI Engineer**.

Deeply refine `[PROJECT NAME]` using a coherent system for animation, microinteractions, icons, status indicators, progress, feedback, color, accent customization, light/dark themes, design tokens, accessibility, and performance.

The objective is not “more animation” or “more color”. Build a calm, responsive system in which:

**State → Feedback → Motion → Color → Meaning.**

Every visual effect must communicate something.

## 1. Main principle

Use motion as an information channel. It should explain what changed, what is happening, what action was accepted, where focus is, what is active, what is loading, what completed, and what requires attention.

Before adding any effect ask: **What information does this motion communicate?** If there is no answer, omit it.

## 2. Motion audit

Audit the whole interface. Find:
- static areas lacking feedback;
- abrupt state switches;
- elements that appear/disappear without transition;
- buttons without tactile response;
- unclear active states;
- weak loading feedback;
- icons that benefit from contextual motion;
- hard-to-scan statuses;
- transitions that could make spatial relationships clearer.

Produce a map:

`Component → event → current state → proposed animation → duration → easing → purpose`.

Do not animate everything identically.

## 3. Motion design system

Centralize motion tokens, conceptually:

```css
--motion-instant: 80ms;
--motion-fast: 140ms;
--motion-base: 220ms;
--motion-medium: 320ms;
--motion-slow: 480ms;
--motion-scene: 700ms;
--ease-standard: cubic-bezier(0.2, 0, 0, 1);
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);
--ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);
--ease-spring: ...;
```

Adapt exact values to the product. Most UI should use 3–5 canonical durations, not random timings.

## 4. Animation hierarchy

Use four levels:

1. **Micro feedback** — hover, focus, press, toggle, checkbox, radio, icon response, selection, drag handle; typically 80–180ms.
2. **Component transitions** — dropdown, tooltip, popover, accordion, tabs, sidebar, modal, toast, context menu; typically 160–320ms.
3. **Layout transitions** — panel expansion, dashboard rearrangement, view-mode changes, resizing, insertion, filtering; typically 220–450ms.
4. **Scene transitions** — page changes, onboarding, fullscreen overlay, workspace transitions, intro; typically 400–800ms and used rarely.

## 5. Do not animate everything

Avoid simultaneous animation of every card, row, text fragment, icon, or dashboard element.

Priority: **state change > important action > navigation > decoration**.

## 6. Button microinteractions

Support relevant states:
`idle`, `hover`, `pressed`, `focus-visible`, `loading`, `success`, `error`, `disabled`.

On hover use restrained background/opacity/border/icon changes. If scaling, keep it small, around `1 → 1.015` or `1 → 0.98` on press. Avoid bouncy buttons.

## 7. Icon animation

Animate icons only when movement matches semantics:
- Refresh → one controlled rotation.
- Settings → slight rotation/position change when opening.
- Chevron → rotates with disclosure state.
- Play/Pause → morph.
- Menu/Close → morph.
- Upload → arrow rises during action.
- Download → inverse movement.
- Sync → rotates only while syncing.
- Check → stroke/reveal on completion.
- Warning → one short attention pulse.
- Bell → slight movement only on new event.
- Search → state transition when search mode opens.
- Favorite → fill/stroke transition.

## 8. Do not make icons toy-like

Avoid endless bouncing, constant spinning, elastic wobble, random motion, and cartoon behavior in professional interfaces.

## 9. One icon system

Audit all icons. Do not mix Lucide, Material, Heroicons, Font Awesome, and arbitrary SVGs without deliberate normalization.

Choose one base icon system with consistent stroke width, bounding box, optical size, corner treatment, and visual weight. Add custom product icons only when necessary.

## 10. Status system

Define a consistent vocabulary, at least as applicable:
`neutral`, `informational`, `active`, `success`, `warning`, `critical`, `disabled`, `pending`, `syncing`, `offline`, `unknown`.

The same status must mean the same thing everywhere. Avoid semantic drift such as green meaning “active” in one place and “completed” in another unless context makes it explicit.

## 11. Status indicators

Choose format by context:
- small dot for compact state;
- dot + label when color alone is insufficient;
- status badge for tables/lists, used sparingly;
- inline state such as `Connection · Online`;
- icon + text for important conditions.

Do not turn every status into a pill.

## 12. Status animation

Transitions should be smooth and meaningful:
- Connecting → spinner/state transition → Connected;
- Syncing → rotating sync icon → stop → check/normal state;
- Processing → progress indicator;
- Waiting → subtle pulse;
- Error → one attention animation.

Do not make critical states flash forever.

## 13. Status bar

For persistent system state, use a compact low-height status bar only when the information is genuinely useful. Possible content:
`Connected`, `Sync · OK`, `3 tasks`, `CPU 18%`, `Version 1.6.2`, `Last sync 12s ago`.

Keep it calm and easy to scan; do not compete with primary navigation.

## 14. Contextual status bar

Adapt the bar to current mode, for example:
- normal: `SYSTEM OK · 4 MODULES ONLINE`;
- edit: `EDIT MODE · 3 UNSAVED CHANGES`;
- syncing: `SYNCING · 62%`;
- issue: `1 DEVICE OFFLINE`.

## 15. Progress bars

Use determinate progress only when real measurable progress exists. Do not invent percentages.

For unknown progress use a spinner, indeterminate bar, skeleton, or textual stage.

## 16. Progress style

Prefer simple minimal progress: thin bar or compact embedded indicator. For meaningful long operations, show percentage, stage, processed items, or reliable process state.

## 17. Skeleton loading

Use skeletons when preserving content structure helps. Match the approximate shape of real content. Keep shimmer restrained. Respect reduced motion.

## 18. Toast/notification system

Define `success`, `information`, `warning`, `error`, `progress` categories. Keep toasts concise and avoid sending one for every trivial operation. If state change is already obvious, a toast may be unnecessary.

## 19. Action feedback

After an action, give explicit state feedback. Example:
`Save → spinner → check → Saved → normal state` or `Connect → Connecting… → Connected`.

Never make the user guess whether an action occurred.

## 20. Color as information

Separate:
- neutral interface colors;
- accent color;
- semantic success/warning/danger/info colors.

Do not mix accent and semantic roles.

## 21. Semantic color tokens

Do not hardcode component colors. Build semantic tokens such as:
`--surface-primary`, `--surface-secondary`, `--surface-elevated`, `--text-primary`, `--text-secondary`, `--text-tertiary`, `--border-subtle`, `--border-default`, `--border-strong`, `--accent`, `--accent-hover`, `--accent-active`, `--accent-subtle`, `--accent-contrast`, `--success`, `--success-subtle`, `--warning`, `--warning-subtle`, `--danger`, `--danger-subtle`, `--info`, `--info-subtle`.

## 22. Carefully select the palette

Do not copy default Tailwind colors uncritically. Evaluate hue, saturation, lightness, perceptual contrast, neutral harmony, OLED/LCD behavior, dark mode, and accessibility. Prefer OKLCH or another perceptually uniform color space when supported.

The palette must remain pleasant during long sessions, not just screenshots.

## 23. Neutral colors

Avoid raw `#000000` and `#ffffff` everywhere unless justified. For light mode, consider warm/neutral off-whites; for dark mode, graphite/charcoal/near-black.

## 24. Light theme

Design light theme as a real system, not black text on white. Define primary, secondary, elevated, hover, selected surfaces, border hierarchy, and text hierarchy.

## 25. Dark theme

Dark mode is not a mechanical inversion. Prefer graphite/charcoal/deep neutral gray over pure black unless OLED-black mode is an explicit product goal. Maintain surface hierarchy.

## 26. Dark-mode contrast

Do not make all text pure white. Use primary, secondary, and muted levels.

## 27. Accent color

Allow an accent color when appropriate. It may influence selected items, active navigation, primary CTA, focus ring, progress, selected toggles, and controlled highlights. Most UI remains neutral.

## 28. Appearance settings

Provide `Settings → Appearance` with:
- Theme: `System`, `Light`, `Dark`;
- Accent color presets, e.g. a curated 6–10 choices among Neutral, Blue, Cyan, Teal, Green, Lime, Amber, Orange, Red, Rose, Violet.

## 29. Accent presets

An accent preset is not a single hex. Define or derive:
`accent`, `accent-hover`, `accent-active`, `accent-subtle`, `accent-muted`, `accent-border`, `accent-contrast`.

Light and dark values may differ.

## 30. Custom accent

For power users, optional custom color is acceptable. Normalize saturation, derive hover/active/subtle variants, choose readable foreground, and check contrast. Never spray a raw user hex across every state.

## 31. Live preview

Theme and accent changes in settings should apply immediately where safe. Persist them when settings close.

## 32. System theme

Support `prefers-color-scheme`. `System` should update without reload when the OS theme changes.

## 33. Theme transition

A short 150–250ms color transition is acceptable. Avoid cinematic fade-outs. Prevent flashes of the wrong theme at startup.

## 34. Focus color

Accent may drive focus ring, but focus must remain visible for every accent. Auto-correct contrast where needed. Keyboard focus must not rely on a tiny hue shift.

## 35. Semantic colors independent of accent

A red accent must not erase the distinction between selected and danger. Combine color with icon, text, shape, or label. Never communicate critical meaning using color alone.

## 36. Hover system

Create one hover philosophy:
- text: underline reveal, opacity, small shift;
- button: background/border/icon shift;
- row: surface highlight;
- image: controlled scale around `1 → 1.02`.

Do not invent unrelated hover behavior per component.

## 37. Selected state

Selected must be more obvious than hover. Combine accent line, background, check icon, border, or stronger typography. Do not rely only on color.

## 38. Toggles

Animate the thumb smoothly without excessive spring. Make `off`, `on`, `disabled`, and `focus` states obvious. Accent may color the on state.

## 39. Checkbox/radio

Use short 120–200ms check-stroke or radio-dot transitions if helpful.

## 40. Dropdown

Open with restrained opacity, small translateY, and optionally `0.98 → 1` scale. Keep selected item obvious.

## 41. Sidebar

If collapsing, animate width only when performance allows; otherwise prefer transform/opacity strategies. Labels and icons should transition coherently without layout chaos.

## 42. Modal

Use overlay fade plus small opacity/scale. Avoid modals flying in from far away unless spatial navigation truly calls for it.

## 43. Drawer

Move from the side it logically belongs to. Right settings panel comes from right; mobile bottom sheet from bottom. Motion should reinforce spatial model.

## 44. Tabs

Tabs should feel immediate. Use sliding indicator, fade, or small directional translation around 120–220ms. Avoid heavy animation.

## 45. Tables

Do not constantly animate rows. Animate only meaningful insertion, deletion, status change, sort, or filter transitions. A brief background highlight can reveal a changed value.

## 46. Real-time data

Do not flash every update. Use short crossfade, subtle digit change, or transient highlight. Routine changes should not demand attention.

## 47. Numeric counters

Large KPI changes may transition, but do not count from zero every time a page opens. In real-time mode, change only the value that changed.

## 48. Error motion

Use a single restrained feedback action: small controlled input movement, error label, border change, warning icon. Avoid repeated shake.

## 49. Success motion

Use check stroke, fade, or icon morph. Do not use confetti for ordinary saving.

## 50. Attention motion

Use one or two subtle pulses. Avoid endless pulse, blinking, or bounce.

## 51. Loading icons

Spinner stroke should visually match the icon system. Choose one appropriate loading mechanism; do not stack spinner + skeleton + progress simultaneously.

## 52. Offline state

Provide explicit `Offline → Reconnecting… → Connected` behavior. If local work continues, tell the user.

## 53. Sync state

Use consistent states such as `Saved`, `Saving…`, `Syncing…`, `Synced`, `Offline`, `Conflict`, `Error`. Users should never wonder whether data is stored.

## 54. Accessibility and reduced motion

Honor `prefers-reduced-motion`. Remove parallax and decorative transforms; reduce complex transitions while retaining essential state feedback.

## 55. Reduced transparency

Honor `prefers-reduced-transparency` where supported if transparency/blur exists, but do not make glassmorphism foundational.

## 56. Contrast audit

Audit body, muted text, buttons, selection, borders, inputs, focus, status indicators for every light/dark + accent combination.

## 57. Color blindness

Do not rely only on red/green. Add icon, label, shape, or text, e.g. `✓ Connected`, `! Error`.

## 58. Performance

Prefer animating `transform` and `opacity`. Be cautious with `width`, `height`, `top`, `left`, `filter`, and `box-shadow` in frequent animations. Avoid layout thrashing.

## 59. GPU usage

Do not blanket the UI with `will-change`. Use compositor hints only where needed and avoid leaving unnecessary layers after animation.

## 60. Smoothness

Test primary microinteractions on desktop, weak laptops, mobile, touch, and reduced-motion mode. If decorative motion drops frames, simplify or remove it.

## 61. Appearance settings structure

A professional appearance page may include:
- Theme: System / Light / Dark;
- Accent swatches;
- optional density: Comfortable / Compact;
- optional Motion: System / Full / Reduced.

## 62. Visual swatches

Keep swatches compact, named, keyboard accessible, and visibly selected with a check or equivalent. Do not present an uncontrolled rainbow.

## 63. Theme preview

Optional small previews may help Light/Dark/System choice, but clarity matters more than decoration.

## 64. Preference persistence

Persist theme, accent, and motion choices across reload/restart/login as appropriate.

## 65. No startup theme flash

Resolve the correct preference before rendering enough UI to flash the wrong theme.

## 66. Token architecture

Use the hierarchy:

`Primitive colors → Theme colors → Semantic tokens → Component tokens → Components`.

Components should not know raw palette values.

## 67. Shared light/dark architecture

Do not create two unrelated CSS systems. Themes should redefine semantic variables while components consume those variables.

## 68. Color science

Use OKLCH or comparable perceptual methods when practical, especially for accent variants and equal perceived brightness. Do not derive hover by arbitrary `darken(10%)` if a systematic method is available.

## 69. Accent restraint

Accent should occupy roughly 5–15% of the visual interface. Use it for active, primary action, focus, progress, selection, and key status highlight.

## 70. Test without accent

Temporarily neutralize accent. If the hierarchy collapses, the design depends too much on color. Typography, spacing, weight, layout, borders, and scale must still work.

## 71. Test without motion

Disable animation. The interface must remain completely understandable.

## 72. Icon audit

For every icon ask:
- Is it needed?
- Could text be clearer?
- Is it understandable without tooltip?
- Does its weight match the system?
- Does it need animation?

Remove decorative icon clutter.

## 73. Motion audit after implementation

For every animation record `Trigger`, `Purpose`, `Duration`, `Easing`, `Reduced-motion behavior`. If purpose cannot be stated, remove it.

## 74. Color audit

Search for hardcoded hex/rgb/hsl values. Move accidental component colors into the design system.

## 75. UI-state audit

For each interactive component, verify applicable `default`, `hover`, `focus`, `active`, `selected`, `disabled`, `loading`, `error` states. Different logical states must not look identical.

## 76. Responsive motion

On mobile, reduce travel distances, duration, and parallax. Touch has no hover; critical interactions must not depend on hover.

## 77. Haptic-like visual feedback

On touch, use short scale/surface/icon reactions. Native haptics may be used only for meaningful actions where platform support exists. Do not vibrate for every tap.

## 78. Empty states

Optional subtle animated illustration/icon is fine, but the primary job is to explain why the area is empty and what the user should do next.

## 79. Onboarding

Onboarding may use more motion, but avoid long intros. Let users start quickly. Motion should teach rather than delay.

## 80. Brand character

After building the functional motion system, add one or two signature behaviors such as a distinctive arrow move, reveal, progress treatment, logo animation, or accent-line interaction. Do not create ten unrelated signature effects.

## 81. Implementation order

1. Audit colors, icons, states, transitions, status UI, themes.
2. Build design tokens.
3. Build theme architecture.
4. Build accent system.
5. Unify iconography.
6. Implement component states.
7. Add motion primitives.
8. Add contextual microinteractions.
9. Add status/progress feedback.
10. Run accessibility audit.
11. Run performance audit.
12. Final polish.

Do not start with decorative animation.

## 82. Final polish pass

Walk through opening/closing, loading, navigation, view switching, selection, forms, saving, errors, success, long operations, network state, sync, empty, and disabled states. Remove abrupt jumps and reduce any excessive motion.

## 83. Quality criteria

The system is ready only when:
- Light theme is coherent on its own.
- Dark theme is coherent on its own.
- System follows OS preferences correctly.
- Accent can change without breaking semantic colors.
- Presets work in both themes.
- Motion has a unified system and no random timings.
- Icons are stylistically unified and only contextually animated.
- Loading/sync/error/success states are understandable.
- Status is not conveyed by color alone.
- UI is understandable without color and without animation.
- Reduced motion is supported.
- Focus navigation is visible.
- Microinteractions cause no layout shifts.
- UI stays smooth on weaker hardware.
- There are no irritating infinite animations.
- Color remains pleasant during long sessions.

## 84. Final priority rule

Resolve conflicts in this order:

**Clarity → Feedback → Accessibility → Consistency → Performance → Motion → Decoration**.

Color explains meaning. Motion explains change. Icons explain action. Status explains state. Typography creates hierarchy. Whitespace preserves calm. All layers must behave as one system.
