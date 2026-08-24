# Tool contract for Cordis / deepseek-harness / other orchestrators

## Recommended operations

### `visual.validate`
Runs progressive auto mode.

Input:
```json
{
  "url": "http://127.0.0.1:3000/dashboard",
  "scenario": "filters-open",
  "viewport": {"width": 390, "height": 844},
  "theme": "dark",
  "baseline": "auto",
  "vlm": "auto",
  "changedFiles": ["src/components/Filters.tsx"]
}
```

Output: `VisualEvidencePacket` described in `full-spec.md`.

### `visual.inspect_layout`
Returns bounded layout/ARIA evidence and deterministic violations.

### `visual.compare_baseline`
Returns screenshot/diff artifacts and localized diff regions.

### `visual.inspect_vlm`
Accepts screenshot/crops + bounded layout evidence and returns structured semantic findings.

### `visual.capture`
Captures normalized screenshot and state metadata without comparison.

## Orchestration rules

- The Go/main harness owns escalation policy and budgets.
- The Playwright worker must not call the coding model directly.
- The VLM worker must not mutate code or browser state.
- Every result includes a state identity (route/scenario/viewport/theme/build hash).
- Tool calls should be idempotent for the same stable state where practical.
- Artifacts are stored out-of-band and referenced by path/URI.
- Secrets and password values are never included in evidence packets.

## Suggested auto policy

```text
visual.validate(auto)
  -> L0 runtime
  -> L1 layout/ARIA
  -> if deterministic blocker: return immediately
  -> if baseline exists: L2 diff
  -> if diff semantic meaning is obvious: return
  -> if changed UI/design surface or ambiguous: L3 fast local VLM
  -> if low confidence + material: optional quality VLM
```

For development loops, re-run only failed/affected states after a patch; run the full visual suite at the completion gate.
