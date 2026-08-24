# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# CMS UX, Content Operations & Editorial Workflow

Act as a **Senior Product Designer, CMS Architect, UX Researcher, Information Architect, Content Operations Specialist, Design Systems Architect, and Frontend/Backend Product Engineer**.

Deeply audit and redesign `[PROJECT NAME]` so editors, administrators, and content managers can work as quickly, safely, and predictably as possible.

Do not treat this as a visual redesign only. Improve the full content lifecycle:

**find → create → edit → verify → relate → review → publish → update → rollback → analyze**.

Primary quality criterion:

> Minimize actions and cognitive load while maximizing the probability of a correct, safely published result.

The CMS should feel like a professional editorial workspace, not a technical admin panel.

## 1. Main principles

Priority order:

**Clarity → Speed → Safety → Context → Predictability → Power → Visual polish**.

Do not sacrifice working speed for decoration, hide important functionality for minimalism, or turn a professional CMS into a gallery of cards.

Users must always understand where they are, what they edit, whether changes are saved/published, who changed the item, where it is used, how it will look, what publishing will do, and whether they can undo safely.

## 2. Audit the existing CMS first

Map entities such as pages, articles, news, projects, categories, tags, authors, media, SEO, menus, templates, reusable blocks, forms, settings, roles, statuses, versions, and relationships.

For each screen capture:

`User task → current path → action count → problems → error risk → proposed improvement`.

Find duplicates, meaningless fields, technical naming, hidden dependencies, long forms, ambiguous buttons, unconfirmed destructive actions, missing previews, unexplained fields, computable values entered manually, copy/paste workflows, and any need to remember internal IDs or site structure.

## 3. Core workflows

Optimize at least:
1. Create content.
2. Find content.
3. Quick text correction.
4. Replace image.
5. Edit SEO.
6. Link related content.
7. Reorder blocks.
8. Save draft.
9. Send for review.
10. Schedule publication.
11. Publish.
12. Unpublish.
13. Inspect history.
14. Roll back an error.
15. Find outdated content.
16. Bulk edit.
17. Find broken links/missing data.
18. See where an item is used.

## 4. CMS home/workspace

The home screen must answer **What needs my attention now?** rather than present decorative analytics.

Useful groups can include:
- Requires attention: review queue, scheduled items, errors, stale content, missing media, failed publication.
- Recent work.
- Drafts.
- Scheduled.
- Quick actions such as New article, New project, Upload media, Review pending.

## 5. Command center

For professional CMS, add a global command palette, commonly `Ctrl/Cmd + K`, to search/open content, people/settings, create items, navigate to media/SEO, and run common actions.

Search by title, slug, ID, body text, tag, category, author where appropriate. Use fuzzy search.

## 6. Global search

Search must be fast and globally accessible. Group results by entity type such as Pages, Posts, Projects, Media, Categories, Users. Show useful context: title, type, status, updated date, author, location in structure.

Do not force users to remember which CMS section owns the object.

## 7. Content list

Use professional table/dense-list patterns for large content sets, not giant cards. Columns may include title, type, status, author, updated, published, category, language, SEO state. Let users configure visible columns when justified.

## 8. Filters

Provide useful combinable filters such as status, author, category, tag, updated, published, language, errors, unpublished changes, scheduled, SEO issues. Make active filters visible and provide Clear filters.

## 9. Saved views

Power users may save views containing filters, sort, columns, and density, e.g. My drafts, Waiting for review, Publishing this week, SEO problems, Needs update.

## 10. Bulk actions

Support safe bulk publish/unpublish/schedule/archive/category/tag/author/delete where relevant. Before destructive bulk changes, show what will change and how many objects are affected.

## 11. Content editor

Make editing the primary CMS workspace. Avoid forcing users through many tabs. Group by meaning:
- Content.
- Media.
- Metadata.
- SEO.
- Publishing.
- Relations.
- Advanced.

## 12. Progressive disclosure

Do not expose 70 fields at once. Show mandatory, common, and important fields first; move rare options to Advanced without hiding truly important controls.

## 13. Smart fields

Compute values that can be computed: slug from title, update timestamps, current author, reading time, image dimensions, SEO length, etc. Allow override where business needs require it.

## 14. Autocomplete

Use searchable autocomplete for relationships. Never require raw IDs. Search should show enough context to disambiguate.

## 15. Relationship picker

Show title, thumbnail, type, status, slug as appropriate. Exclude already selected items and support keyboard navigation.

## 16. Content blocks

Give blocks human names such as Hero, Image + text, Features, Gallery, FAQ, CTA. Avoid internal implementation names such as `BlockType17`.

## 17. Block insertion

`+ Add block` should open a searchable block palette. Group blocks into meaningful categories such as Content, Media, Navigation, Marketing, Data, Layout and show small previews when useful.

## 18. Drag and drop

Allow reorder where valuable, but provide accessible alternatives such as Move up/down. Do not require a manual save after reorder if safe autosave exists.

## 19. Block duplication

For complex pages support duplicate/copy/paste/convert/save-as-reusable where it saves time.

## 20. Reusable content

For shared CTA/legal/contact/brand blocks, clearly warn that editing a reusable block affects N pages and show where it is used.

## 21. Preview

Preview should be continuously accessible via Edit | Preview or split mode. Support desktop/tablet/mobile views. Use actual production styles, not a rough simulation.

## 22. Live preview

Where architecture allows, update preview near-immediately after edits. Eliminate `Save → Open site → Refresh → Find page` loops.

## 23. Preview unpublished content

Allow secure draft preview URLs without making content public.

## 24. Autosave

Implement reliable autosave with visible `Saving… → Saved` or `Saved 8 sec ago`. On failure show `Could not save`. Do not emit a success toast every few seconds.

## 25. Unsaved changes

If autosave is absent or partial, show `Unsaved changes` and warn before navigation. Never silently lose typed content.

## 26. Draft vs published version

Visually distinguish live and draft. Show when a draft has unpublished changes and allow comparison.

## 27. Diff

History should show what was added, removed, and changed, ideally structurally for block CMS.

## 28. Version history

Record user, date, changes, publication state and offer Restore with clear consequences.

## 29. Undo

Use undo for reversible operations such as deleted block or archived page. Prefer undo over confirmation when the action is safely reversible.

## 30. Publishing validation

Before publish, validate critical conditions such as required fields, links, canonical, image, slug, etc. Show a checklist only when useful.

## 31. Publish panel

Centralize status, visibility, publish date, author, last published, last modified and the main Publish/Update action. If live content has draft changes, make `Update live page` explicit.

## 32. Scheduling

Support date, time, timezone and show a normalized final statement such as `Will publish Aug 25, 2026 at 10:00 Europe/Berlin`. Avoid timezone ambiguity.

## 33. Publication calendar

For high-volume editorial teams, offer a calendar of scheduled, published, drafts, campaigns with direct links to items.

## 34. Workflow

If team needs justify it, support Draft → Ready for review → Approved → Scheduled → Published. Do not impose enterprise workflow on a single-user CMS.

## 35. Review

Allow review handoff with context/comment and preserve who needs to act.

## 36. Comments

Attach comments to content/block context and allow Resolve. Do not turn CMS into a separate chat application.

## 37. Roles and permissions

Use understandable roles such as Administrator, Editor, Author, Reviewer, SEO, Media manager, but implement permissions as capabilities where appropriate.

## 38. Hide irrelevant inaccessible functions

Do not fill the interface with disabled admin controls for users who can never access them. Hide administrative sections where appropriate.

## 39. Audit log

Track important operations: publish, delete, permission/settings changes, restore, bulk actions. Record who/what/when.

## 40. Media library

Provide search, grid/list, filters, upload, drag/drop, multi-select, metadata, replace, and usage tracking.

## 41. Media search

Search filename, title, alt text, caption, tags, upload date, uploader where relevant.

## 42. Image details

Show dimensions, file size, format, alt, title, usage, focal point as relevant. Do not require external file inspection.

## 43. Alt text

Integrate alt text into normal workflow. Warn when required but missing. Support `Mark as decorative`.

## 44. Image processing

Automate resize/WebP/AVIF/thumbnails and hide encoder-level complexity from ordinary editors.

## 45. Replace media

When replacement affects shared usage, explain impact and offer `Replace globally` vs `Upload as new asset`.

## 46. Usage tracking

Every shared entity should expose `Used in`, e.g. `Used in 14 pages`, especially before deletion.

## 47. Safe delete

Show dependent usage and consequences. Never silently destroy referenced content.

## 48. Archive before hard delete

Prefer Archive for content. Keep irreversible delete as a separate destructive action.

## 49. SEO

Integrate SEO into the editor: SEO title, meta description, slug, canonical, OG image, indexing.

## 50. SEO preview

Show search/social metadata previews without claiming exact search-engine rendering.

## 51. SEO validation

Give concrete warnings such as title too long. Avoid meaningless gamified scores like `83/100` when specific problems can be explained.

## 52. Slug

Auto-generate, allow edit, validate uniqueness. When changing a published URL, offer automatic redirect creation.

## 53. Redirects

Help prevent broken URLs, e.g. `Create 301 redirect from /old to /new?`, preferably enabled by default when safe.

## 54. Broken links

Check internal links, missing pages, deleted entities, broken media before publish or in a health center.

## 55. Content health

Provide a `Content Health` view for broken links, missing images/alts, stale content, empty required fields, orphan pages, redirects, failed publication, SEO issues. Each issue should have a direct Fix path.

## 56. Outdated content

Support `Last reviewed` and optional review cadence; surface items needing revalidation.

## 57. Content ownership

For larger sites, assign owner/team such as Marketing, Engineering, Legal to clarify responsibility.

## 58. Navigation management

Use a visual hierarchy for menus. Support drag/drop, nesting, content search, external link, visibility. Do not require manually typing internal URLs.

## 59. Internal links

Pick internal pages by title/reference so URL changes do not break relationships.

## 60. Forms

If CMS manages site forms, create a focused form builder for fields, labels, required state, order, destination. Do not create an unnecessary universal no-code platform.

## 61. Settings architecture

Group settings by meaning: General, Domains, Languages, SEO, Integrations, Users & roles, Appearance, Advanced. Avoid one 100-option page.

## 62. Settings search

For large CMS, allow search by setting name/topic.

## 63. Dangerous settings

Separate destructive/infrastructure settings into a Danger zone. Do not mix them with routine metadata.

## 64. Inline editing

Use inline edit for simple fields such as title, tags, status, category where it genuinely speeds work. Do not turn every table into Excel.

## 65. Quick edit

Allow quick edits to simple properties without loading the full editor.

## 66. Keyboard shortcuts

Support common high-value shortcuts, e.g. save, command palette, preview, publish, but avoid easy accidental destructive shortcuts. Document shortcuts.

## 67. Context menu

Use contextual menus for secondary actions such as Open, Preview, Duplicate, Archive, Copy link. Primary actions must not be hidden exclusively behind them.

## 68. Sidebar

Keep top-level IA stable and concise, e.g. Overview, Content, Media, Navigation, Forms, SEO, Analytics, Users, Settings. Avoid 25 first-level destinations.

## 69. Breadcrumbs

Use breadcrumbs for deep hierarchies so context is always visible.

## 70. Status bar

For complex professional CMS, a small contextual status layer may show Saved, Draft, Last sync, Preview available. Keep it unobtrusive.

## 71. Status colors

Define semantic status mapping such as Draft→neutral, Review→informational, Scheduled→accent/info, Published→success, Failed→danger, Archived→muted. Do not rely on color alone.

## 72. Avoid badge overload

Use badges only where status needs compact scanning. Else use text, icon, or dot.

## 73. Error messages

Errors explain what happened, why if known, and how to fix it. Prefer `Could not publish: SEO title exceeds allowed length` + `Fix` over `Error 422`.

## 74. Validation

Prefer inline validation and avoid waiting until the end of a huge form, but do not show errors before the user has had a chance to interact.

## 75. Required fields

Mark required fields clearly. If nearly all are required, mark exceptions as Optional instead.

## 76. Warning levels

Distinguish Info, Warning, Blocking error. Do not color every issue red.

## 77. Empty states

Explain what will appear, why it matters, and what to do next. Do not leave blank tables.

## 78. Onboarding

Teach through real first actions, sample content, and contextual tips rather than long slideshows.

## 79. Help

Provide contextual help for complex fields without forcing users into external docs for basic meaning.

## 80. Human terminology

Use editorial/product language, not implementation vocabulary. Prefer Related article/Page/Category over Entity relation field/Node/Taxonomy term. Keep internal details in advanced/debug mode.

## 81. Admin debug mode

If developers need IDs, schema names, JSON, API/cache/internal refs, isolate them in advanced/debug mode.

## 82. Multilingual content

Clearly show language states such as RU/EN/DE and whether each translation exists, is missing, outdated, or linked.

## 83. Translation status

Detect when a translation is stale because the source changed after it was last translated.

## 84. Copy from language

Allow copying a source-language structure/content into a new translation instead of rebuilding blocks manually.

## 85. Analytics in context

If useful, show only analytics that support editorial decisions, e.g. Views 30d, conversions, recent traffic. Do not turn the writing screen into an analytics dashboard.

## 86. CMS ↔ frontend links

Provide `View on site`, and where feasible `Edit in CMS` from the authenticated public page.

## 87. Prevent content from breaking the site

Validate critical dependencies before publishing. Block truly unsafe publication with an actionable explanation.

## 88. Staging

Use Draft/Preview/Staging/Production only if architecture and team workflow genuinely need them. Make environment state explicit.

## 89. Publish queue

If publish triggers build/deploy, show real states such as Queued, Building, Deploying, Published, Failed instead of leaving the user after clicking Publish.

## 90. Performance

CMS should feel fast. Use quick navigation, instant command palette, debounced search, pagination/virtualization, background autosave, and safe optimistic UI. Avoid blocking the whole app for local actions.

## 91. Skeleton/loading strategy

Use skeletons for predictable content structure and inline spinners for short local operations. Avoid full-screen loaders for every transition.

## 92. Persistent preferences

Remember useful choices such as last section, table columns, density, filters, theme, sidebar state. Do not persist dangerous temporary selection.

## 93. Mobile

Even desktop-first CMS must support urgent mobile workflows: find content, open, edit text, change status, upload image, preview, publish/unpublish. Replace wide desktop tables with adaptive lists when appropriate.

## 94. Tablet

Treat tablet as a potentially full editorial device. Support touch, virtual keyboard, split preview, drag/drop, and adaptive sidebar.

## 95. Touch targets

Use about 44×44px minimum interactive hit area. Icons may appear smaller but their targets must remain comfortable.

## 96. Theme

Support System/Light/Dark as appropriate without changing semantic status meanings.

## 97. Accent color

If user-selectable accent exists, apply it to selection, primary actions, focus, progress, selected controls. Keep critical/warning/success semantic.

## 98. Motion

Use restrained motion for panels, autosave state, block expansion, drag/drop, status changes, errors. Avoid constantly animated working tables.

## 99. Accessibility

Support keyboard navigation, focus-visible, screen-reader labels, semantic HTML, proper form labels, contrast, reduced motion, error association, and `aria-live` for meaningful async states.

## 100. Information density

For professional CMS, consider Comfortable and Compact density. Comfortable is default; Compact helps large tables. Avoid excessive whitespace that hurts productivity.

## 101. No UI for UI's sake

Before adding a card, panel, badge, icon, modal, or extra surface, ask whether it improves speed or clarity.

## 102. Remove steps

Count actions for important flows. If `find article → edit title → save` takes nine clicks, redesign it.

## 103. Avoid modal hell

Prefer inline editing, side panel, dedicated page, or popover according to task size. Reserve modal for isolated actions and confirmations.

## 104. Context panel

A right-side panel is appropriate for Publishing, SEO, Relations, etc. On small screens convert to drawer.

## 105. Avoid chaotic command duplication

Define one primary location for major actions such as Publish and at most one contextual duplicate where justified.

## 106. Primary action

Each screen should usually have one visually primary action.

## 107. Secondary actions

Preview/Duplicate/Archive should have less weight. Separate destructive actions.

## 108. Content model audit

If CMS is hard to use because schema is broken (`Page`, `Page2`, `LandingPageNew`), repair the content model instead of masking it with UI polish.

## 109. Content schema design

For each type determine required, optional, computed, reusable, referenced, and rarely edited fields. Design forms around those categories.

## 110. Field grouping

Group by user task, not database table or technical grouping.

## 111. Defaults

Use sensible defaults such as Draft status, current author, project timezone. Good defaults reduce actions.

## 112. Smart suggestions

Use quiet contextual suggestions such as adding related content where useful. Do not create a constantly nagging assistant.

## 113. AI functions

AI may draft alt text, summaries, tags, translations, duplicate detection, internal links, or rewrite selected text. Always preview substantive changes before applying.

## 114. No implicit AI autopublish

Do not automatically publish AI-generated content unless the workflow was explicitly configured to do so.

## 115. Content quality assistant

Before publishing, present specific actionable issues such as missing alt or broken link. Avoid meaningless aggregate scores.

## 116. Disaster recovery

Design for accidental deletion, tab close, network failure, edit conflict, failed deployment, corrupt content. Recovery is part of UX.

## 117. Conflict resolution

Do not silently overwrite concurrent edits. Tell users who changed content and offer Compare, Merge, Overwrite, Keep their version as appropriate.

## 118. Real-time collaboration

Add presence/collaborative editing only if truly needed. Do not add multiplayer complexity merely because it is modern.

## 119. Final UX audit scenarios

Test at least:
A. Create and publish a new article.
B. Correct a typo in a live article.
C. Replace an image.
D. Change a live URL.
E. Bulk change category.
F. Recover deleted content.
G. Find content with partial title.
H. Work from mobile.
I. Recover from mistaken publication.
J. Resolve two-editor conflict.

Measure clicks, navigation depth, waiting, error opportunities, and feedback clarity.

## 120. Final UX polish

Review terminology, spacing, typography, focus, icons, statuses, empty/error states, hover, loading, autosave, publishing, preview, mobile. Remove elements that do not contribute.

## 121. Readiness criteria

CMS is ready only when:
- a new user understands the basic workflow without instruction;
- expert users work quickly;
- content is found in seconds;
- relationships use search instead of IDs;
- editing avoids unnecessary context switches;
- preview is directly accessible;
- save/unsaved state is always clear;
- draft and published versions cannot be confused;
- publication consequences are clear;
- URL changes help create redirects;
- version history and restore are accessible;
- dependent deletion is safe;
- media usage is tracked;
- errors explain remediation;
- dangerous actions are separated;
- mobile supports urgent edits;
- keyboard operation is viable;
- users need not understand DB internals;
- UI does not look like a generic admin template;
- professional information density remains appropriate;
- UI does not get in the way of content work.

## 122. Implementation order

1. Domain audit — reconstruct content model and workflows.
2. UX audit — identify friction, redundancy, error-prone paths.
3. Information architecture.
4. Content lists/search/filter/sort/bulk.
5. Editor.
6. Preview.
7. Publishing workflow.
8. History, autosave, undo, recovery.
9. Media library.
10. SEO and content health.
11. Team workflow only if needed.
12. Mobile.
13. Accessibility.
14. Performance.
15. Final visual polish.

## 123. Existing-codebase mode

Do not rewrite an existing CMS just to refresh the UI.

First locate:
- content models;
- CRUD endpoints;
- permission model;
- publication lifecycle;
- preview architecture;
- media subsystem;
- routing;
- autosave/versioning;
- all usage sites of major entities;
- backward-compatibility constraints.

Create a change map:

`Problem → file/module → change → reason → risk → verification`.

Preserve working data and APIs unless there is a strong reason to migrate them.

## 124. Expected result

The CMS should combine:

**professional content editor + IDE-like navigation speed + polished contemporary product UI**.

It should be simple enough for ordinary editors and powerful enough for experts.

Final philosophy:

> The CMS should not make people maintain the CMS. It should disappear behind the work of managing the site.

Core formula:

**Content first + Search everywhere + Context-aware editing + Minimal navigation overhead + Safe publishing + Immediate preview + Autosave + Version history + Clear status + Smart relations + Powerful keyboard workflow + Professional information density.**

Automate what can safely be automated. Keep actions in context when possible. Hide rarely used complexity in Advanced. Explain destructive consequences. Prevent data loss architecturally. If the UI requires users to remember CMS internals, redesign it.
