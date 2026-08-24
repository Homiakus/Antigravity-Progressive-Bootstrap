# Visual runtime integration

This skill composes with `web-visual-runtime-validation` when the target is a runnable web application.

When a runnable browser implementation exists, compose with `web-visual-runtime-validation` so visual/art-direction decisions are checked against the actual rendered pages, not only source or mockups.

The visual validator should use deterministic DOM/ARIA/geometry checks first, trusted screenshot regression second, and local VLM semantic inspection only when needed. Do not use VLM output as a substitute for deterministic layout evidence.
