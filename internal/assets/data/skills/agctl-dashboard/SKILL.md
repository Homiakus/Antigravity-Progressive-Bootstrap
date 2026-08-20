---
name: agctl-dashboard
description: Start local observability web dashboard without spaces via /agctl-dashboard or /agctldashboard.
---

# agctl Dashboard Skill

Start local observability web dashboard for Antigravity control plane.

When `/agctl-dashboard` or `/agctldashboard` is invoked:
1. Run `agctl dashboard serve --listen 127.0.0.1:8787` in background/daemon mode (or verify if already running).
2. Present a direct link to the dashboard: `http://127.0.0.1:8787`.
