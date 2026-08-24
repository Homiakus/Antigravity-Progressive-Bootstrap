# Local VLM routing

## Architecture

Treat the VLM as a replaceable semantic visual analyzer behind a provider interface.

Suggested tiers:
- `fast`: edge VLM for routine visual triage; current MiniCPM-V 4.6-class ~1.3B local inference is a strong candidate.
- `quality`: larger local VLM for dense UI or low-confidence escalation.
- `remote`: disabled by default; optional only by explicit product/privacy policy.

Verify model/runtime compatibility with the target GPU/CPU before pinning a default.

## Latency reduction

- Do not run vision if L0/L1 already prove the defect.
- Use full-view downscale + high-resolution suspect crops.
- Send only changed/suspicious states.
- Reuse model process and warm weights.
- Prefer constrained structured output with a low token budget.
- Avoid asking the VLM to narrate all visible content.

## Grounding

Give the VLM:
- expected user task;
- target viewport/theme;
- annotated element refs;
- compact DOM/ARIA geometry;
- diff boxes;
- changed component names when known.

The VLM should answer only about visible UI. It should not infer implementation details or requirements not provided.

## Validation

Corroborate critical VLM findings with deterministic evidence when possible. If the VLM says two controls overlap but their boxes do not, treat the result as uncertain and inspect the screenshot/crop rather than blindly patching CSS.
