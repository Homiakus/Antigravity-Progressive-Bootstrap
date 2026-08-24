# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Adaptive & Responsive Website Quality

Act as a **Senior Responsive Web Designer, Frontend Architect, UX Engineer, Accessibility Specialist, Mobile UI Designer, and cross-browser compatibility engineer**.

Deeply audit and improve `[PROJECT NAME]` so it looks intentional and works reliably across a very wide range of screens, orientations, input methods, pixel densities, and viewport constraints.

Do not merely add a few standard media queries. Build a coherent responsive system covering layout, typography, spacing, media, navigation, forms, tables, cards, modals, menus, CMS, dashboards, touch interaction, states, animation, information density, and complex data-heavy screens.

Main goal:

> On every reasonable device class, the UI should look as if it were designed for that environment rather than merely shrunk from desktop.

## 1. Main principle

Do not use `desktop → shrink everything → hide pieces` as the core strategy. Use **content-driven responsive design** based on available width/height, content structure, input capabilities, pixel density, orientation, viewport, and accessibility preferences.

Breakpoints exist where layouts fail, not because `768px` is conventional.

## 2. Responsive audit

Audit all major pages/components: home, navigation, header/footer, hero, content/article, forms, search, cards, galleries, tables, dashboard, CMS, settings, modal, drawer, dropdown, tooltip, notifications, empty/error states, auth, media viewer.

Inspect continuously from about `320px` through `3840px`, not only a few snapshots.

## 3. Responsive issue map

For each issue record:

`Page/component → width → problem → cause → solution → verification`.

Do not add a breakpoint if intrinsic/fluid layout can solve the issue.

## 4. Avoid device-only design

Do not build solely around iPhone/iPad/Laptop labels. Use space-based classes such as extra-compact, compact, medium, wide, large, extra-large only as conceptual aids; choose real boundaries from content failure points.

## 5. Mobile-first where useful

For new or substantially rewritten components, prefer a stable mobile-first base and progressively enhance as space grows. Do not rewrite a working architecture merely to claim mobile-first compliance.

## 6. Fluid design

Prefer `%`, `fr`, `minmax()`, `auto-fit`, `auto-fill`, `clamp()`, `min()`, `max()`, and intrinsic sizing over large sets of hardcoded dimensions.

## 7. CSS Grid

Use Grid for complex layouts. `repeat(auto-fit, minmax(...))` patterns are valid only when minimum widths reflect real content needs.

## 8. Flexbox

Use Flexbox for nav/toolbars/button groups/inline controls. Allow wrapping where logical; audit unnecessary `flex-shrink: 0`.

## 9. Container queries

Prefer container queries for reusable cards, dashboard widgets, CMS panels, sidebars, and components that appear in multiple widths. Components should adapt to their own container, not only global viewport.

## 10. Fluid typography

Use `clamp()` with defined minimum, fluid preferred value, and maximum instead of dozens of font-size media queries.

## 11. Mobile typography

Do not automatically reduce editorial display type to generic 24px. Preserve character while verifying wrapping, Cyrillic, long German words, line height, overflow, and hyphenation.

## 12. Long words/identifiers

Audit URLs, emails, filenames, technical IDs, product names, and long translations. Use `overflow-wrap`, `word-break`, or `hyphens` contextually rather than globally mangling words.

## 13. Text width

Keep long reading text to an appropriate measure, commonly around `45–75ch`. Do not let articles span 200 characters per line on ultrawide screens.

## 14. Responsive spacing

Make spacing fluid where useful, e.g. `padding-inline: clamp(...)`; avoid fixed desktop-sized padding everywhere.

## 15. Gutter system

Centralize page gutters. A conceptual progression might be 16px compact, 24–32px medium, 40–64px wide, 64–96px large; tune to the actual design.

## 16. Max widths

Define max content, reading, dashboard, and form widths. Allow full-bleed media/graphs/special sections when justified.

## 17. Ultrawide

Test 2560/3440/3840 widths. Do not leave a tiny desktop site floating in huge emptiness. Expand grid, gutters, media, or secondary panels while preserving readable text widths.

## 18. Small screens

Test at least 320px. Find hardcoded width, overflow, button/label problems, dialog width failures, and form issues.

## 19. Height responsiveness

Audit limited viewport height, including laptops with browser chrome/devtools and phones in landscape. Pay special attention to modal, menu, drawer, hero, onboarding, dialog.

## 20. Modern viewport units

Use `svh`, `dvh`, `lvh` where appropriate. Do not rely only on `100vh` on mobile.

## 21. Hero

For fullscreen hero, prefer `min-height: 100svh` or an equivalent robust strategy. Account for address bar, landscape, browser UI, safe areas, and ensure CTA is not clipped.

## 22. Safe areas

Support notches, Dynamic Island, rounded corners, and home indicator using `env(safe-area-inset-*)` where fixed/fullscreen UI needs it.

## 23. Orientation

Test portrait and landscape. Phone landscape has large width but little height; use compact-height behavior when needed.

## 24. Navigation

Let full navigation collapse when content no longer fits, not at an arbitrary device width. Use full → compact → menu/drawer progression as needed.

## 25. Mobile menu

Make mobile navigation fast, obvious, accessible, scrollable when long, and usable one-handed. Avoid animation that slows access.

## 26. Header

On small screens prioritize brand, navigation trigger, and genuinely essential CTA. Remove secondary desktop controls rather than crushing them.

## 27. Sticky header

Keep sticky header short. It may compact on scroll without causing layout shift. Test virtual keyboard and safe areas.

## 28. Footer

Adapt multi-column footer into 2-column/accordion/sequential groups as appropriate. Avoid extremely long mobile footers.

## 29. Cards

Cards should change internal layout based on space, e.g. image/content horizontal when wide and vertical when compact.

## 30. Card grid

Choose column count from a meaningful minimum card width rather than hardcoded 4→3→2→1 breakpoints.

## 31. Dashboard

Do not merely stack everything on mobile. Reorder by information priority and preserve critical KPI/actions first.

## 32. Responsive priority

Assign P0 critical, P1 primary, P2 secondary, P3 optional to major dashboard elements. Collapse or relocate P3 on small screens; never hide critical information only for visual neatness.

## 33. Sidebar

Desktop sidebar may become icon rail, drawer, bottom nav, or modal menu depending on product. Avoid keeping a 280px sidebar on a narrow tablet without reason.

## 34. Tables

Choose a strategy per table:
- horizontal scroll;
- column priority;
- row details;
- card/list transformation;
- frozen key column.

Do not default every table to `overflow-x:auto` without analysis.

## 35. Horizontal scroll

When required, make it discoverable, preserve key identifiers, consider sticky first column, and avoid breaking main vertical scroll.

## 36. Forms

Collapse multi-column forms when labels/inputs become cramped, not merely when a generic tablet breakpoint is crossed.

## 37. Input size

On touch devices, use roughly 44–48px control height/hit area where appropriate. Do not keep 28px desktop-density inputs on phones.

## 38. Mobile keyboard

Test forms with the virtual keyboard open. Keep the active field, validation, and action buttons visible; fixed bottom UI must not cover input.

## 39. Input types

Use semantic HTML input types (`email`, `tel`, `number`, `date`, `search`) to improve mobile keyboards.

## 40. Autocomplete

Use browser autocomplete correctly for login, contacts, address, payment where applicable.

## 41. Dropdowns

Long desktop dropdowns may become bottom sheets/fullscreen pickers/search dialogs on mobile. Choose based on option count and task.

## 42. Modals

Modals must fit viewport via responsive width, `max-height`, and internal scrolling. Large mobile modals often work better fullscreen or as bottom sheets.

## 43. Bottom sheets

Use for mobile filters, selection, contextual actions, secondary settings when appropriate. Do not convert every dialog automatically.

## 44. Tooltips

Never make critical information hover-only. Touch users need tap/focus/visible labels/help alternatives.

## 45. Hover capability

Gate hover-specific behavior with capability queries such as `@media (hover:hover)`. Large screens may still be touch-only.

## 46. Pointer capability

Use `@media (pointer: coarse)` when control sizes/drag handles need input-specific adaptation. Do not infer pointer type solely from width.

## 47. Touch targets

Important targets should have about 44×44px hit areas. The visible icon can remain 18–24px.

## 48. Action spacing

Keep destructive and primary actions sufficiently separated on touch devices.

## 49. Bottom navigation

Use app-like bottom nav only when there are about 3–5 stable primary sections and it suits the product. Do not force websites into mobile-app patterns.

## 50. One-handed use

Place frequent mobile actions in accessible regions while respecting safe areas.

## 51. Images

Use responsive sizing (`max-width:100%; height:auto` where appropriate), but also design aspect ratio, cropping, focal point, and source sizes.

## 52. `srcset`/responsive images

Use `srcset`, `sizes`, or framework image components. Do not send 3000px assets to a 390px phone without reason.

## 53. Art direction

Use `<picture>` or alternate crops where desktop media composition fails vertically on mobile.

## 54. Aspect ratio

Use `aspect-ratio` to prevent layout shift; avoid arbitrary fixed media heights.

## 55. Video

Preserve ratio, viewport fit, mobile controls, and loading performance. Disable/simplify heavy background video on weak devices when needed.

## 56. Embeds

Audit iframe/maps/YouTube/external widgets; wrap responsively and prevent page-level horizontal overflow.

## 57. Icons

Keep icon visual size stable; adapt hit area separately.

## 58. Buttons

Let buttons adapt to labels; avoid unnecessary fixed widths. Test long translations.

## 59. Button groups

Horizontal desktop groups may wrap, stack, or become sticky action bars on compact screens while preserving primary/destructive hierarchy.

## 60. CTA

Primary CTA may become full-width on mobile when it improves usability, but do not make every button full-width.

## 61. Breadcrumbs

On mobile, shorten to parent/current, allow controlled horizontal scroll, or use back navigation. Avoid seven-level wrapped breadcrumbs.

## 62. Tabs

When tabs do not fit, use horizontal scroll, dropdown, subset, or secondary navigation. Do not shrink labels to unreadability.

## 63. Filters

Desktop sidebar filters often become `Filters` button → drawer/sheet. Show active filter count such as `Filters · 3`.

## 64. Search

Desktop search may remain inline; mobile may open a dedicated search mode. Preserve quick access.

## 65. Toolbars

Adapt by priority. Keep primary actions visible; move secondary actions into More, but do not hide the entire toolbar behind ellipsis.

## 66. CMS

On mobile, at minimum allow finding content, opening, editing text, uploading media, changing status, preview, publish. Advanced settings can be secondary.

## 67. Rich-text/editor behavior

Editors must support small screens, touch selection, virtual keyboard, image/link insertion. Toolbars may scroll, collapse, or become contextual.

## 68. Code blocks

Keep horizontal scroll inside code/pre containers rather than forcing the whole page to overflow.

## 69. `<pre>`

Audit separately because it is a common source of overflow.

## 70. Charts

Responsive charts should reduce tick labels/annotations, reposition legend, or use controlled horizontal scroll rather than simply shrinking into illegibility.

## 71. Chart tooltip

Keep tooltip within viewport and support tap on touch.

## 72. Maps

Ensure map controls remain reachable on touch and away from unsafe browser edges.

## 73. Animations

On mobile, reduce translation distances, parallax, heavy blur, and simultaneous animated elements while preserving the motion language.

## 74. Reduced motion

Honor `prefers-reduced-motion`. Responsive design includes user capabilities, not only geometry.

## 75. High DPI

Provide crisp raster sizes or SVG on retina/high-DPI displays.

## 76. Zoom

Test 200% browser zoom. Do not disable user scaling without an exceptional reason.

## 77. System text scaling

Do not rely on fixed heights that break when system/browser text size increases.

## 78. Accessibility

Maintain keyboard, screen-reader, focus order, visible focus, semantic DOM order across responsive modes.

## 79. DOM order

Prefer one logical DOM order for mobile/desktop. Avoid CSS `order` arrangements that make keyboard/screen-reader sequence incoherent.

## 80. Scroll

Avoid unnecessary nested scroll regions, especially on mobile. One main vertical scrolling surface is often best.

## 81. Horizontal overflow audit

Find page-level overflow from fixed width, transforms, negative margins, SVG, tables, pre/code, iframes, images, long strings, absolute elements. Treat unexpected global horizontal scrolling as a bug.

## 82. Fixed positioning

Test fixed UI against virtual keyboard, safe areas, browser chrome, and orientation.

## 83. Sticky elements

Do not let multiple sticky elements consume half a small viewport.

## 84. Foldables

Avoid assuming mobile is always one narrow column. Use flexible containers; support viewport segments only if product needs justify it.

## 85. Tablet

Treat tablet as its own environment. Test iPad/Android portrait and landscape; a tablet may support a sidebar with altered density.

## 86. Small laptops

Explicitly test `1366×768` and similar sizes; desktop should not be designed only for 1920×1080.

## 87. Browser chrome

Test bookmarks bars, devtools, browser sidebars, and zoom. UI should tolerate modest viewport reduction.

## 88. Cross-browser

Test current Chrome, Edge, Safari, Firefox; on mobile Safari iOS and Chrome Android. Use fallbacks where support matters.

## 89. Feature queries

Use `@supports` for progressive enhancement where appropriate.

## 90. JavaScript breakpoints

Prefer CSS for layout. If JS must know size, centralize it; avoid scattered `window.innerWidth < 768` checks.

## 91. Resize performance

Do not perform expensive work on every resize event. Use ResizeObserver, debounce, or CSS container queries.

## 92. ResizeObserver

Use for components dependent on their own size, but avoid unnecessary observer proliferation.

## 93. Responsive design tokens

Centralize container widths, page gutters, section spacing, font scales, control sizes, nav heights, panel widths.

## 94. Breakpoint tokens

Centralize global breakpoints when useful, while allowing container-driven components to use their own thresholds.

## 95. Avoid breakpoint hell

If one component needs many tiny adjacent breakpoints, repair layout primitives instead of accumulating patches.

## 96. Do not hide layout bugs

Do not solve broken overflow with `overflow:hidden` unless clipping is intentional. Fix the cause.

## 97. Absolute positioning

Avoid absolute positioning for primary document structure. Reserve for overlays, badges, decorations, and controlled compositions.

## 98. `min-width:0`

Audit flex/grid children for missing `min-width:0`, a frequent overflow cause.

## 99. `minmax(0,1fr)`

Use where long content otherwise stretches Grid tracks.

## 100. Responsive QA matrix

Test representative viewport points such as:

```text
320×568
360×800
375×812
390×844
412×915
430×932
568×320
844×390
768×1024
820×1180
1024×768
1280×720
1366×768
1440×900
1536×864
1920×1080
2560×1440
3440×1440
3840×2160
```

These are QA checkpoints, not media-query targets.

## 101. Test between checkpoints

Continuously resize and look for failure points such as 713px or 1087px. The layout must work throughout the range.

## 102. Visual regression

Where infrastructure exists, add screenshot-based regression tests for key pages across representative widths with a sensible tolerance.

## 103. E2E responsive flows

Test real workflows, e.g. mobile `menu → page → form → submit` and desktop `navigation → editor → preview → publish`. Screenshots alone are insufficient.

## 104. Overflow automation

Automate detection of unexpected page-level horizontal overflow, e.g. compare `scrollWidth` vs `clientWidth`.

## 105. Console health

Ensure no resize/render/hydration/image-sizing/layout-script errors appear across primary viewport classes.

## 106. Performance

Account for weaker mobile hardware. Avoid loading the same heavy hero animation/assets when unnecessary. Use responsive images, lazy loading, code splitting, reduced motion, and `content-visibility` where appropriate.

## 107. Core Web Vitals

Responsive changes must not regress LCP, CLS, INP. Watch font loading, media, dynamic nav, banners, embeds.

## 108. Font loading

Choose fallbacks/metrics to reduce layout shifts caused by web fonts.

## 109. Mobile performance

Audit CPU/GPU as well as network. Limit blur, giant shadows, canvas/WebGL, huge DOM, complex scroll animation.

## 110. Real-content testing

Test long/short titles, long descriptions, 0/1/1000 items, missing images, portrait/landscape media. Do not design only around ideal demo content.

## 111. Localization

Test languages with longer text such as Russian/German. Avoid button widths based only on English.

## 112. RTL readiness

If RTL may matter, prefer logical properties (`inline-start/end`) over hardcoded left/right. Do not overbuild RTL support if out of scope, but avoid making it impossible.

## 113. Scrollbars

Do not hide system scrollbars without reason. Keep custom scroll areas usable.

## 114. Print

If the product includes articles, invoices, reports, or printable docs, test A4/print styles. Skip if irrelevant.

## 115. PWA

If installable, test standalone mode, safe areas, and fixed navigation because viewport differs from browser mode.

## 116. Responsive content

Adapt information, not only layout. Use progressive disclosure for secondary detail on compact screens without deleting important functionality.

## 117. Mobile is not less functional

Keep important features accessible through different presentation, e.g. desktop toolbar `Save | Preview | Duplicate | Export | Archive` vs mobile `Save | Preview | More`.

## 118. Responsive animation

Scale motion to physical screen size. Desktop 60px translations may become 10–24px on mobile.

## 119. Preserve whitespace

Compact screens still need rhythm; do not collapse all spacing to zero.

## 120. Information density

Adapt density by task. Marketing mobile may stay spacious; professional tables may remain dense. Do not impose one spacing philosophy everywhere.

## 121. Desktop quality

Responsive work must not degrade desktop. Do not optimize exclusively for mobile.

## 122. Large-screen enhancement

Use extra space for persistent secondary panels, larger preview, editor+preview, expanded navigation, extra metadata when it helps; do not make those enhancements required to complete tasks.

## 123. Device capabilities

Account for hover, pointer precision, reduced motion, color scheme, and contrast preferences. Responsive design includes capabilities, not only pixels.

## 124. Responsive component system

Document responsive logic for each primitive, e.g. Button, Card, Navigation, Table, Modal, Toolbar, Sidebar. Do not re-solve the same behavior independently on every page.

## 125. Document the rules

Create a responsive guideline listing minimum widths, layout modes, content priority, interaction differences, and container/breakpoint behavior for each major component.

## 126. Implementation order

1. Inventory layouts/components.
2. Audit failure points.
3. Repair Grid/Flex/container primitives.
4. Standardize responsive tokens.
5. Fix navigation.
6. Fix core pages.
7. Fix complex UI/tables/dashboard/CMS/editors.
8. Fix forms/touch/keyboard.
9. Fix media/charts/embeds.
10. Height/orientation pass.
11. Accessibility/zoom/text scale.
12. Cross-device/browser QA.
13. Mobile performance.
14. Visual polish.

## 127. Existing-codebase mode

Before changing styles, find global styles, design tokens, layout components, breakpoints, containers, typography, navigation, repeated media queries, hardcoded dimensions, viewport-dependent JS.

Produce:

`Problem → component → file → cause → change → verification`.

Fix foundational primitives before local hacks.

## 128. Automated anti-pattern search

Search for fixed width, `min-width`, `max-width`, `100vh`, `position:absolute`, `overflow:hidden`, `white-space:nowrap`, hardcoded pixels, `window.innerWidth`, device-specific conditions, duplicate breakpoints, huge z-index. Evaluate context; do not mark every occurrence as wrong.

## 129. Avoid device detection for layout

Do not implement `if iPhone → layout A`. Prefer capability/layout detection. UA detection is only for genuine platform-specific bugs and should be isolated.

## 130. Final responsive walkthrough

Walk the full product at representative widths: navigation, type, spacing, images, controls, forms, tables, dialogs, loading, errors, empty states, footer. Do not audit only homepage.

## 131. Visual quality check

At each representative viewport evaluate composition, typography, rhythm, hierarchy, interaction target size, overflow, density, and preservation of brand character.

## 132. Transitional widths

Pay special attention to large tablets, small laptops, half-screen browser windows, and split-screen. These intermediate sizes often fail first.

## 133. Windows Snap

Test 50% and 33% snapped desktop windows. The site should remain usable in narrow desktop configurations.

## 134. Split view

Test tablet split-screen, especially CMS/dashboard. Container-driven architecture should remain robust.

## 135. Do not mask content

When layouts change, ensure important content/actions do not disappear, keyboard focus is not lost, and data remains accessible.

## 136. Preserve state on resize

Do not lose selected tab, typed text, filters, scroll state, or editor state merely because a breakpoint changes. Avoid remounting complex components when it destroys state.

## 137. SSR/hydration

Avoid rendering radically different server/client DOM solely from unknown viewport width when it causes hydration mismatch. Prefer CSS adaptation.

## 138. Content visibility

For long pages, `content-visibility` may help where safe; do not apply mechanically.

## 139. Mobile image priorities

Prioritize above-the-fold mobile visuals and avoid preloading desktop-only assets on mobile.

## 140. Quality bar

Success is not merely “nothing overflows”. High quality means intentional composition, preserved typographic character, stable information hierarchy, capability-appropriate interaction, efficient use of space, fluid behavior, functional complex components, and premium visual integrity.

## 141. Readiness criteria

The site is ready only if:
- no unexpected global horizontal overflow;
- works from 320px;
- phone portrait/landscape are complete;
- tablet portrait/landscape are complete;
- small laptop, Full HD, ultrawide, 4K are coherent;
- 200% zoom and text scaling remain usable;
- touch targets work;
- keyboard navigation persists;
- modals fit;
- virtual keyboard does not hide critical actions;
- table strategies are intentional;
- forms work on touch;
- navigation never wraps chaotically;
- media avoids CLS and oversized mobile downloads;
- typography remains readable;
- long content does not break layout;
- Light/Dark both work;
- mobile motion is restrained;
- reduced motion works;
- Safari/Chrome/Firefox/Edge behave correctly;
- quality holds between breakpoints.

## 142. Final responsive decision order

When fixing a problem:

**structure → intrinsic layout → fluid sizing → container query → global breakpoint → local hack as last resort**.

Do not add a breakpoint before understanding the cause. Do not hide an element if it can be rearranged. Do not shrink controls to unreadability. Do not make mobile a crippled desktop copy. Optimize for available space/capabilities, not a named phone model.

## 143. Expected result

The site should behave as one adaptive system:
- compact and usable at 320px;
- expressive around 430px;
- make real use of tablet space;
- remain comfortable on 1366px laptops;
- feel complete at 1920px;
- use ultrawide intelligently.

Core formula:

**Intrinsic Layout + Fluid Typography + Fluid Spacing + Content-driven Breakpoints + Container Queries + Touch Awareness + Height Awareness + Safe Areas + Accessibility + Performance + Real-device Testing = high-quality responsiveness.**

The user should not notice the adaptation mechanism; the product should simply look and work right on their device.
