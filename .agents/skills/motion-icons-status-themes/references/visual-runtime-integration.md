# Visual runtime integration

This skill composes with `web-visual-runtime-validation` when the target is a runnable web application.

When motion/theme changes are implemented in a runnable web app, compose with `web-visual-runtime-validation`; capture deterministic theme states and disable animation only for static regression captures, while testing motion separately.

The visual validator should use deterministic DOM/ARIA/geometry checks first, trusted screenshot regression second, and local VLM semantic inspection only when needed. Do not use VLM output as a substitute for deterministic layout evidence.
