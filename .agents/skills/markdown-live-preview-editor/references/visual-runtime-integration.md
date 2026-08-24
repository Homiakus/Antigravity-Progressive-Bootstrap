# Visual runtime integration

This skill composes with `web-visual-runtime-validation` when the target is a runnable web application.

For a runnable browser editor, compose with `web-visual-runtime-validation` to test real editor/split/preview rendering, toolbar/virtual-keyboard states, and preview visual regressions.

The visual validator should use deterministic DOM/ARIA/geometry checks first, trusted screenshot regression second, and local VLM semantic inspection only when needed. Do not use VLM output as a substitute for deterministic layout evidence.
