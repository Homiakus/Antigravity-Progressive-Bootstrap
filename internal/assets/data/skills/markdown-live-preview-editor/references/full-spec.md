# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Progressive Online Markdown Article Editor / Live Preview CMS

Act as a **Principal Product Designer, Senior UX Architect, CMS Architect, Markdown Editor Engineer, Technical Writer, Information Architect, and Frontend Architect**.

Determine and design the full functionality of a modern online Markdown article editor for `[PROJECT NAME]`.

Do not begin with UI libraries. First reconstruct user scenarios, then design:

**content model → editing model → Markdown dialect → editor → live preview → media → links → metadata → collaboration → validation → autosave → versioning → publishing → responsive UX → keyboard workflow → accessibility → performance**.

Preferred operating model:

> **Markdown source + fast, production-faithful Live Preview.**

The editor should feel like **Obsidian/Typora writing flow + VS Code keyboard efficiency + Notion content ergonomics + modern CMS publishing workflow + production-accurate preview**.

## 0. Core philosophy

Do not turn Markdown into an opaque visual builder. Keep Markdown readable, portable, Git-friendly, understandable outside the CMS, and editable in a plain text editor.

Canonical pipeline:

`Markdown Source → Parsing/AST → Validation → Content transforms → Production renderer → Live Preview`.

Preview and production should share as much of the rendering pipeline as possible.

## 1. Product discovery

Identify real content types: articles, news, documentation, long-form, landing content, tutorials, reviews, changelog, FAQ, case studies. For each, capture structure, required metadata, common Markdown constructs, media, tables, code, math, internal links, SEO, CTA, and special blocks.

## 2. User levels

Design one progressively disclosed editor for Beginner, Regular editor, Power user. Beginners need toolbar/help/autocomplete/templates; regular editors need keyboard-first Markdown; power users need shortcuts, command palette, raw source/frontmatter, advanced blocks, and Git-friendly behavior.

## 3. Main editor layout

On wide desktop, prefer header + resizable Markdown editor + Live Preview + compact status. Provide at least `Editor`, `Split`, `Preview` modes. Default to Split when screen width supports it.

## 4. Live Preview

Preview updates automatically and near-immediately after Markdown changes. Never require `Save → Refresh → Open preview` for routine writing.

## 5. Preview stability

Do not jump preview scroll to top on each render. Preserve context with scroll anchoring, section mapping, or block mapping.

## 6. Synchronized scroll

Offer optional `Off`, `Section sync`, `Continuous sync`. Prefer section/block mapping over naïve equal-scroll-percentage syncing.

## 7. Source ↔ preview mapping

Where feasible, selecting a source block highlights its rendered block; clicking/double-clicking preview can navigate to the corresponding source range.

## 8. One document state

Editor and preview must share one canonical document state. Do not create an independent visual document model that can diverge from Markdown.

## 9. Markdown dialect

Explicitly specify the dialect. Prefer CommonMark/GFM base and document support for headings, emphasis, strike, lists, tasks, blockquotes, links, images, tables, fenced code, footnotes, math, callouts, directives, embeds. No implicit extensions.

## 10. Basic toolbar

Keep toolbar compact: Heading, Bold, Italic, Link, Image, Code, Quote, List, Table, More. Do not build a 40-button ribbon.

## 11. Smart toolbar

Make toolbar contextual when helpful: text selection → formatting/link/code; table → row/column/align/delete; image → alt/caption/replace/alignment. Changes still map to readable Markdown.

## 12. Command palette

Provide `Ctrl/Cmd + K` or equivalent for insert image/table/code/callout/embed/internal link, media library, article settings, preview, publish, and other infrequent commands.

## 13. Slash commands

Optional `/image`, `/table`, `/code`, `/callout`, `/video`, `/gallery`, `/quote` should insert transparent, readable Markdown/directives rather than hidden block JSON.

## 14. Natural Markdown shortcuts

Support direct typing of Markdown markers (`#`, `##`, `-`, `1.`, `>`, fenced code). Decorations/live parsing must not interfere with plain text input.

## 15. Text formatting

Support bold, italic, strikethrough, inline code, links; add superscript/subscript only if product requirements justify them.

## 16. Headings

Support H1–H6 technically but define editorial rules, e.g. article title is H1 and body recommends H2–H4. Multiple H1 may warn rather than necessarily block.

## 17. Outline

For long-form content provide an outline from headings. Clicking outline navigates editor and preview to the section.

## 18. Reorder sections

Optional heading/section drag reorder may move the corresponding Markdown source ranges. Only implement if source-range mapping is reliable.

## 19. Links

Selected text + `Ctrl/Cmd + K` should open link entry. Support URL, internal article, anchor, asset.

## 20. Internal links

Search existing content by title rather than requiring copied URLs.

## 21. Stable internal references

Prefer stable internal entity/reference semantics so slug/URL changes can be migrated and backlinks/dependencies remain known.

## 22. Link preview

Hover/select may show title, status, URL; broken links should be identifiable.

## 23. Broken-link detection

Detect missing internal links, invalid anchors, deleted assets contextually before publication.

## 24. Media insertion

Support drag/drop, clipboard paste, file picker, media library, and optionally URL.

## 25. Paste image flow

Optimize `copy screenshot → Ctrl+V → upload → Markdown inserted → preview rendered` to minimal interaction.

## 26. Drag/drop image flow

Validate → upload → optimize → insert Markdown; show inline progress.

## 27. Image optimization

Automate resize, thumbnails, WebP/AVIF, metadata stripping where appropriate. Do not expose encoder complexity to ordinary writers.

## 28. Image UX

Prompt efficiently for alt text and optional caption after insertion.

## 29. Alt text

Warn before publication when required alt is missing. Support explicit decorative-image semantics.

## 30. Image preview in source

Optional inline thumbnail/decorations are acceptable if they do not hide or corrupt raw Markdown.

## 31. Gallery

If galleries are a real use case, define a readable directive syntax such as a fenced/custom directive containing normal Markdown images, not opaque JSON.

## 32. Video

Define explicit, documented, validated, safe embed syntax for supported providers.

## 33. Code blocks

Support syntax highlighting, language autocomplete, copy in preview, wrap option, filename/title, optional line highlighting while keeping fenced Markdown canonical.

## 34. Code metadata

If advanced code meta is needed, specify a consistent grammar such as language + title + highlighted lines; do not create ad-hoc variants.

## 35. Tables

Provide Insert table with row/column size selection that generates GFM Markdown.

## 36. Table editor

When cursor is in a table, provide contextual Add row/column, Delete, Align actions that transform Markdown rather than hidden model state.

## 37. Paste table

Support Excel/Google Sheets paste conversion to a Markdown table, preferably with an explicit `Paste as Markdown table` affordance when ambiguity exists.

## 38. Blockquotes

Use standard Markdown blockquotes. Reserve custom directives for richer informational blocks.

## 39. Callouts

Define a compact controlled syntax for Note/Info/Tip/Warning/Danger, e.g. a directive block, with production-faithful preview.

## 40. Callout insertion

Slash/toolbar selection inserts valid Markdown/directive markup and preserves source transparency.

## 41. Custom components

Before adding a custom component, ask whether standard Markdown can represent the content adequately.

## 42. MDX

Do not introduce MDX automatically. It increases complexity, security surface, parser complexity, and editorial burden. Prefer Markdown + controlled directives unless true component authoring is required.

## 43. Frontmatter

Decide whether YAML is visible. Prefer structured metadata UI for ordinary users with optional raw metadata for power users.

## 44. Structured metadata

Provide Title, Slug, Description, Author, Category, Tags, Cover, Publish date, SEO in a contextual/collapsible panel rather than consuming the writing canvas.

## 45. Title source of truth

Choose exactly one canonical title model: metadata field or in-document H1. Do not maintain two independent titles.

## 46. Slug

Auto-generate, allow edit, validate uniqueness, and propose redirects when published URLs change.

## 47. Description/excerpt

May auto-generate from content but allow manual override.

## 48. Tags

Autocomplete existing tags and normalize taxonomy to avoid semantic duplicates caused by casing/separators.

## 49. Category

Use searchable selection where taxonomy exists.

## 50. SEO panel

Keep SEO outside the main writing canvas: SEO title, meta description, canonical, OG image, indexing.

## 51. Live SEO preview

Show a compact approximate metadata/search/social preview without claiming exact Google rendering.

## 52. Article statuses

Use Draft, Review, Scheduled, Published, Archived only when workflow needs them; simplify for single-author systems.

## 53. Save state

Always expose `Saving…`, `Saved`, `Offline`, `Save failed` near title/status.

## 54. Autosave

Prefer debounced autosave: local editor state → debounce → draft persistence. Do not issue a network write per keystroke.

## 55. Local recovery

Protect work even when backend save fails. Use local cache/IndexedDB or equivalent and show `Recovered unsaved draft` when recovering.

## 56. Conflicts

Use revision/version checks for multi-tab/multi-user edits. Never silently overwrite newer versions; offer Compare/Merge/Keep mine/Reload latest as appropriate.

## 57. Version history

Record author, time, change summary; allow Compare and Restore.

## 58. Markdown diff

Show added/removed/modified changes. For articles, semantic section/block diff can be more useful than raw line diff.

## 59. Undo/redo

Provide normal local editor history (`Ctrl+Z`, redo). CMS version history is not a replacement.

## 60. Article templates

Optional templates for Tutorial, News, Review, Case study, Documentation should produce readable Markdown skeletons.

## 61. Snippets

Allow reusable snippets for disclaimer/CTA/technical note/product table when helpful; generated source remains transparent.

## 62. Word count

Compactly show word count and optionally reading time without making them dominant.

## 63. Reading time

Treat reading time as approximate metadata, not a core KPI.

## 64. Content validation

Validate missing title, broken links, duplicate heading IDs, missing alt, invalid directives, unsupported embeds, malformed tables, invalid frontmatter, unresolved internal refs.

## 65. Validation severity

Use Error, Warning, Suggestion. Reserve blocking errors for genuinely unpublishable states; do not underline the entire document in red.

## 66. Problems panel

For complex articles show a navigable problems list; clicking an issue should move to the source location.

## 67. Spellcheck

Use browser/editor capabilities or integration as appropriate. Do not conflate spellcheck with content validation.

## 68. Grammar

Optional and non-blocking.

## 69. AI assistance

If AI exists, make it selection/context-based: improve, shorten, expand, tone, summary, alt text, title, tags, translate. Do not make a chat panel dominate the editor.

## 70. AI change safety

Never silently rewrite the article. Show Preview/Diff and Accept/Reject for substantive AI edits.

## 71. AI command placement

Put AI in selection menu, command palette, or optional assistant panel—not as 15 permanent toolbar buttons.

## 72. Find

Support `Ctrl/Cmd + F` within article.

## 73. Replace

Support `Ctrl/Cmd + H` for power users when editor engine can do it well.

## 74. Go to heading

Provide command palette heading navigation for long articles.

## 75. Keyboard shortcuts

Respect common `Ctrl/Cmd+B`, `I`, `K`, `Z`, redo, Find. Avoid hijacking browser/system shortcuts without strong reason.

## 76. Focus mode

Offer a distraction-reduced mode hiding metadata/secondary navigation/analytics while preserving editor, save state, and optional preview.

## 77. Zen/writing mode

Optional editor-only centered reading-width mode is useful for long-form writing.

## 78. Source width

Keep Markdown source at a comfortable reading width; allow code/tables to exceed it when needed.

## 79. Preview width

Match the actual production article content width instead of stretching preview to the whole panel.

## 80. Production preview

Use real fonts, components, content styles, code theme, images wherever practical.

## 81. Responsive preview

Offer Desktop/Tablet/Mobile or adjustable preview width. Device bezel mockups are unnecessary; accurate viewport width matters more.

## 82. Preview theme

If public site supports light/dark, preview those production modes independently of CMS UI theme.

## 83. Preview links

Prevent accidental navigation away from editing; e.g. ordinary click selects/inspects, modified click opens link.

## 84. Interactive preview safety

Forms/CTA inside preview must not perform production actions. Mark preview-only interactions.

## 85. Preview render failure

A preview error must not take down the editor. Show a localized render error with source line/block context.

## 86. Split resize

Allow 30/70, 50/50, 70/30 or free resizing; remember user preference.

## 87. Small laptop

When split panes become too narrow, switch to Editor/Preview tabs rather than forcing two unusable columns.

## 88. Mobile

Use Edit/Preview switching on phones; do not use 50/50 split.

## 89. Mobile toolbar

Keep compact, horizontally scrollable/contextual, or place secondary actions under More. Do not consume a quarter of the screen.

## 90. Mobile keyboard

With virtual keyboard open, keep cursor/editor visible and primary actions reachable.

## 91. Tablet

Tablet landscape may support split preview; adapt instead of forcing phone mode.

## 92. Touch

Drag/drop must not be the only way to perform an action. Provide touch alternatives.

## 93. Publish action

Use one clear Publish action; validate before publication. Avoid unnecessary confirmation dialogs when all checks pass.

## 94. Publish check

If no blockers, publish directly or through a compact panel. If warnings exist, show actionable attention items without forcing irrelevant checklists.

## 95. Schedule

Support Publish now / Schedule when required, with explicit timezone.

## 96. Updating live content

For published articles, use `Update` or `Publish changes`, not a confusing “new publish” flow.

## 97. Draft vs live

Clearly distinguish Live version and Draft changes for already-published content.

## 98. Draft preview

Preview must render the current draft, not just live content.

## 99. Open live

Provide `View live` for published articles.

## 100. Content list → editor

`Content → find article → click → editor` should be direct and fast; avoid unnecessary intermediate detail pages.

## 101. Create article

Optimize `New article → title → write`; metadata can follow later.

## 102. Quick create

If title can be entered directly in the editor, avoid a redundant create modal.

## 103. Exit

With reliable autosave, leaving should be frictionless. If save failed and no safe local copy exists, warn explicitly.

## 104. Duplicate article

Duplicate should create a new draft without copying live publication state.

## 105. Copy Markdown

Provide power-user `Copy Markdown` where useful.

## 106. Download Markdown

Allow `.md` download when portability is important.

## 107. Import Markdown

Parse frontmatter, validate dialect, check/localize assets when possible, surface warnings, then open the article.

## 108. Paste Markdown

Large Markdown fragments should paste without corruption.

## 109. Paste rich text

Convert pasted Google Docs/Word/web HTML to clean Markdown while preserving semantic structure.

## 110. Paste sanitization

Strip irrelevant inline styles/tracking spans; preserve headings, links, emphasis, lists, tables, images when possible.

## 111. Raw HTML policy

Explicitly ban or allowlist safe HTML. Never blindly execute arbitrary HTML.

## 112. Security

Audit XSS, iframe, raw HTML, unsafe URLs, directives, SVG, user script. Markdown is not inherently safe.

## 113. External embeds

Use an allowlist of providers such as YouTube/Vimeo if needed. Do not permit arbitrary iframe by default.

## 114. Article AST

Create one parsed AST/intermediate representation reused for preview, validation, TOC, source mapping, SEO/link extraction, and transforms. Do not independently parse Markdown in multiple subsystems.

## 115. Source map

Maintain `AST node ↔ source range` where feasible; this underpins synchronized preview, error highlighting, outline, section navigation, and preview-to-source mapping.

## 116. Incremental parsing

For large documents evaluate incremental parsing; do not necessarily build a custom parser, but avoid expensive full pipelines if they become measurable bottlenecks.

## 117. Preview debounce

Use a short measured debounce, conceptually around 50–150ms, to preserve real-time feel without waste.

## 118. Expensive preview blocks

Do not recreate diagrams/charts/embeds/highlighters on every keystroke if their source block did not change.

## 119. Large-article test

Test around 1k, 5k, 20k words.

## 120. Large code block test

Test substantial fenced code content.

## 121. Many-images test

Test 50–100 images and lazy-load preview media.

## 122. Accessibility

Keyboard-operate toolbar, editor, preview, metadata, outline, dialogs.

## 123. Screen readers

Icon-only actions need accessible names; validation messages must map to their source context.

## 124. Focus restoration

Switching Editor/Preview or closing dialogs should restore focus/cursor logically.

## 125. Contrast

Editor selection, syntax colors, errors must work in light and dark.

## 126. Reduced motion

Respect reduced motion for panels and preview transitions.

## 127. CMS theme vs preview theme

CMS may offer System/Light/Dark; production preview theme is separate. Do not conflate them.

## 128. Code editor theme

Syntax highlighting should harmonize with CMS theme; avoid excessive theme configuration.

## 129. Editor font

Only add Sans/Serif/Monospace selection if writing UX benefits; otherwise choose a strong default.

## 130. Font size

Optional editor-size adjustment can aid accessibility; do not confuse with production typography.

## 131. Status bar

A compact row may show Saved, word count, reading time, warning count, but is optional if state is already clear.

## 132. Context sidebar

A right panel may group Article, SEO, Media, Links, Problems, History. Do not show all simultaneously.

## 133. Panel persistence

Remember panel open state, split size, editor/preview mode; avoid restoring confusing transient state.

## 134. Outline + Problems

These are especially useful long-form contextual views: navigation and quality without clutter.

## 135. Comments/review

If team workflow needs comments, attach them to selection/paragraph/section; do not store review comments inside Markdown source.

## 136. Review mode

Reviewers may use Preview + comments without needing raw Markdown.

## 137. Suggestions

If suggested edits are required, model them explicitly rather than silently replacing the author's text.

## 138. Collaborative editing

Real-time collaboration is a separate capability. Add only when requirements justify CRDT/OT complexity.

## 139. Presence

If collaboration exists, keep presence subtle (who is editing/viewing); do not recreate Figma unnecessarily.

## 140. Offline

At minimum, loss of network must not lose current text.

## 141. Network recovery

Show `Offline — changes saved locally → Reconnecting → Synced` when that reflects reality.

## 142. Analytics

Keep analytics out of writing canvas; show post-publication metrics separately if useful.

## 143. No distractions

Do not show live analytics, unrelated notifications, or CMS vanity metrics while writing.

## 144. Empty article

New document should be calm: Title + `Start writing…`, not twenty empty metadata fields.

## 145. Onboarding

Use brief contextual hints such as `/` for blocks, `Ctrl+K` for links, drag images; make hints disappear after learning.

## 146. Help

Provide a compact Markdown reference, not an embedded manual.

## 147. Unsupported syntax

When importing unsupported constructs, warn rather than silently deleting. Preserve source round-trip where safe.

## 148. Round-trip safety

`open → edit unrelated text → save` must not rewrite/format the whole document unexpectedly and destroy Git diffs.

## 149. Format document

Formatting should be explicit or clearly configured; do not normalize the whole source on every save unexpectedly.

## 150. Source preservation

Preserve intentional blank lines, comments, and safe unsupported syntax where possible.

## 151. Frontmatter preservation

Changing one metadata field should not gratuitously reorder/normalize the entire YAML document if that creates noisy diffs.

## 152. Editor engine evaluation

Compare CodeMirror 6, Monaco, ProseMirror-hybrid, or alternatives by Markdown source editing, source mapping, mobile, bundle size, accessibility, extensibility, performance, IME, collaboration, syntax highlighting. Do not choose Monaco merely because it is powerful.

## 153. Preview security architecture

Do not simply do `markdown → innerHTML` without sanitization and controlled rendering.

## 154. Same renderer goal

Strive for CMS Preview and Production to use the same component/render system.

## 155. Preview differences

If exact fidelity is impossible, make known differences explicit and minimize them.

## 156. Extension model

If extensibility is needed, create a controlled registry for directives, validators, toolbar commands, preview components rather than hardcoding every extension into core.

## 157. Custom directive contract

Each extension should define syntax, parser, AST node, validation, preview renderer, production renderer, editor insertion command.

## 158. Scenario A — New article

`New article → title → write → paste image → continue → live preview → publish` with no mandatory configuration dialogs.

## 159. Scenario B — Long-form

`Open → Outline → jump section → edit → preview remains synced → Problems shows broken link → fix → update`.

## 160. Scenario C — Image

`Copy image → paste → upload → Markdown inserted → preview → alt text`.

## 161. Scenario D — Internal link

`Select phrase → Ctrl+K → type article → choose result → Markdown link inserted → preview updates`.

## 162. Scenario E — Table

`/table → choose size → insert → paste spreadsheet data → edit`.

## 163. Scenario F — Code article

`/code → choose Go → paste source → highlighted preview → optional filename`.

## 164. Scenario G — Publish

`Publish → validate → no blockers → publish → show live URL`. Avoid unnecessary checklist friction.

## 165. Scenario H — Publish with problem

`Publish → blocking issue → click issue → cursor jumps → fix → publish`.

## 166. Scenario I — Mobile quick fix

`Open article → search text → fix typo → preview → update` must be comfortable on phone.

## 167. Scenario J — Recovery

`Write → network disappears → local draft → browser closes → reopen → recovered draft restored/offered`.

## 168. Scenario K — Import

`Import Markdown → metadata detected → assets checked → warnings → article opens`.

## 169. Scenario L — Review

If collaboration is required: reviewer preview → comment paragraph → author editor → comment maps to source → edit → resolve.

## 170. Automated quality tests

Test Markdown parsing, rendering, sanitization, directives, source mapping, internal links, heading IDs, frontmatter, round-trip.

## 171. Golden rendering tests

Maintain representative article fixtures and snapshot expected AST/output.

## 172. Visual regression

Screenshot-regression headings, lists, table, code, images, callout, gallery, quote.

## 173. Editor performance tests

Measure keystroke latency, preview latency, large-file open, large paste, search, scroll.

## 174. Mobile browser tests

At minimum iOS Safari and Android Chrome for web CMS; validate IME, paste, keyboard, selection, toolbar.

## 175. IME

Test composition events for Russian and, where product scope requires, Chinese/Japanese. Avoid aggressive transforms that break composition.

## 176. Paste matrix

Test plain text, Markdown, HTML, Google Docs, Word, Excel, image, code, URL.

## 177. Final UX audit

Run critical workflows keyboard-only, then mouse/touch, then mobile.

## 178. Remove-friction test

For image, link, publish, preview, metadata ask whether one step can be removed safely.

## 179. Remove-toolbar test

Hide toolbar temporarily; power users should still work efficiently through Markdown/keyboard/commands.

## 180. Remove-preview test

Editor-only mode should remain understandable; this tests the quality/readability of the Markdown dialect.

## 181. Source readability test

Open `.md` in a plain editor. If every line is opaque proprietary component syntax, the abstraction is too complex.

## 182. Production fidelity test

Compare Live Preview vs Published article; visual difference should be minimal.

## 183. UX priority

When requirements conflict, prioritize:

**Text safety → Writing speed → Markdown portability → Preview fidelity → Keyboard efficiency → Clear publishing state → Accessibility → Mobile usability → Advanced functionality → Visual decoration**.

## 184. Anti-patterns

Do not build: WYSIWYG that accidentally emits Markdown, 40-icon toolbar, modal per insertion, proprietary block JSON, unnecessary MDX, full iframe reload per keystroke, unclear autosave, save-button-only data safety, two independent titles/content states, hardcoded production styles, permanent preview spinner, re-render of heavy embeds on every keystroke, destructive formatting on save, silent AI rewrite, mobile 50/50 split, mouse-only operations, raw IDs/internal URL copy-paste, or 10-step media insertion.

## 185. MVP

MVP should include Markdown editor, Live Preview, Editor/Split/Preview, autosave, title+metadata, headings, emphasis, lists, links, internal-link picker, image upload/paste, alt text, code blocks, tables, outline, validation, draft/publish, mobile editor, version safety.

## 186. V2

After MVP stability: callouts, advanced embeds, templates, snippets, Problems panel, version diff, scheduling, advanced SEO, rich paste, focus mode, section synchronization, preview→source navigation.

## 187. V3 / advanced

Only with proven demand: collaboration, comments, suggested edits, AI assistance, custom components, extension API, advanced diagrams, complex structured content.

## 188. Required research outputs before coding

Produce:
1. Product requirements.
2. Markdown specification.
3. MVP/V2/optional feature matrix.
4. User journeys.
5. Editor state/parser/AST/source-map architecture.
6. Live Preview fidelity/incremental-update architecture.
7. Media architecture.
8. Metadata/frontmatter architecture.
9. Draft→preview→publish lifecycle.
10. Autosave/conflict/versioning/recovery architecture.
11. Desktop/tablet/mobile UX.
12. Accessibility requirements.
13. Performance budgets.
14. Functional/visual/performance/security test plan.

Then produce a step-by-step implementation plan.

## 189. Existing-editor mode

Do not rewrite immediately. Audit editor engine, Markdown parser, renderer, preview, state model, autosave, backend persistence, media upload, metadata, publishing, mobile.

Create:

`Problem → Current implementation → User impact → Root cause → Proposed solution → Files/modules → Risk → Tests`.

Fix fundamental problems before visual polish.

## 190. Final philosophy

The best Markdown editor does not force a choice between source and visual editor. It makes Markdown itself comfortable while Live Preview instantly shows the production result.

Target loop:

**Write Markdown → see production result instantly → insert complex content without leaving keyboard flow → never lose work → publish confidently.**

Final formula:

**Readable Markdown + Live Production Preview + Source Mapping + Excellent Paste UX + Media Automation + Autosave + Keyboard-first Editing + Safe Publishing**.

The editor should disappear from the writer's attention; the article remains central.
