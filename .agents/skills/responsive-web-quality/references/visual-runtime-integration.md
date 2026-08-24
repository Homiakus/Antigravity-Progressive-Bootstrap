# Visual runtime integration

This skill composes with `web-visual-runtime-validation` when the target is a runnable web application.

Compose with `web-visual-runtime-validation` whenever the app is runnable: use browser geometry/ARIA checks and screenshot evidence to discover actual failure intervals before adding breakpoints.

The visual validator should use deterministic DOM/ARIA/geometry checks first, trusted screenshot regression second, and local VLM semantic inspection only when needed. Do not use VLM output as a substitute for deterministic layout evidence.
