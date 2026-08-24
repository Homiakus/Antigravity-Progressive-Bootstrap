---
name: web-visual-runtime-validation
description: >-
  Use this skill when an agent must verify the real rendered appearance and interaction quality of a web application after frontend changes. It defines a progressive Playwright-based visual validation ladder: deterministic browser/runtime checks, compact DOM/accessibility/computed-style geometry extraction, visual screenshot regression, and selective local VLM inspection with structured evidence. Use it to create fast agentic write-render-inspect-fix loops without sending every screenshot to a cloud vision model.
compatibility: >-
  Agent Skills open standard. Best with a Playwright-capable browser runner and optional local VLM endpoint (for example Ollama/llama.cpp/vLLM-compatible local inference). Pixel diffing can use Playwright's built-in screenshot assertions or direct pixelmatch when custom region analysis is needed.
metadata:
  version: "2.1.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Web Visual Runtime Validation

## Objective

Give the coding agent a fast, evidence-driven visual feedback loop over the **actual rendered application**:

`code change → browser render → deterministic checks → semantic/layout evidence → visual diff → selective local VLM → textual evidence packet → targeted fix → re-test`.

The main agent should receive compact structured evidence rather than raw screenshots whenever a cheaper deterministic layer can answer the question.

## Activation boundary

### Use this skill when
- Frontend/UI changes must be verified in a real browser rather than by reading CSS/DOM alone.
- The user asks for Playwright screenshot inspection, visual regression, pixel diffing, layout/overlap detection, or local VLM validation.
- Another UI skill is implementing or auditing a web app and browser automation is available.
- An autonomous coding harness needs a closed write-render-inspect-fix loop.

### Prefer another skill / do not use when
- The task is only visual ideation with no runnable web app.
- The target is a terminal TUI; use `go-tui-premium-ui`.
- The target is a native host UI that cannot be exercised through the configured browser/Electron harness.
- Do not use VLM output as the sole correctness gate when deterministic DOM/layout or screenshot evidence can verify the issue.

## Inputs to establish

- App start/build command and target URL/route.
- Critical routes, user states, themes, and representative viewports.
- Stable test data or fixtures and authentication strategy.
- Whether visual baselines already exist and where they are stored.
- Available browser environments and local VLM runtime/hardware.
- Expected design intent or reference state when semantic visual review is requested.

If these can be derived from the repository/test configuration, inspect them instead of asking the user to repeat them.

## Progressive validation ladder

Use the **cheapest sufficient level** and escalate only when evidence is ambiguous or the requested quality bar requires it.

### L0 — Runtime sanity

Always run for affected states:
- page/navigation succeeds;
- no relevant uncaught page errors;
- no failed critical requests;
- app reaches an explicit ready condition;
- fonts/assets needed for the view are loaded;
- viewport/document dimensions are captured;
- unexpected page-level horizontal overflow is checked.

### L1 — Semantic + geometry inspection

Extract a compressed representation of visible/interactive/key layout nodes, not raw HTML.

For each useful node capture where available:
- stable element id/ref: role+accessible name, test id, or stable locator;
- bounding box `x,y,width,height`;
- visibility and clipping;
- scroll/client dimensions;
- display/position/overflow;
- z-index / stacking-context hints;
- opacity;
- font size/line height/white-space for text;
- foreground/background colors when useful;
- touch/click target size for interactive controls.

Derive deterministic findings before using vision:
- offscreen/clipped important elements;
- page-level overflow;
- zero-size or invisible interactive controls;
- rectangle overlaps involving important nodes;
- fixed/sticky bars covering focused/target controls;
- unexpected wrap/collapse at the current width;
- suspiciously small interaction targets;
- inaccessible/missing roles/names in the ARIA snapshot.

Use `page.ariaSnapshot()` / `locator.ariaSnapshot()` or `toMatchAriaSnapshot()` for compact semantic structure where appropriate.

### L2 — Visual regression / pixel evidence

When a trusted baseline exists, prefer Playwright `toHaveScreenshot()` for ordinary regression gates. Normalize the environment and use masks/style rules for known dynamic regions.

Use direct pixelmatch only when the harness needs custom behavior such as:
- region-level diff masks;
- connected-component or window-density localization;
- mapping diff regions back to DOM nodes;
- custom baseline storage outside Playwright Test.

Never report only “N pixels differ”. Convert the diff into useful evidence:
- total diff ratio;
- dense diff regions/bounding boxes;
- DOM elements intersecting those regions;
- whether the change is expected, unknown, or clearly broken.

### L3 — Local VLM semantic visual inspection

Run a local VLM selectively when:
- no trustworthy baseline exists and visual composition must be judged;
- deterministic checks pass but the result can still be visually wrong;
- a pixel diff exists but semantic interpretation is unclear;
- typography, hierarchy, spacing rhythm, visual balance, clipping/crowding, or “looks broken” questions require visual understanding.

Default routing should prefer a fast edge VLM. A current candidate is MiniCPM-V 4.6-class local inference; do not hard-code one model into the architecture. Keep a provider/model adapter and verify runtime support on the user's hardware.

Feed the VLM **context, not just a screenshot**:
1. low-resolution full view for global composition;
2. high-resolution crops for suspicious regions;
3. compact layout/ARIA summary with stable element refs;
4. diff regions or changed components when available;
5. the expected user task/design intent.

Ask for structured findings, not prose narration. The VLM should return issue type, affected refs/region, severity, confidence, and concise rationale.

### L4 — Stronger semantic escalation (optional)

Use a slower/larger local or explicitly allowed remote vision model only when L3 confidence is low and the decision materially affects the task. Do not make cloud vision a default dependency.

## Browser determinism rules

Before creating or comparing visual baselines, stabilize the environment:
- pin browser/runtime versions in CI where practical;
- pin viewport, device scale factor, locale, timezone, color scheme, and reduced-motion mode;
- use stable fixtures/test data;
- freeze or control time for time-dependent UI when needed;
- wait for an explicit app-ready/hydration condition rather than arbitrary sleeps;
- wait for required fonts;
- disable animations/caret for screenshot comparisons when animation is not the subject;
- mask or style volatile regions (timestamps, random avatars, third-party embeds) instead of raising global thresholds;
- keep baseline generation and comparison in the same or equivalent rendering environment.

Never auto-accept or regenerate failed baselines merely to make a test green.

## Agent feedback loop

1. Identify the smallest affected route/state/component set from the code change.
2. Build/start or attach to the app.
3. Reproduce the target state with Playwright.
4. Run L0 and L1.
5. If a baseline exists, run L2.
6. Escalate to L3 only when needed.
7. Produce one `VisualEvidencePacket` with deterministic evidence first and VLM interpretation clearly labeled.
8. The main coding agent maps evidence to source files/components and applies the smallest plausible fix.
9. Re-run only affected visual checks for the fast inner loop.
10. After local success, run the broader representative matrix and existing E2E/visual regressions.
11. Preserve failure artifacts for debugging and review.

Do not run the VLM on every keystroke, frame, or hot-reload event. Trigger visual validation at meaningful checkpoints after layout-affecting changes or explicit agent decisions.

## Evidence hierarchy

Prefer:
1. DOM/layout invariant violation with exact element refs and geometry.
2. ARIA/semantic mismatch.
3. Trusted pixel/screenshot regression with localized diff.
4. Local VLM finding corroborated by layout/diff evidence.
5. Local VLM-only aesthetic finding.

Never present level 5 as deterministic proof.

## Progressive viewport strategy

For the fast inner loop, test the smallest representative set that brackets the changed component's failure modes. Typical examples:
- compact phone portrait;
- tablet/narrow split view;
- small laptop/desktop;
- wide desktop when the component uses expanded layouts.

When a failure is detected, sweep around that width to find the natural breakpoint/failure interval. Do not create a giant device matrix for every inner-loop change.

The final regression pass should use the parent UI skill's full responsive/functional matrix.

## Integration contract

Expose the visual layer to the main harness through tool-like operations such as:
- `visual.capture(state)`
- `visual.inspect_layout(state)`
- `visual.compare_baseline(state)`
- `visual.inspect_vlm(state, evidence)`
- `visual.validate(state, mode="auto")`

The exact transport may be MCP, JSON-RPC, stdio, HTTP, or in-process APIs. Keep the contract structured and transport-neutral.

For Go-based orchestrators, a practical architecture is:
- Go orchestrator/harness owns workflow, budgets, retries, artifacts, and agent messages;
- Node/TypeScript Playwright worker owns browser automation and screenshot/DOM extraction;
- local VLM worker exposes a small local HTTP/OpenAI-compatible/Ollama-style endpoint;
- the main agent receives only the evidence packet and artifact paths unless it explicitly needs the image.

## Failure budgets and escalation

Set per-run budgets so the validation loop does not become the bottleneck:
- deterministic L0/L1: run by default;
- L2: run when baseline is available or regression safety is requested;
- L3: run on changed visual surfaces, suspicious deterministic results, or explicit design review;
- L4: rare escalation only.

Cache/reuse unchanged evidence by route/state/build hash where safe, but invalidate it on relevant CSS/component/theme/layout changes.

## Completion gates

- The agent has validated the real rendered state, not only source code.
- Deterministic issues are reported with element refs and geometry.
- Visual-diff failures include localized evidence and artifacts.
- VLM use is selective, structured, and clearly marked as semantic interpretation.
- Failed baselines are never silently updated.
- Dynamic content and rendering environment are controlled enough for meaningful comparison.
- Critical responsive states are checked after the local fix.
- A final report distinguishes confirmed defects, expected visual changes, and uncertain aesthetic observations.

## Progressive-disclosure routing

- `references/full-spec.md` — Read for the complete architecture, algorithms, detector list, and closed-loop behavior.
- `references/playwright-harness.md` — Read when implementing browser capture, ARIA/layout extraction, screenshot determinism, or diff localization.
- `references/vlm-routing.md` — Read when implementing local VLM selection, prompts, crops, confidence, and escalation.
- `references/tool-contract.md` — Read when exposing this capability to Cordis/deepseek-harness/another orchestrator.
- `references/output-contract.md` — Read before producing a visual audit/report or implementation plan.
- `references/evals.md` — Read only when evaluating or tuning this skill.
- `references/sources.md` — Read for current upstream documentation and version-sensitive claims.

## Skill composition

This skill is commonly composed with:
- `responsive-web-quality` for viewport/capability matrices and responsive root-cause repair;
- `premium-editorial-ui` for high-end visual/art-direction review;
- `motion-icons-status-themes` for state/motion/theme verification;
- `cms-editorial-workflow-ux` for real CMS screen workflows;
- `markdown-live-preview-editor` for authoring/preview surfaces.
