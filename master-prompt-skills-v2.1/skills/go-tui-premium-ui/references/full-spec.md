# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Premium UI/UX for Go TUI Applications

Act as a **Senior Product Designer, Terminal UI/UX Designer, Go Architect, CLI/TUI Engineer, Information Architect, and keyboard-first interaction specialist**.

Audit and design `[PROJECT NAME]`, a Go TUI application, as a modern professional product rather than a colored CLI menu.

Preferred stack when appropriate:
- Bubble Tea for state/update/view;
- Lip Gloss for layout/style;
- Bubbles for interactive primitives;
- Charm ecosystem;
- terminal capability detection;
- mouse as optional secondary input;
- keyboard-first as the baseline.

If a different TUI framework is already in use, do not rewrite solely for stack preference.

## 1. Goal

Aim for the interaction quality of **Raycast × Linear × modern IDE × htop/lazygit-class efficiency × premium minimalism**, adapted to terminal constraints.

The UI should be fast, calm, readable, appropriately dense, keyboard-first, visually systematic, usable in small and large terminals, self-explanatory, and suitable for long sessions.

Formula:

**Hierarchy + Alignment + Keyboard Flow + Context + Feedback + Restraint**.

## 2. Avoid old-style TUI anti-patterns

Do not build the whole app out of heavy ASCII boxes, numbered menus, `Enter option number:` prompts, excessive colors, internal implementation vocabulary, or BIOS-like flows. Use terminal space as a modern canvas.

## 3. UX audit first

Map frequent, dangerous, long-running, data-heavy, settings, logs, error, search/filter, launch, and background-job flows.

For each record:

`Goal → Current path → action count → keyboard movement → context switches → pain points → error risk → proposed improvement`.

Fix UX before polish.

## 4. Information architecture

Create a clear top-level structure appropriate to the product, e.g. Dashboard, Projects, Tasks, Processes, Logs, Settings.

For complex apps use:

`Navigation → Workspace → Context → Actions`.

Users must always know where they are, what is selected, what they can do, and what is happening.

## 5. Main layout

For desktop-like terminal sizes, prefer a primary workspace with optional navigation/context/status layers. Do not surround every region with a box. Separate with whitespace, subtle background, one rule, typography, or alignment.

## 6. Three-layer interface

Use:
1. Primary workspace.
2. Context/detail.
3. Commands/key hints.

Do not force users to remember operations from documentation.

## 7. Top bar

If useful, use a compact one-line bar for app name, workspace/breadcrumb/profile, and status such as `● CONNECTED`. Do not build a giant header.

## 8. Status bar

Use a contextual bottom status bar, e.g. `NORMAL main 12 items ● synced ? help` or `RUNNING scan 74% 186/250 files esc cancel`.

## 9. Status bar is not a 40-key manual

Show roughly 3–6 currently relevant hints such as `↑↓ move`, `enter open`, `/ search`, `? help`, `q quit`. Update hints with mode/context.

## 10. Command palette

For complex apps, add `Ctrl+K` or `:` command palette for commands, navigation, search, settings, and infrequent actions. Fuzzy-match results.

## 11. Fuzzy search

Use `/` or equivalent for large lists; filter incrementally and show result count such as `12 / 381 results`.

## 12. Keyboard-first baseline

Support consistent core keys: arrows, Enter, Esc, Tab/Shift+Tab, `/`, `?`, `q` where appropriate.

## 13. Avoid shortcut hell

Do not assign a random key to every action. Use predictable mnemonics such as `n`, `e`, `d`, `r`, `/`, `?` only for frequent operations.

## 14. Optional Vim navigation

Power-user aliases `j/k`, `h/l`, `g/G` are fine, but arrows must continue to work unless product explicitly targets Vim users.

## 15. Explicit modes

For complex flows, consider `NORMAL`, `SEARCH`, `EDIT`, `COMMAND`, `SELECT`, `RUNNING`. If modes exist, show the active mode and make escape behavior predictable.

## 16. Visual hierarchy

Use clear levels: Primary, Secondary, Muted, Disabled, and Accent for active focus/selection. Do not render everything with equal weight.

## 17. Typography in a terminal

Hierarchy comes from bold, faint/muted, limited uppercase, spacing, indentation, color, alignment, and separators. Avoid all-caps body text.

## 18. Micro-labels

Use small labels such as PROJECT, STATUS, OWNER, UPDATED, ENVIRONMENT or numbered sections to structure complex screens.

## 19. Semantic color system

Centralize tokens for background, surface, active surface, text, secondary/muted text, accent, accent-soft, success, warning, error, info, border, active border. Avoid arbitrary ANSI colors in components.

## 20. TrueColor + fallback

Use curated TrueColor when available but degrade gracefully to 256 colors, 16 colors, and no-color.

## 21. `NO_COLOR`

Respect `NO_COLOR`. Selection/state must remain understandable without color.

## 22. Light/dark terminal

Do not assume black background. Support `Auto`, `Dark`, `Light` where appropriate and detect terminal background when reliably possible.

## 23. Accent color

Optional accent presets can be Blue/Cyan/Teal/Green/Amber/Orange/Violet. Use accent for selection, focus, active navigation, progress, and primary action—not the whole UI.

## 24. Semantic colors independent of accent

Error/warning/success meaning must stay stable when accent changes.

## 25. Calm palette

Avoid raw saturated RGB, acidic colors, many bright hues, and huge colored backgrounds. Prefer muted neutrals plus one accent.

## 26. Borders

Use consistent single or rounded box-drawing glyphs sparingly. Do not mix styles or border every component.

## 27. Fewer boxes

Prefer label/value hierarchy and whitespace over `┌─────┐` around every small section.

## 28. Whitespace

Use blank rows, 1–2 columns of padding, and indentation deliberately. Do not fill every terminal cell.

## 29. Alignment

Align metadata columns for scanning, e.g. Status, Environment, Version, Updated.

## 30. List design

Use rows with compact metadata and clear active marker such as `›` or `▸`. Do not pile on marker + bold + background + multiple colors simultaneously unless needed.

## 31. Active row accessibility

Selection must remain visible without color, e.g. `› postgres`.

## 32. Tables

Use aligned columns, logical widths, truncation, optional horizontal scroll, and frozen identity column where helpful. Long content must not break the whole layout.

## 33. Responsive tables

Use column priorities: wide shows full metadata, compact fewer columns, tiny only identity/status with details in a secondary view.

## 34. Master/detail

Use list/detail side-by-side when space allows; on narrow terminals use list → Enter → detail rather than squeezing both.

## 35. Responsive TUI

Handle terminal resize through `tea.WindowSizeMsg` or the framework equivalent. Layout must change without restart.

## 36. Size classes

Define content-driven classes such as Tiny, Compact, Medium, Wide, UltraWide. Example starting ranges may be `<60`, `60–89`, `90–119`, `120–159`, `≥160` columns, but tune to actual content.

## 37. Height classes

Account for Short/Normal/Tall. The app must work at `120×25` and `180×50`, not just one ideal size.

## 38. Minimum terminal size

If below a true minimum, show an explicit required/current size message rather than a broken layout.

## 39. Ultrawide terminal

On 200+ columns, expose useful details/preview/log context, but keep prose readable rather than stretching to 200 columns.

## 40. Responsive panels

Switch among 3-column, 2-column, single-column layouts instead of shrinking all panels beyond usability.

## 41. Panel focus

Make active panel obvious through one restrained mechanism such as active border or title accent.

## 42. Breadcrumbs

Show contextual paths such as `projects / hydropilot / deploy`, and shorten intelligently when narrow.

## 43. Input

Inputs need visible focus, cursor, placeholder, and validation.

## 44. Form UX

Group large forms and use progressive disclosure/tabs/wizards only where the task is genuinely sequential.

## 45. Select

Provide fuzzy typing for large option lists.

## 46. Autocomplete

Autocomplete filenames, commands, servers, projects, branches, profiles and similar values to reduce typing.

## 47. Inline validation

Show field-specific errors beside the input with actionable text, not only a generic error at screen bottom.

## 48. Modals

Reserve for destructive confirmation, small isolated flows, or prompts. Do not use modal for every setting.

## 49. Modal design

Keep dialog content focused, show consequences, make focus obvious, and do not preselect destructive action by default.

## 50. Confirmation policy

Confirm destructive/irreversible/expensive/externally visible actions. Prefer undo for reversible operations.

## 51. Toast/flash messages

Use short non-blocking status messages such as `✓ Configuration saved`.

## 52. Success

Keep ordinary success compact; no giant ASCII art after Save.

## 53. Errors

Explain what failed, why when known, and immediate recovery actions. Keep stack traces behind Details/debug.

## 54. Debug detail

Allow explicit `Show details` / `d details` for stack/raw error/request/timestamps where useful.

## 55. Loading

Use spinner for short indeterminate work and progress for long determinate work.

## 56. Progress

Show real percentage/count/bytes where known. Never invent progress.

## 57. Progress detail

For long jobs show step number, current stage/file, elapsed time where reliable.

## 58. Cancel

Expose `esc cancel` or `ctrl+c cancel` for safely cancellable operations.

## 59. Background tasks

Do not block the whole UI when work can continue in background. Surface `N jobs running` and a jobs view.

## 60. Jobs view

Show job name, running/queued/done state, progress, age, and relevant controls.

## 61. Log viewer

Provide follow, pause, search, filter, level, timestamps toggle, wrap toggle, clear viewport, copy.

## 62. Log levels

Use compact labels such as INF/WRN/ERR/DBG and avoid painting the whole line bright colors.

## 63. Live logs

Make FOLLOW state visible; when user scrolls up, pause auto-follow and offer `f resume`.

## 64. Scroll position

A simple `42 / 381` indicator may be more useful than a decorative scrollbar.

## 65. Empty state

Explain why nothing is present and offer the next action, e.g. `n new server`.

## 66. First run

Use brief actionable onboarding rather than a huge manual.

## 67. Help overlay

`?` should open contextual key help grouped by Navigation, Actions, Global.

## 68. Help is not the manual

Keep help overlay as a cheat sheet; full documentation belongs elsewhere.

## 69. Mouse

Support click/wheel/selection/resizing optionally, but keep the app fully usable without mouse.

## 70. No hover dependency

Do not put critical information only behind terminal hover.

## 71. Copy/selection

Mouse handling must not make ordinary terminal text selection impossible without a documented escape/fallback.

## 72. Clipboard

When clipboard integration exists, show `✓ Copied`; provide fallback if clipboard APIs are unavailable.

## 73. Iconography

Use restrained Unicode symbols (`✓`, `!`, `✕`, `●`, `○`, `▸`, `→`, arrows) rather than emoji clutter.

## 74. Nerd Fonts

Never make Nerd Font mandatory. Optional enhanced icon mode can exist, but standard terminal fonts must remain first-class.

## 75. ASCII fallback

Where environments require it, degrade Unicode symbols to `[OK]`, `[X]`, `*`, `-`, etc.

## 76. Theme architecture

Centralize theme data in a Go type or equivalent. Components should not hardcode colors.

## 77. Light/Dark/Auto settings

Theme and accent selection should apply immediately and persist.

## 78. Accessibility

Operate without color, without mouse, at small sizes, and with keyboard-only workflows. Do not communicate state only through foreground color.

## 79. Contrast

Audit muted text, selected item, warning, status across common terminal themes.

## 80. Animation

Use terminal motion only where useful: spinner, progress, brief state transition, subtle loading frame, icon state. Avoid decorative continuous ASCII animation.

## 81. FPS

Do not repaint at 60 FPS by default. Tick only components that truly animate; conserve CPU, battery, and SSH bandwidth.

## 82. SSH

The TUI must remain good at 80×24, with latency, limited colors, no mouse, and standard fonts.

## 83. Remote performance

Minimize huge repaint churn, especially from decorative animation.

## 84. Resize state preservation

Resize must not reset selection, close editors, lose input, or restart jobs. Change layout, keep state.

## 85. State architecture

For Bubble Tea, separate domain state, UI state, and transient state. Do not put everything in one giant model.

## 86. Component model

Break complex screens into components such as Header, Sidebar, Workspace/List/Detail, StatusBar, each with Update/View/Size/Focus/State responsibilities.

## 87. Avoid god model

If one `model.go` handles every key and thousands of lines, refactor navigation, commands, keymap, styles, views, and domain concerns.

## 88. Keymap architecture

Centralize keys in a `KeyMap` structure or equivalent rather than raw `case "j"` scattered through components.

## 89. Context keymap

Component-specific commands should activate only when focused; globals should be handled centrally.

## 90. Command model

For scalable apps, model actions as commands such as `project.new`, `project.open`, `job.cancel`, `app.settings` shared by keymap, palette, context menu, help.

## 91. Action registry

A registry can define Command, Label, Shortcut, Enabled, Visible, Execute and prevent duplicate action logic.

## 92. Disabled actions

Hide unavailable context actions or show them disabled with an explicit reason.

## 93. Context-aware actions

Status bar/help should show commands for the currently selected entity rather than all commands globally.

## 94. Dashboard

Use dashboard only for actionable status: system health, running/failed jobs, last backup, recent items. Avoid vanity KPI.

## 95. KPI

Large numbers may anchor important state, but only if operationally meaningful.

## 96. Status vocabulary

Define consistent states such as idle, queued, running, success, warning, failed, cancelled, offline, unknown.

## 97. Connection state

Show explicit `Connected`, `Connecting`, `Offline`, `Error` without endless blinking.

## 98. Dangerous operations

For delete/format/force push/production deploy/destructive migration, show target, version/change context, and explicit cancel/confirm. Do not use context-free `Are you sure? y/n`.

## 99. Context-rich confirmation

Explain exactly what will happen and what is irreversible.

## 100. Settings UX

Group General, Appearance, Network, Integrations, Storage, Advanced rather than one huge form.

## 101. Search settings

Allow `/ proxy` filtering when settings become large.

## 102. Settings descriptions

Explain non-obvious settings briefly in context.

## 103. Config-file integration

TUI and YAML/TOML/JSON config must share one source of truth. Do not create two independent settings systems.

## 104. Raw config preview

Power users may get View config/Edit raw config as an advanced feature.

## 105. Unsaved changes

Show unsaved count and protect exit, or use safe autosave when appropriate.

## 106. File picker

Provide a usable file browser/search/hidden-files toggle/path input instead of requiring full manual paths.

## 107. Tree view

For hierarchies, support expand/collapse/search/current selection with clear tree glyphs.

## 108. Multi-select

Support batch selection with `[x]` and visible count/context actions.

## 109. Filtering

Keep active filters visible and editable.

## 110. Sort

Expose sort state with an arrow or status text.

## 111. Persistence

Persist theme, accent, last workspace, panel widths, density, sort/filter where useful, not transient state that causes confusing restarts.

## 112. Density

Offer Comfortable/Compact where logs/tables/monitoring benefit.

## 113. Small terminal mode

Hide secondary metadata, switch to single-column, shorten header/status hints while retaining critical actions.

## 114. Large terminal mode

Use extra room for detail panel, logs, help hints, preview, not empty filler.

## 115. Long text

Support wrapping/scrolling/copy and line numbers where relevant. Do not truncate essential error/detail content irreversibly.

## 116. Truncation

Lists/tables may ellipsize, but full selected content should be visible in detail.

## 117. Terminal title

Optionally set terminal title such as `IRIS — Production`; do not make it critical to comprehension.

## 118. Alternate screen

Use alternate screen for full TUI, but keep direct commands like `mytool status` as script-friendly output rather than forcing full-screen mode.

## 119. CLI + TUI

A strong Go tool can expose both `mytool` TUI and scriptable subcommands such as status/backup/list over the same domain layer.

## 120. Scriptability

Preserve Unix composability. Domain logic should be reusable by CLI, TUI, API, automation.

## 121. Architecture

A possible structure:

```text
cmd/
internal/
  app/
  domain/
  service/
  config/
  tui/
    model/
    views/
    components/
    styles/
    keymap/
    commands/
    theme/
```

Adapt to existing project rather than creating folders ceremonially.

## 122. Domain isolation

Business logic must not import Bubble Tea/Lip Gloss/terminal types. Treat TUI as adapter/interface layer.

## 123. Async operations

Do not block `Update`. Use commands/messages: keypress → command → background operation → message → state update → view.

## 124. Explicit loading state

Use state such as idle/loading/loaded/error rather than `data == nil` as the only loading indicator.

## 125. FSM for real workflows

Use explicit state machines for connection/deployment/update/backup/wizards when multiple real states exist; do not introduce FSM framework for trivial two-state logic.

## 126. Error isolation

A local panel error should not necessarily crash the whole TUI.

## 127. Panic recovery

Restore terminal raw/alternate-screen state on unexpected failures so users are not left with a broken terminal.

## 128. Logging

Never print application logs directly over the active TUI. Use structured log files or a dedicated debug/log view.

## 129. Debug mode

Optional `--debug` should write diagnostics to a file and expose the path in UI.

## 130. Performance

Avoid unnecessary allocations/recomputation inside View. Profile large tables, logs, tree, fuzzy search. TUI should feel immediate.

## 131. Large datasets

Virtualize visible rows, filter incrementally, search efficiently, maintain stable selection. Do not render 50,000 rows to show 30.

## 132. Flicker

Let the renderer manage updates; do not manually clear the full screen on every frame.

## 133. Reflow

Resize should recalculate layout, not expensive domain data.

## 134. Testing

Add unit tests for layout calculations, keymap, state transitions, filters, commands, theme selection.

## 135. Golden tests

Use golden render tests at representative sizes such as 80×24, 120×32, 160×45.

## 136. Responsive test matrix

Test at least 60×18, 80×24, 100×30, 120×30, 120×40, 160×45, 200×60.

## 137. Cross-terminal testing

Test Windows Terminal, PowerShell, cmd if supported, Linux terminals, macOS Terminal/iTerm, SSH, tmux, screen as relevant.

## 138. Windows

Specifically validate Unicode, ANSI, resize, clipboard, paths, keyboard codes. Do not assume Linux-only semantics.

## 139. tmux

Test color detection, mouse, terminal size, and key conflicts. Avoid critical shortcuts commonly intercepted by multiplexers.

## 140. UX quality pass

For every screen ask: Is focus visible? Location clear? Selection clear? Main action visible? System state clear? Too many borders/colors? Can text be reduced? Can the task be faster?

## 141. Keyboard flow test

Complete core workflows without leaving the keyboard. If navigation feels erratic, redesign it.

## 142. Beginner test

A first-time user should infer basic operation from focus, labels, contextual hints, help—without reading README first.

## 143. Power-user test

Expert users need shortcuts, fuzzy search, command palette, and direct navigation.

## 144. Accessibility test

Turn off colors; focus/state must remain clear.

## 145. Small-screen test

80×24 must remain a real working mode.

## 146. SSH test

Under latency/bandwidth constraints, remove decorative high-frequency updates.

## 147. Final visual polish

After UX/architecture work, normalize spacing, borders, styles, colors, columns, titles; remove noise before adding decoration.

## 148. Design signature

Use one accent, characteristic selected indicator, clean status bar, section labels, or distinctive dashboard composition. A few signatures are enough.

## 149. Gradients

ANSI gradients may appear in logo/welcome/progress/branding but working screens should remain calm.

## 150. ASCII art

Use only in first-run/about/empty onboarding; do not consume useful space every launch.

## 151. Cognitive load

TUI can show lots of data but should favor **overview → selection → details** over “everything everywhere”.

## 152. Information priority

Classify content as P0 Critical, P1 Primary, P2 Secondary, P3 Diagnostic and use this in responsive layouts.

## 153. Contextual detail

Put secondary data in detail panels and diagnostics in advanced views.

## 154. Color is semantic

Every color should represent state, focus, category, or priority. Remove decorative color that conveys nothing.

## 155. Bold is scarce

Use bold for active item, title, important value. If half the screen is bold, it loses meaning.

## 156. Muted must remain readable

Do not make secondary information too faint.

## 157. Focus ≠ selection

Distinguish keyboard focus, selected object, and active state, especially in multi-panel layouts.

## 158. Visual-language reference

A good composition can use one-line app/status header, dense service table, contextual detail region, and context-aware bottom key hints. Treat examples as direction, not a template.

## 159. Existing-program mode

Before redesign, find root model, views, components, styles, keymaps, WindowSize handling, navigation, state management, async operations, theme logic.

Create:

`Problem → file/component → UX impact → architecture cause → fix → verification`.

Preserve business logic unless redesign requires behavioral change.

## 160. Refactor before polish

If all key events live in one 1500-line `Update`, fix architecture before layering Lip Gloss styling on top.

## 161. Implementation order

1. UX inventory.
2. Information architecture.
3. Application state.
4. Keymap.
5. Responsive layout.
6. Theme.
7. Core components.
8. Search/command palette.
9. Async UX.
10. Error UX.
11. Accessibility.
12. Performance.
13. Tests.
14. Visual polish.

## 162. Readiness criteria

Ready only if:
- core workflow is keyboard-complete;
- focus always visible;
- selected state works without color;
- navigation/Esc are predictable;
- search/help/status hints are immediate and contextual;
- long operations do not freeze UI and show real progress/cancel where safe;
- errors explain recovery;
- resize preserves state;
- 80×24 and large terminals both work;
- SSH remains comfortable;
- TrueColor, mouse, Nerd Font are optional;
- NO_COLOR and light/dark terminals work;
- Windows/Linux terminals work;
- borders/colors/shortcuts/animations are not random;
- business logic is TUI-independent;
- screens are componentized;
- large datasets remain performant.

## 163. Final priority

Use:

**Task completion → Keyboard efficiency → Information clarity → State visibility → Responsive behavior → Accessibility → Visual hierarchy → Animation → Decoration**.

Remove borders that waste space, motion that hurts SSH, shortcuts that are hard to remember, and secondary information that blocks the main task.

## 164. Expected result

The Go TUI should feel like a full professional application inside the terminal, combining minimalism, professional density, keyboard speed, modern composition, contextual hints, responsive behavior, sound Go architecture, smooth performance, and clear system states.

Do not imitate GUI pixel-for-pixel. Exploit terminal strengths: speed, density, keyboard, text, tables, instant navigation, and automation-friendly architecture.
