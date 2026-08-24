# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Premium UI/UX System for Obsidian Plugins

Act as a **Senior Product Designer, Obsidian Plugin UX Architect, Interaction Designer, Design Systems Architect, Frontend Engineer, Accessibility Engineer, and productivity-software specialist**.

Audit and redesign `[PLUGIN NAME]` as a coherent extension of Obsidian rather than an unrelated web app embedded inside it.

Core formula:

**Obsidian Native UX × Editorial Art Direction × Swiss Information Design × Professional Tool Density × Progressive Disclosure × Keyboard-first Workflow × Controlled Motion**.

The plugin should feel native, fast, calm, systematic, productive for long sessions, and distinctive through composition rather than global restyling.

# Module A — Premium Native Obsidian UI/UX

## A1. Native-first rule

Before inventing a custom UI primitive, check whether Obsidian already has the right interaction model. Prefer workspace leaves/ItemView, ribbon, command palette, context/editor/file menus, status bar, modal, notice, settings tab, sidebars, native controls, Obsidian icons, and Obsidian CSS variables.

Custom UI is justified only when core patterns are insufficient.

## A2. Do not redraw Obsidian globally

The plugin must not globally override Obsidian typography, radius, sidebar/editor styles, accent, light/dark theme, or other plugins.

Scope plugin CSS with a namespace such as `.my-plugin`, `.my-plugin-view`, `.my-plugin-settings` and avoid broad selectors such as `.workspace`, `.modal`, `.setting-item`, `button` unless intentionally constrained.

## A3. Transfer ORGNZM principles, not website layout

Bring over typography hierarchy, whitespace, grid, micro-labels, controlled asymmetry, editorial rhythm, large information anchors, minimal chrome, and controlled motion.

## A4. Where editorial style is appropriate

Use it strongly in dashboard/home, project overview, analytics, knowledge overview, onboarding, empty states, full custom views, gallery, timeline, relationship explorer, preview/report screens.

## A5. Where to restrain it

Settings should look like excellent Obsidian settings; modals/context menus/ribbon/editor decorations/command flows should stay native and compact.

## A6. Audit codebase first

Locate Plugin entry, ItemView/WorkspaceLeaf, PluginSettingTab, Modal, Notice, ribbon icons, commands, context menus, status bar, editor extensions, CodeMirror integrations, Markdown post processors, framework components, styles.css, CSS variables, hardcoded colors, inline styles, mobile conditions, resize handling.

Create a UI architecture map.

## A7. Screen inventory

Inventory Ribbon, Command palette, Workspace view, Sidebar view, Editor integration, Context menu, Modal, Settings, Status bar, Notifications, Mobile; define each surface's task.

## A8. Workflow map

Trace real user paths, action count, context switches, keyboard access, waiting, and error probability.

## A9. Obsidian-specific UX principle

Users usually live in notes. Keep frequent actions near the editor through commands, inline actions, context menus, or side views before forcing dedicated fullscreen workspaces.

## A10. Three levels of interaction

Use:
1. Instant — command/hotkey/context/inline.
2. Contextual — popover/modal/sidebar/properties.
3. Workspace — dedicated complex view.

Use the smallest level that fits the task.

## A11. Command-first architecture

Represent every important operation as a reusable command/action so UI surfaces and command palette call the same business logic.

## A12. Command palette

Name commands in human language. Avoid internal implementation names and redundant plugin prefixes if Obsidian already supplies context.

## A13. Ribbon

Use ribbon only for a genuinely frequent global entry action. Prefer 0–1 primary ribbon icons, with other actions in commands/context.

## A14. Icons

Prefer Obsidian/Lucide-compatible icon language. Keep stroke/weight/bounds consistent. Avoid emoji as UI icons and giant extra icon libraries without need.

## A15. Workspace view

Treat ItemView as a real workspace with compact header, primary workspace, optional context panel, and restrained status/actions. Avoid web navbar/card-heavy framing.

## A16. Header

Keep custom-view header compact: title, breadcrumb/context, search, primary action, a few contextual controls.

## A17. Typography

Inherit Obsidian typography with `var(--font-interface)`, `var(--font-text)`, `var(--font-monospace)` according to role. Do not ship a new body font merely for branding.

## A18. Editorial display type

In plugin-owned dashboards, stronger hierarchy and large metrics are allowed, but scale them to the leaf. Never put 160px website headings in a narrow sidebar.

## A19. Micro-labels

Use labels such as STATUS/UPDATED/RELATED/INDEX/PROJECT inside owned views; keep ordinary interface labels sentence case where appropriate.

## A20. Avoid card overload

Prefer typography, spacing, dividers, and list/table structures over a rounded card for every metric.

## A21. Surface hierarchy

Use Obsidian CSS variables such as primary/secondary backgrounds, modifier borders/hover, text normal/muted/faint, and interactive accent or their current semantic equivalents. Avoid hardcoded white/black.

## A22. Accent

By default inherit the user's Obsidian accent. Use it for primary actions, selected/focus state, active navigation, progress, and controlled highlight. Do not force a separate blue brand accent over user themes.

## A23. Plugin-specific color

Add plugin color settings only when they represent data/category/visualization semantics, not to replace the host accent.

## A24. Light/dark

Normal plugin UI follows Obsidian theme automatically. Do not add an independent plugin Light/Dark switch unless a separate preview/output genuinely needs it.

## A25. Community themes

Test Default and diverse community light/dark/high-contrast/custom-radius/custom-font themes. The plugin may adapt visually but must remain readable and functional.

## A26. Theme torture test

Test extreme radius, saturated/unsaturated accent, very dark/light backgrounds, larger interface font. If layout breaks, styling is too rigid.

## A27. Settings

Settings are not a landing page. Use native Obsidian setting rows: section, setting name, concise description, control.

## A28. Settings IA

For many settings group General, Appearance, Behavior, Integration, Advanced. Avoid headings for one control and giant undifferentiated pages.

## A29. Settings search

Only add search/filter when settings volume justifies it; first reduce unnecessary settings.

## A30. Good defaults

Prefer good automatic defaults over exposing implementation decisions as settings.

## A31. Advanced settings

Keep IDs/regex/raw JSON/cache paths and technical knobs in Advanced/debug and explain consequences.

## A32. Reset

Prefer local Restore default for groups/controls over one dangerous global reset.

## A33. Modals

Use small focused modals for confirmation, compact selection, simple creation, focused forms.

## A34. Large workflows

Move complex flows to ItemView/workspace instead of huge modal pages.

## A35. Notices

Use Notice for brief meaningful feedback such as index rebuilt, not every autosave.

## A36. Error UX

Make errors contextual and actionable. Explain the failing resource and recovery action rather than `Something went wrong`.

## A37. Empty states

Never leave custom views blank. Explain why empty and provide the next action.

## A38. Sidebar view

Sidebar space is limited. Use search, compact list, status, contextual controls rather than full dashboard.

## A39. Sidebar priorities

P0 current context/navigation/primary action; P1 status; P2 details; P3 analytics. Move P3 to a main workspace if necessary.

## A40. Master/detail

Wide leaves can show list | detail; narrow leaves navigate list → detail. Optimize for leaf width, not monitor width.

## A41. Container-driven responsiveness

Obsidian panes are independently resizable. Prefer container queries, intrinsic layout, and ResizeObserver only when necessary.

## A42. Container modes

Use content-driven compact/standard/wide/expanded thresholds, e.g. around <360, 360–640, 640–960, >960 as a starting point, not a rigid standard.

## A43. Responsive custom views

Compact = single column; standard = primary + collapsible details; wide = persistent details; expanded = optional analytics/preview. Do not simply scale everything down.

## A44. Editor integration

Editor decorations must be minimal, fast, stable, not interfere with typing, and avoid changing line height unless absolutely required.

## A45. Avoid editor overload

Do not combine many gutter icons, colored highlights, inline buttons, floating toolbar, and badges at once. Writing stays primary.

## A46. Hover controls

Secondary editor actions may appear on hover/focus, but critical actions require keyboard/context alternatives.

## A47. Context menu

Use contextual actions for note/selection/link/file/folder. Keep them relevant and limited, not a 15-item plugin dump.

## A48. Keyboard-first

Expose main workflows through command palette; add view-specific keyboard navigation only where it improves a complex custom view. Avoid forcing default hotkeys; let users assign them in Obsidian where possible.

## A49. Focus

Do not remove focus indicators without a strong accessible replacement. Keyboard focus must always be clear.

## A50. Power-user flow

Complex views may support arrows, Enter, Escape, search, command palette, but avoid inventing a novel keyboard language if standard interactions suffice.

## A51. Search

Large lists need fast search with stable selection, keyboard access, and no lag.

## A52. Filters

Avoid 15 filter pills. Use compact controls/menus/dropdowns/saved views where professional workflows justify them.

## A53. Tables

For data-heavy plugins, use sortable columns, clear selection, compact density, optional sticky header, keyboard navigation, resize-aware columns.

## A54. Responsive tables

Wide = full columns; narrow = essential columns; compact = primary label plus secondary metadata. Never cause global horizontal overflow.

## A55. Lists

For most Obsidian tasks, a clean dense list is better than cards. Show secondary metadata and clear selection.

## A56. Properties/data model integration

If data belongs naturally in note metadata, evaluate Properties/frontmatter/Bases/links instead of creating a parallel proprietary metadata store.

## A57. Linking

Exploit Obsidian links/notes. Use searchable pickers/autocomplete for related notes rather than manual paths.

## A58. Path UX

Do not make users memorize vault paths. Use picker/suggest. Truncate long paths visually but make the full path discoverable.

## A59. Status system

Use consistent Idle/Indexing/Ready/Modified/Syncing/Warning/Failed/Offline/Disabled semantics across surfaces.

## A60. Status bar

Use Obsidian status bar only for genuinely global compact state such as index count or sync progress. Do not turn it into a dashboard.

## A61. Status redundancy

Do not make status bar the sole critical indicator; mobile/layout differences require important status inside plugin views too.

## A62. Plugin design tokens

Build a small plugin token layer over Obsidian variables, e.g. plugin surface/text/muted/border/accent mapped to host tokens.

## A63. No random colors

Search hex/rgb/hsl/gradients and migrate UI colors to semantic variables unless color represents user data.

## A64. Product/data colors

Project/category/graph-series colors may be independent data colors, while core surfaces/controls still inherit Obsidian.

## A65. Spacing system

Create xs/sm/md/lg/xl spacing tokens based on Obsidian rhythm rather than random 7/13/19/27px values.

## A66. Radius

Inherit host/theme radius variables where possible. Do not force one brand radius across community themes.

## A67. Shadows

Use very sparingly; host surfaces and borders are usually enough.

## A68. Glassmorphism

Avoid foundation-level blur/transparency due to theme conflicts, performance, and visual noise.

## A69. Performance is UX

Evaluate startup, rendering cost, event frequency, vault size, editor latency, memory, and mobile for every UI feature.

## A70. Large vault

Test representative vault sizes such as 100, 5k, 50k notes when architecture permits. Do not render the full dataset to show a small viewport.

## A71. Lazy rendering

Use virtualization/paging/incremental rendering for large lists.

## A72. Startup

Do not construct a heavy dashboard on startup if its view is closed. Initialize work lazily.

## A73. View lifecycle

Views must open/close cleanly, unregister events/observers/timers, unmount framework roots, and persist only intended state.

## A74. Framework choice

Do not rewrite vanilla/React/Svelte simply for design. Evaluate existing architecture first.

## A75. Internal components

A small internal layer such as Toolbar, SectionHeader, SearchBox, EmptyState, StatusIndicator, ListRow, Metric, Inspector, Progress is useful. Do not create a giant proprietary design system over Obsidian.

## A76. Design signature

Use 1–3 distinctive compositional elements: editorial headers, selected indicator, large metrics, signature transition, unique visualization. Distinctiveness should come from composition, not recoloring Obsidian globally.

# Module B — Motion, Icons, States & Feedback

## B1. Goal

Find every place where the user might ask “What happened?” and add clear feedback.

## B2. Motion should be calmer than a marketing site

Typical ranges: micro 80–140ms, controls 120–180ms, panels 160–240ms, large transitions 200–320ms. Avoid 800ms cinematic transitions in productivity flows.

## B3. Motion tokens

Centralize fast/normal/panel durations and standard/out easing; avoid random durations.

## B4. Animate state, not decoration

Good targets: panel open/close, disclosure, progress, selection, save/sync/index state, drag reorder, status change.

## B5. Avoid editor/list animation noise

Do not animate editor text, every list row, sidebar on each selection, persistent icons, gradients, decorations.

## B6. Icon motion

Refresh rotates once, sync while active, chevron rotates with disclosure, check appears at completion, warning pulses once. No perpetual animation.

## B7. Loading categories

Near-instant operations need no indicator; short async uses spinner+text; deterministic long tasks show progress; indeterminate long tasks show spinner+current stage.

## B8. Progress fidelity

Prefer real counts such as `648 / 1,482 notes` over fake percentages.

## B9. Long operations

Show current stage/file and safe Cancel where appropriate.

## B10. Background work

Do not block editor if background execution is safe; keep compact status visible.

## B11. Success

Use small `Saved`/check feedback, not large animations.

## B12. Error

One restrained attention animation is enough. No blinking red.

## B13. Status indicator

Do not rely on color alone; combine glyph/text such as `● Ready`, `! Warning`, `× Failed`.

## B14. Semantic colors

Use Obsidian semantic color variables rather than a parallel red/green system.

## B15. Reduced motion

Honor user motion preference; remove decorative transforms while retaining state feedback.

## B16. Hover

Hover is enhancement only. Touch and keyboard alternatives remain mandatory.

## B17. Drag/drop

Clearly show source, target, insertion point without chaotic layout shifts.

## B18. Transient feedback

Choose inline state or Notice based on context; do not duplicate already-obvious feedback.

# Module C — Settings, Configuration & Power-user UX

## C1. Goal

Settings should be understandable without routinely consulting README.

## C2. Reduce settings first

For every option ask why the user must decide it; automate with a good default when safe.

## C3. Group by task

Use General, Indexing, Appearance, Integrations, Advanced or equivalent meaningful categories.

## C4. Description

Explain non-obvious controls briefly: what changes and when it matters.

## C5. Native controls

Prefer native toggle/dropdown/text/slider/button/searchable suggest over custom controls merely for style.

## C6. Live settings

Apply changes immediately when safe; avoid a global Save button without technical reason.

## C7. Validation

Validate paths/settings before persistence and show contextual error.

## C8. File/folder picker

Use searchable suggestions rather than raw path typing.

## C9. Appearance

Reasonable plugin-owned settings may include Density and Animations/Reduced motion, but do not duplicate Obsidian's host Light/Dark for ordinary UI.

## C10. Accent

Default to `Use Obsidian accent`; plugin-specific data colors must be framed separately.

## C11. Advanced

Separate technical/destructive settings and explain consequences.

## C12. Reset

Prefer local/group reset.

## C13. Import/export

For complex plugins, export/import configuration where useful; exclude secrets/private content unless explicitly safe.

## C14. Diagnostics

Provide a separate Diagnostics area for plugin version, Obsidian version, platform, index/cache state, last error.

## C15. Copy diagnostics

Offer safe copy for support, excluding secrets/private note contents/sensitive paths unless needed and clearly handled.

# Module D — Responsive Obsidian Plugin UI

## D1. Leaf width is the primary viewport

Do not design solely from monitor width.

## D2. QA widths

Check custom views around 240, 320, 400, 600, 800, 1200, 1600px as QA points, not rigid breakpoints.

## D3. Height

Also test small panes such as 600×250 after nested splits.

## D4. Container-driven design

Prefer container queries to viewport-only media queries.

## D5. Compact mode

Remove secondary columns/metadata, switch to list/detail navigation, keep primary actions.

## D6. Wide mode

Show useful details/preview/additional context while keeping reading measures sensible.

## D7. Resize state

Do not lose selection, scroll, filters, input, or edit state on resize; avoid full remount when possible.

## D8. Mobile-specific pass

Obsidian Mobile is not desktop minus sidebar. Design separately.

## D9. Mobile priorities

Keep primary action, search, viewing, basic edit, back navigation, error recovery.

## D10. Touch controls

Visual icons may be small but hit areas must be comfortable.

## D11. Hover-free mobile

Everything important must have tap/visible/context alternatives.

## D12. Mobile modal

Convert oversized modals to fullscreen/sheet-like flows where useful.

## D13. Virtual keyboard

Bottom controls must not cover active input.

## D14. Desktop-only backend

If mobile cannot support a desktop-only Node/Electron operation, provide an explicit disabled/fallback UX rather than a dead button.

## D15. Mobile navigation

Do not add a permanent proprietary navbar over Obsidian Mobile without strong need.

## D16. Orientation

Test phone/tablet portrait and landscape.

## D17. Split workspace

Test single leaf, vertical split, horizontal split, nested splits.

## D18. Sidebar adaptation

A plugin view opened in sidebar should adapt rather than require 600px minimum width.

## D19. Minimum viable width

If a complex visualization cannot fit, offer `Open in main workspace` instead of rendering broken UI.

# Module E — Obsidian CMS / Knowledge-management UX

## E1. Do not create a second data universe without need

Evaluate notes, Properties, links, tags, folders, Bases, frontmatter before adding proprietary storage.

## E2. Note-centric CRUD

Person/Project/etc. entities should map to understandable user-visible notes when appropriate and remain openable as notes.

## E3. Search

Large plugin databases need search while keeping entities discoverable through Obsidian search where possible.

## E4. Master/detail

Use items + inspector on wide panes and navigable single-panel mode on narrow panes.

## E5. Create flow

Typical entity creation should be short: New project → Name → defaults/template → Create; advanced fields later.

## E6. Inline edit

Allow quick status/owner/category/date changes in context rather than forcing Settings.

## E7. Relationship autocomplete

Person/Project/Company/Parent/Related note relationships use autocomplete rather than raw wikilink/path entry.

## E8. Backlink context

Show useful linked-note context without merely cloning the core Backlinks view.

## E9. Status vocabulary

Use a consistent model such as Active/Paused/Done/Archived where domain-appropriate.

## E10. Dense list/table

Hundreds of entities need compact lists/tables, not giant cards.

## E11. Saved views

Professional flows may expose Active/My items/Recently updated/Needs attention.

## E12. Filters/query model

Keep filters aligned with user data and avoid a proprietary query language unless truly needed.

## E13. Bulk operations

Use only where they save real work; always show selected count.

## E14. Delete semantics

Distinguish Remove from plugin, Archive, Delete note. Never unexpectedly delete user vault data.

## E15. Source of truth

Make it clear where data lives. If note/frontmatter is canonical, do not create invisible duplicate state.

# Module F — Final Obsidian UI/UX Audit

## F1. Full lifecycle audit

Trace startup → discoverability → command → navigation → interaction → feedback → persistence → recovery → mobile → unload.

## F2. Native compatibility

Verify CSS scoping, no collateral core/plugin impact, semantic host variables, accent inheritance, light/dark/community-theme compatibility.

## F3. CSS audit

Search `!important`, hardcoded colors, inline styles, global selectors, fixed width/height, huge z-index, `overflow:hidden`, unscoped animation. Evaluate each occurrence, not mechanically.

## F4. UX audit

For every action ask whether it can be performed closer to current context. Long trips from note → settings → plugin → advanced → save → back indicate friction.

## F5. Discoverability

A new user should know how to open the plugin, what it does, and the main action without reading a long README.

## F6. Power-user path

Commands, keyboard, search, and minimal modal usage should support experts.

## F7. Focus audit

Use keyboard only; check tab order, focus visibility, Enter/Escape, modal focus, focus restoration.

## F8. Screen-reader audit

Icon-only buttons need names; states cannot be color-only; forms need labels.

## F9. Motion audit

For each animation record Trigger, Purpose, Duration, Reduced-motion behavior; remove if purpose is unclear.

## F10. Theme audit

Test Default Light/Dark plus high-contrast-ish and heavily customized community themes. Exact visual identity may vary, functionality/clarity may not.

## F11. Accent torture test

Test blue/red/green/yellow/purple, low/high saturation; focus/selection must remain readable.

## F12. Font test

Test larger interface font, custom community fonts, monospace overrides; avoid fixed heights dependent on one text metric.

## F13. Resize test

Continuously resize leaf and find intermediate failure points.

## F14. Mobile test

Execute actual flows: Open → Search → Select → Edit → Confirm → Back, not screenshots only.

## F15. Large-vault test

Measure startup, search, filters, list rendering, indexing progress, memory.

## F16. Empty-vault test

UI should remain coherent with almost no content.

## F17. Extreme-data test

Test long filenames/paths, Unicode, Cyrillic, emoji titles, thousands of items, missing files, deleted targets.

## F18. Error recovery

Inject missing folder, invalid config, network failure, permission error, corrupted cache, interrupted operation and ensure recovery is clear.

## F19. Disable/enable lifecycle

On disable clean views/events/timers/observers/framework roots; on re-enable recover correctly.

## F20. Performance

Audit editor typing latency, workspace resizing, scrolling, search, view/modal opening. The plugin must not degrade baseline Obsidian.

## F21. Final visual polish

Only after UX/performance: normalize spacing, reduce borders, unify icons/selection/status colors/type, remove noise.

## F22. Remove-20% test

Try removing around 20% of decorative cards/badges/icons/separators/labels/buttons. Keep them removed if clarity improves.

## F23. Native test

Ask: if this were a core Obsidian feature, would it plausibly look/behave like this? If not, identify why.

## F24. Brand test

Then ask whether the plugin still has character. Add character through composition, spacing, hierarchy, visualization, microinteraction—not global recoloring.

## F25. Quality bar

Aim for **Obsidian core feature + Linear-level polish + Raycast-level interaction speed + editorial hierarchy + IDE-level information density**.

# Module G — Existing-codebase implementation workflow

## G1. Discovery

Before editing, locate main plugin entry, views, components, settings, styles, commands, editor extensions, mobile code, storage, state management, tests.

## G2. UI architecture map

Produce `Surface | Component | Source file | Purpose | Problems | Proposed change | Risk | Tests`.

## G3. Design-token audit

Find colors, radius, spacing, transitions, typography, shadows and map `Obsidian variables → plugin semantic tokens → components`.

## G4. Native integration audit

Identify custom implementations that can be replaced by native Obsidian patterns, but do not rewrite working code without benefit.

## G5. UX refactor first

Fix navigation, workflow, hierarchy, state feedback before animation.

## G6. Responsive architecture

Repair fixed widths, pane resizing, compact modes, mobile.

## G7. Visual system

After UX, normalize spacing, typography, surfaces, accent, icons.

## G8. Motion

Add motion after layout is stable.

## G9. Accessibility

Audit focus, keyboard, touch, screen reader, reduced motion, color independence.

## G10. Performance

Profile startup, renders, events, editor integration, large vault.

## G11. Theme compatibility

Test community themes; never “fix” compatibility by hardcoding Default theme.

## G12. Tests

Add component/state/layout/E2E/visual tests where practical.

## G13. Final audit

Repeat **Native → UX → Responsive → Accessibility → Performance → Visual polish**.

# Anti-pattern checklist

Do not ship:
- a separate web design language that ignores Obsidian;
- hardcoded light/dark backgrounds;
- global accent changes;
- custom body font;
- endless rounded cards;
- decorative gradients/glassmorphism/giant shadows;
- random icons or emoji UI;
- hover-only actions;
- mobile-unusable controls;
- fixed 900px workspaces;
- huge modals;
- loading screens for local operations;
- permanent spinners;
- color-only status;
- default hotkeys for many commands;
- ribbon icons for every feature;
- duplicate command logic;
- raw file-path fields where picker is possible;
- editor decorations that change layout;
- heavy editor animations;
- polling where events exist;
- global CSS overrides;
- `!important` as an architecture strategy;
- gigantic `main.ts` containing all UI;
- event listeners without cleanup;
- framework roots without unmount;
- a framework/design system introduced just for a couple of controls.

# Final philosophy

A strong Obsidian plugin should feel as though **Obsidian always should have supported this capability**.

Distinctiveness should come from composition and product quality, not fighting the host application.

Final formula:

**Native behavior + Obsidian design tokens + excellent information architecture + minimal navigation distance + keyboard-first workflows + container-responsive layout + subtle editorial art direction + restrained motion + theme compatibility + mobile parity + accessibility + high performance.**

When requirements conflict, prioritize:

**User data safety → Native Obsidian behavior → Task efficiency → Information clarity → Keyboard/touch accessibility → Theme compatibility → Performance → Responsive behavior → Visual hierarchy → Motion → Decoration**.
