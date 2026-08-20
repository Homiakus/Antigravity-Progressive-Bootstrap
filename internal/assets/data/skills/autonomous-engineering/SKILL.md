---
name: autonomous-engineering
description: Drives local software engineering through inspect, implement, format, lint, test, build, security checks and final diff review without unnecessary interruptions. Use for implementation, refactoring, debugging, repository repair and engineering automation.
---

# Autonomous Engineering

Work from repository evidence, not assumptions.

## Default cycle

1. inspect the relevant code and configuration;
2. infer local conventions and contracts;
3. plan internally;
4. implement the smallest coherent change;
5. format and static-check;
6. run targeted tests, then broader tests where useful;
7. build/package where applicable;
8. inspect the final diff and regressions;
9. continue fixing failures caused by the change.

Do not stop after producing a plan when the user delegated implementation.
Do not ask whether to run ordinary verification.
Do not use force-push, destructive reset, mass deletion, credential exposure, or unrelated rewrites as shortcuts.
