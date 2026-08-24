# Complete visual runtime validation specification

## 1. Goal

Create a closed, progressive visual validation loop for coding agents working on web applications. The loop must inspect the **actual browser-rendered result**, convert it into compact machine-usable evidence, and return enough localization for the main coding agent to fix the source without blindly reasoning from screenshots.

The system must minimize latency and vision-model usage. Use deterministic browser information first, pixel evidence second, local VLM semantics third.

## 2. Reference architecture

```text
Main coding agent / deepseek-harness / Cordis
                 │
                 ▼
        Visual Validation Orchestrator
                 │
      ┌──────────┼───────────┐
      ▼          ▼           ▼
 Playwright   Diff engine   Local VLM
 worker       (native /     adapter
              pixelmatch)
      │          │           │
      └──────┬───┴──────┬────┘
             ▼          ▼
       VisualEvidencePacket
             │
             ▼
        Main coding agent
```

Recommended language split for a Go harness:
- Go: orchestration, budgets, caching, artifact registry, agent/tool contracts, policy.
- Node/TypeScript: Playwright browser worker; Playwright is the authoritative browser automation layer.
- Local inference process: model-specific and replaceable; communicate via local HTTP/OpenAI-compatible/Ollama endpoint when practical.

Do not embed model-specific concerns into the main orchestrator.

## 3. Validation levels

### Level 0 — Browser/runtime health

Collect:
- URL, route, title, viewport, DPR, theme/locale;
- app-ready state;
- page errors and relevant console errors;
- failed critical network requests;
- font readiness;
- `documentElement.scrollWidth/clientWidth` and scroll height;
- screenshot capture metadata.

Failure examples:
- route fails to load;
- hydration/render exception;
- required stylesheet/font missing;
- page-level horizontal overflow;
- app never reaches ready marker.

### Level 1 — DOM/ARIA/layout vectorization

Do not dump full HTML to the agent.

Build a compact node set from:
- visible interactive elements;
- headings/landmarks;
- main layout containers;
- nodes changed by the current patch when known;
- nodes intersecting visual-diff regions;
- ancestors needed to explain clipping/stacking.

Represent each node approximately as:

```json
{
  "ref": "e42",
  "locator": {"role":"button","name":"Login"},
  "bbox": {"x":150,"y":300,"w":112,"h":44},
  "visible": true,
  "clipped": false,
  "style": {
    "display":"flex",
    "position":"relative",
    "zIndex":"auto",
    "overflowX":"visible",
    "overflowY":"visible",
    "opacity":1,
    "fontSize":16,
    "lineHeight":24,
    "color":"rgb(...) ",
    "backgroundColor":"rgb(...)"
  }
}
```

Keep the representation bounded. Prefer role/name/test-id locators over brittle CSS paths.

### Level 1A — Geometry invariants

Implement deterministic checks such as:
- bounding box fully or materially outside intended viewport/container;
- important element clipped by ancestor overflow;
- zero-area interactive element;
- overlapping important rectangles above a configurable intersection ratio;
- sticky/fixed header/footer covering focused/target elements;
- duplicate full-screen overlays;
- modal outside viewport or action row below inaccessible area;
- unexpected document horizontal overflow;
- grid/flex child min-content overflow;
- very small interactive target;
- text container whose scroll dimensions indicate severe clipping.

Avoid naive “any rectangle overlap is an error”: nested elements, badges, icons, overlays, and intentionally layered components are normal. Use semantic roles, DOM ancestry, stacking context, overlap ratio, and allowlists.

### Level 1B — Accessibility tree

Use Playwright ARIA snapshots for semantic compression and regression. Validate critical landmarks and controls by role/name/state.

ARIA snapshots complement but do not replace automated accessibility testing; they are especially useful to the text agent because they summarize the interaction structure without raw DOM noise.

## 4. Visual regression

### Preferred default

When using Playwright Test, prefer:

```ts
await expect(page).toHaveScreenshot('state.webp', {
  animations: 'disabled',
  caret: 'hide',
  maxDiffPixelRatio: ...,
  mask: [...],
  stylePath: ...
});
```

Use element screenshots for isolated component regressions where a full-page baseline would be too noisy.

### Determinism

Stabilize:
- browser version/container image;
- OS/font stack;
- viewport and DPR;
- locale/timezone;
- light/dark/color scheme;
- reduced motion setting;
- fixture data/auth state;
- time via Playwright Clock when relevant;
- font loading;
- dynamic animations/caret;
- third-party/dynamic regions through masks or screenshot-only CSS.

Avoid arbitrary sleeps. Prefer app readiness, locator state, fonts readiness, or deterministic mock completion.

### Direct pixelmatch mode

Use direct pixelmatch if the agent tool needs structured localization beyond Playwright assertions.

Generate:
- expected screenshot;
- actual screenshot;
- diff mask/image;
- total different pixels / ratio;
- optionally windowed diff density;
- region boxes from connected diff pixels or density windows;
- DOM refs whose boxes intersect each significant diff region.

Noise handling must be local: mask unstable regions or use region density. Do not simply raise a global threshold until real defects disappear.

## 5. Baseline governance

A visual baseline is a reviewed expected state, not whatever last rendered.

Rules:
- baseline creation/update must be explicit;
- a failing test must never update its own baseline automatically;
- store baseline identity by stable route/state/component + viewport/theme/browser project;
- do not create unnecessary combinatorial baseline explosion;
- record why a baseline changed in review/commit metadata when the workflow supports it;
- compare baselines only in equivalent rendering environments.

## 6. Local VLM routing

### Fast local tier

Use a lightweight edge-oriented VLM for first semantic inspection. As of 2026, MiniCPM-V 4.6-class ~1.3B models are a strong candidate for this role and are available through local ecosystems such as Ollama. Keep the interface model-neutral.

### Quality local tier

Escalate to a stronger local model when:
- dense small text/UI exceeds the fast model's confidence;
- global composition requires more nuanced reasoning;
- the fast model contradicts deterministic evidence.

### Inputs

Do not repeatedly send only full-resolution screenshots.

Provide:
- full screenshot downscaled for composition;
- suspicious region crops at native/high resolution;
- changed-region masks;
- compact ARIA/layout summary;
- element reference legend;
- expected design intent and current user task.

An optional powerful technique is an **annotated screenshot**: draw numbered boxes around relevant elements and send a mapping `#12 → button "Login"`. This lets the VLM refer to stable IDs that the agent can map back to source and locators.

### Structured output

Require JSON-like findings:

```json
{
  "summary": "...",
  "issues": [
    {
      "type": "overlap|clipping|spacing|hierarchy|contrast|broken_style|responsive|other",
      "severity": "blocker|major|minor|observation",
      "refs": ["e12", "e18"],
      "region": {"x":0,"y":0,"w":0,"h":0},
      "confidence": 0.0,
      "reason": "short evidence-based explanation"
    }
  ]
}
```

The main agent must not treat the VLM's `confidence` as calibrated probability; use it only as a routing heuristic.

### Prompt pattern

Use a task-specific prompt such as:

> Inspect this rendered UI for visible regressions that affect layout or usability. Use the supplied element refs and geometry. Prioritize overlap, clipping, unintended wrapping, offscreen controls, broken spacing/alignment, unreadable text, incorrect responsive state, or obviously broken styles. Do not invent requirements. Return structured issues only. If uncertain, mark low confidence.

For premium visual review, add the intended art-direction criteria from the parent design skill.

## 7. VisualEvidencePacket

The orchestrator should return a bounded evidence object such as:

```json
{
  "state": {
    "route": "/dashboard",
    "viewport": {"width": 390, "height": 844},
    "theme": "dark",
    "scenario": "filters-open"
  },
  "runtime": {
    "pageErrors": [],
    "failedRequests": [],
    "horizontalOverflowPx": 0
  },
  "layoutFindings": [],
  "aria": {"snapshotPath": "..."},
  "visualDiff": {
    "status": "pass|fail|not-run",
    "diffRatio": 0.0,
    "regions": []
  },
  "vlm": {
    "status": "not-run|pass|issues|uncertain",
    "model": "...",
    "issues": []
  },
  "artifacts": {
    "actual": "...",
    "expected": "...",
    "diff": "...",
    "layout": "...",
    "aria": "...",
    "trace": "..."
  }
}
```

Return only the most relevant node details inline; keep large artifacts on disk.

## 8. Issue mapping to source

The browser worker should expose stable selectors and, when possible, component/test identifiers. The coding agent should map visual evidence to source using:
- test ids/component names;
- CSS module/class references;
- React/Vue/Svelte component boundaries if tooling exposes them;
- route/page ownership;
- changed files from the current patch.

Do not ask the VLM to guess source filenames from pixels.

## 9. Progressive checking policy

### Fast inner loop

After a local frontend patch:
1. affected route only;
2. one or two representative viewports;
3. L0/L1 always;
4. L2 if trusted baseline exists;
5. L3 only on meaningful visual changes or ambiguity.

### Final pass

Before declaring the task done:
- full representative viewport matrix from the parent responsive skill;
- critical themes/states;
- existing E2E tests;
- visual baseline suite;
- VLM semantic review for major newly designed surfaces where requested.

## 10. Performance budgets

Track validation overhead by stage. The visual system exists to accelerate the agent, so it must not become a multi-second/minute tax on every tiny edit.

Suggested policy:
- cache static baseline metadata;
- reuse browser context when safe;
- crop screenshots;
- batch style extraction in one browser evaluation;
- avoid one `getComputedStyle` round-trip per node;
- hash relevant assets/build output to skip unchanged visual states;
- run VLM asynchronously only if the harness can continue useful deterministic analysis, but never claim background completion to the user unless the product actually supports it.

## 11. Visual failure taxonomy

Use consistent categories:
- `runtime`;
- `overflow`;
- `overlap`;
- `clipping`;
- `offscreen`;
- `stacking`;
- `responsive_state`;
- `text_wrap`;
- `spacing_alignment`;
- `typography`;
- `contrast_readability`;
- `missing_asset`;
- `broken_style`;
- `unexpected_visual_diff`;
- `semantic_accessibility`;
- `visual_quality`.

## 12. False-positive control

Explicitly handle:
- caret and focus rings;
- scrollbars;
- animations/transitions;
- timestamps/countdowns;
- random/remote images;
- video/canvas/WebGL;
- anti-aliasing and subpixel font differences;
- browser/platform font rendering;
- ads/third-party embeds;
- skeleton/loading states;
- deliberately overlapping design elements.

## 13. Security and privacy

For local/private apps, keep screenshots and DOM summaries local by default. If an external VLM is ever enabled, make it an explicit provider/policy decision and redact/mask sensitive regions where required.

Do not log secrets, auth tokens, password field values, or sensitive personal data in layout evidence.

## 14. Testing the validator itself

Create fixture pages for known defects:
- horizontal overflow;
- clipped button;
- modal below viewport;
- fixed header covering anchor/focus target;
- intentional overlap that must not fail;
- 1px/noise visual diff;
- real dense visual diff;
- responsive breakpoint regression;
- dynamically changing timestamp masked from screenshot;
- VLM low-confidence case.

The validator needs regression tests because it becomes part of the agent's decision-making loop.

## 15. Definition of done

The capability is production-ready when:
- deterministic validation catches seeded layout defects;
- baseline comparison is stable in pinned CI;
- diff regions map back to useful DOM refs;
- local VLM is optional and provider-neutral;
- VLM prompts return bounded structured findings;
- the orchestrator escalates progressively instead of running all stages every time;
- evidence and artifacts are sufficient for the coding agent to make a targeted fix;
- the final loop revalidates after the fix and preserves regression evidence.
