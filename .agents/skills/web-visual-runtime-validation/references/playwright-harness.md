# Playwright harness implementation guide

## Prefer native Playwright capabilities

Use Playwright Test's `expect(page).toHaveScreenshot()` / locator screenshot assertions for normal visual regression. Playwright waits for two consecutive screenshots to stabilize before comparison and uses pixelmatch under the hood. Configure per-project thresholds sparingly.

Use ARIA snapshot APIs (`page.ariaSnapshot()`, `locator.ariaSnapshot()`, `toMatchAriaSnapshot()`) for compact semantic structure.

## Capture stabilization checklist

Before a visual assertion:
1. enter the exact route/state with locators and explicit readiness;
2. load stable fixture data;
3. ensure required fonts are ready;
4. control time when the UI depends on it (Playwright Clock);
5. disable animations for regression screenshots unless testing motion;
6. hide caret;
7. mask volatile elements or apply `stylePath` screenshot CSS;
8. use a pinned viewport, DPR, locale, timezone, color scheme, and browser project.

Avoid a generic `waitForTimeout(1000)` as readiness logic.

## Batch geometry extraction

Collect multiple nodes in one `page.evaluate`/locator-based pass instead of a browser round trip per property. Bound the node set.

Candidate element selection:
- roles: button, link, input, select, textarea, heading, dialog, navigation, main, complementary, tab, menuitem;
- `[data-testid]` or app-specific stable component attributes;
- top-level layout containers;
- changed/suspected elements supplied by the orchestrator.

## Overlap logic

Do not report parent-child overlap. For unrelated visible nodes compute rectangle intersection ratio and check z-order/positioning semantics. Raise priority when overlap hides an interactive control or readable text.

## Diff localization

With direct pixelmatch:
1. create diff mask;
2. remove tiny isolated noise components below configured area/density;
3. compute connected boxes or windowed diff-density regions;
4. intersect regions with element boxes;
5. report regions + refs.

This converts raw visual differences into actionable agent evidence.

## Trace artifacts

On failure, optionally preserve Playwright trace, screenshot, DOM/layout JSON, ARIA YAML, console/network log, expected image, and diff image. Avoid attaching all artifacts to the agent prompt; provide paths and a short index.
