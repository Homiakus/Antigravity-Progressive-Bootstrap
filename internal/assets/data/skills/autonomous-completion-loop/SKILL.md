---
name: autonomous-completion-loop
description: Enforces end-to-end autonomous software development until the user's requested outcome is materially complete and verified. Use for implementation, debugging, refactoring, migrations, feature work, repository audits with fixes, UI work, test repair, CI repair, and other multi-step engineering tasks where the agent should continue through plan-implement-test-diagnose-fix-retest-verify cycles without routine user intervention.
---

# Autonomous Completion Loop

This skill works with Antigravity PreInvocation, PreToolUse, and Stop hooks.

The user delegates the outcome, not individual steps.

## Core contract

Do not stop at:

- a plan;
- a list of recommendations;
- a partial patch;
- the first build error;
- the first failing test;
- "the remaining work is...";
- "you can now...";
- a progress summary;
- an unverified implementation.

Continue until the requested outcome is implemented and verified.

## Definition of Done

At the start of a non-trivial task, infer a concrete internal Definition of Done from:

1. explicit user requirements;
2. existing project architecture/contracts;
3. tests and acceptance criteria;
4. surrounding implementation patterns;
5. documentation/configuration that forms part of the feature.

Do not invent unrelated scope.

Convert vague goals into testable internal checks whenever possible.

## Execution state machine

```text
UNDERSTAND
   ↓
INSPECT
   ↓
PLAN INTERNALLY
   ↓
IMPLEMENT
   ↓
VERIFY
   │
   ├── PASS → REQUIREMENT COVERAGE REVIEW
   │              │
   │              ├── missing → IMPLEMENT
   │              └── complete → REGRESSION GATE
   │                                 │
   │                                 ├── fail → DIAGNOSE
   │                                 └── pass → COMPLETE
   │
   └── FAIL → DIAGNOSE
                 ↓
              FIX
                 ↓
              VERIFY
```

## Failure handling

When something fails:

1. capture the actual error/output;
2. identify the failure class;
3. distinguish symptom from root cause;
4. change the implementation or environment;
5. rerun the narrowest relevant check;
6. then rerun broader verification.

Do not repeatedly execute the same failing action without changing the hypothesis or state.

After two similar failures, explicitly switch strategy internally.

After repeated architecture-level failure, re-inspect assumptions and interfaces.

## Verification ladder

Use the strongest applicable subset:

1. parse/syntax;
2. formatting;
3. type checking;
4. lint/static analysis;
5. targeted unit tests;
6. related test suite;
7. integration tests;
8. build/package;
9. browser/E2E;
10. security/vulnerability check;
11. final diff review;
12. acceptance check against the original request.

For Go, consider:
- `gofmt`;
- `go test`;
- `go vet`;
- `staticcheck`;
- `govulncheck`;
- build/compile where relevant.

For JS/TS, consider:
- formatter;
- TypeScript typecheck;
- lint;
- unit tests;
- build;
- Playwright/E2E for UI.

Do not run irrelevant checks merely to create an impressive list.

## Browser/UI tasks

For user-facing UI changes:

1. reproduce the affected flow when possible;
2. implement;
3. run browser validation;
4. inspect console/network/runtime when failure persists;
5. verify key viewport/state variants relevant to the request.

Prefer Playwright for user flows and Chrome DevTools for runtime/network diagnosis when configured.

## Autonomous questions policy

Do not ask the user about:

- normal implementation choices;
- library-internal decisions that can be inferred;
- routine naming;
- ordinary refactor strategy;
- whether to run tests;
- whether to fix a test failure caused by your change;
- whether to install a clearly required safe dev dependency;
- whether to continue after a recoverable error.

Ask/stop only for a genuine hard blocker such as:

- unavailable secret/API key that cannot be obtained from configured auth;
- external account authorization;
- missing physical device/resource required for validation;
- destructive irreversible product/business decision with no safe default;
- contradictory requirements with materially different outcomes and no inferable priority.

## Completion state

The active PreInvocation hook supplies a task-state path and task id.

Only mark completion when:

- all material requirements are implemented;
- verification actually ran;
- verification evidence is non-empty;
- no known failing checks caused by the task remain;
- final review found no omitted requirement.

Never forge the completion state to escape the Stop hook.

## Progress and user-facing output

Work continuously.

You may emit concise progress updates when useful, but do not turn them into a stopping point.

The final user response should state:

- what was completed;
- important verification performed;
- any real residual limitation.

Do not ask the user to "tell me to continue".
Do not end with a list of obvious next steps that are still part of the delegated task; do them first.

## Parallelism

Use subagents when tasks are separable and parallel work improves throughput, for example:

- research vs implementation;
- independent modules;
- test investigation vs code investigation;
- frontend/backend pieces.

Do not create subagents for tiny tasks or when shared-file conflicts would outweigh the benefit.

## Scope control

Autonomy does not mean scope explosion.

Do not:
- redesign unrelated subsystems;
- upgrade unrelated dependencies;
- rewrite healthy modules;
- change public APIs without need.

Complete the delegated outcome with the smallest coherent set of changes.
