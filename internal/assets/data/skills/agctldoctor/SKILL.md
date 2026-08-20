---
name: agctldoctor
description: Run agctl full system health check, MCP server live probe, and diagnostic self-test without spaces via /agctldoctor.
---

# agctl Doctor Skill

Run full diagnostic check and MCP live probe on the control plane.

When `/agctldoctor` or `/agctl-doctor` is invoked:
1. Run `agctl doctor --probe-mcp` (or `.\agctl.exe doctor --probe-mcp`).
2. Format the diagnostic report with status badges (`OK`, `WARN`, `FAIL`), details of any findings, and recommended fixes.
