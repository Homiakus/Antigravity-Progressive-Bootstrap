# World-class practice upgrades

## Document architecture
- Treat Markdown plus structured metadata as canonical; use parser/AST/source ranges as shared representation for preview, outline, diagnostics, links, and transforms.
- Preserve round trips so unrelated edits do not rewrite formatting/frontmatter/comments.

## Editor engine choice
- Choose the engine from Markdown ergonomics, mobile/IME, accessibility, extension model, bundle size, collaboration, source maps, and performance.
- Verify current maintenance/version before selecting CodeMirror, Monaco, ProseMirror hybrids, or alternatives.

## Rendering and security
- Reuse production rendering components/style when possible; otherwise define a versioned preview-rendering contract.
- Sanitize untrusted HTML, URL schemes, SVG, and embeds per threat model; prefer allowlisted providers.
- Preview forms/widgets must not trigger production side effects.
- Add XSS/malformed-directive regression tests.

## Persistence and authoring
- Autosave with revisions and local recovery; never silently last-write-wins valuable content.
- Treat paste as first-class: plain/Markdown/rich HTML/Docs/Word/spreadsheets/images/code/URLs.
- Keep upload progress/failures inline and internal links tied to stable content identity when possible.

## Performance/accessibility
- Debounce preview, reuse unchanged heavy blocks, lazy-load offscreen media, and benchmark long documents/many images/code.
- Test IME, undo/redo, mobile virtual keyboard, keyboard-only toolbar/outline/problems, and renderer-failure isolation.
