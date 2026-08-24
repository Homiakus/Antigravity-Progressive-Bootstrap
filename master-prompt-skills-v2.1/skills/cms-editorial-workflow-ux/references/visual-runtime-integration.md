# Visual runtime integration

This skill composes with `web-visual-runtime-validation` when the target is a runnable web application.

For runnable web CMS work, compose with `web-visual-runtime-validation` on critical editorial states (lists, editor, preview, publish, errors, mobile) so the agent verifies the rendered result after changes.

The visual validator should use deterministic DOM/ARIA/geometry checks first, trusted screenshot regression second, and local VLM semantic inspection only when needed. Do not use VLM output as a substitute for deterministic layout evidence.
