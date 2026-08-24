---
name: deep-codebase-integrity-audit
description: >-
  Use this skill for an evidence-driven system-level codebase audit that must find multiple sources of truth, override/default drift, magic values, duplicated state/logic/contracts, identifier/config/state collisions, overengineering, unnecessary abstractions, allocations, algorithmic complexity, cache/retry/concurrency/resource ownership risks, and then produce an ordered file-level remediation plan with tests, benchmarks, migration steps, and definitions of done. Use it beyond ordinary code review.
compatibility: >-
  Agent Skills open standard. Requires repository access for full value; performance conclusions should use available build, test, profiler, race/static-analysis, and benchmark tooling rather than guesswork.
metadata:
  version: "2.0.0"
  standard: "agentskills.io"
  architecture: "progressive-disclosure"
---

# Deep Codebase Integrity Audit

## Objective

Turn a complex codebase into an evidence-backed map of ownership, consistency, complexity, and technical debt, then convert findings into safe, atomic remediation work.

## Activation boundary

### Use this skill when
- The user explicitly asks for deep architecture/performance/reliability audit rather than normal review.
- The project may have duplicated configuration/defaults/state/contracts, magic numbers, hidden overrides, or multiple implementations.
- The task asks to detect overengineering, allocations, complexity, collisions, races, retry/cache issues, and produce a detailed fix plan.

### Prefer another skill / do not use when
- Do not claim a bottleneck without measurement or at least a clearly marked hypothesis plus verification step.
- Do not centralize everything into a global singleton in the name of SSOT.
- Do not rewrite the system before characterization tests and dependency/ownership mapping.

## Inputs to establish

- Repository and build/test instructions
- Runtime/workload assumptions
- Known incidents or hot paths if available
- Supported platforms
- Performance/reliability constraints

If repository/screenshots/runtime evidence are available, inspect them before asking the user to restate information that can be derived. If an input is genuinely unavailable, state the assumption rather than inventing evidence.

## Core workflow

1. Start read-only: map entry points, modules, dependencies, data flow, state ownership, configuration, generated code, tests, and runtime boundaries.
2. Build concept-centric traces: canonical/derived/cached/snapshot/override/duplicate sources, all writers/readers, default/override precedence, schemas, validators, identifiers, normalization, and units.
3. Search for semantic duplication and collisions, not only textual duplicates: status rules, retries/timeouts, cache keys, paths, state transitions, permissions, contracts, and legacy/new parallel paths.
4. Audit overengineering by tracing real variability and responsibilities through interfaces/wrappers/factories/events/state machines; distinguish useful boundaries from pass-through layers.
5. Audit algorithms and allocations only with expected N/workloads. Use call graphs, benchmarks, profiles, escape analysis, DB/file/network traces, and race/concurrency tools when available.
6. Classify each finding by evidence, priority, confidence, blast radius, root cause, fix cost, and regression risk. Separate confirmed problems from hypotheses.
7. Design target ownership/contracts and an incremental migration graph. Add characterization/regression tests before risky refactors; migrate consumers; remove legacy paths explicitly.
8. Produce atomic tasks with files/functions, exact changes, dependencies, tests/benchmarks/failure injection, rollback notes, and measurable definition of done; then specify a re-audit gate.

## Evidence and decision rules

- Separate **observed fact**, **inference**, and **recommendation**.
- Prefer the smallest change that solves the demonstrated problem.
- Preserve working business/domain behavior unless the requested UX/architecture change explicitly requires a behavior change.
- Do not claim verification that was not actually performed.
- For version-sensitive libraries, APIs, standards, browser/host behavior, or framework capabilities, verify current documentation when external access is available.
- When the task is implementation rather than advice, map recommendations to concrete components/files/contracts and include tests.
- Do not load every reference automatically. Use the routing section below to keep context focused.

## Progressive-disclosure routing

- `references/full-spec.md` — Read for the complete original master specification and edge-case requirements.
- `references/practice-upgrades.md` — Read when making architecture/implementation decisions or setting the final quality bar.
- `references/output-contract.md` — Read before producing a formal audit, specification, or implementation plan.
- `references/evals.md` — Read only when evaluating or tuning this skill itself.
- `references/sources.md` — Read when a recommendation depends on current external standards, APIs, browser/host behavior, or security guidance.

## Completion gates

- Every finding cites concrete code/runtime evidence or is explicitly labeled as requiring measurement.
- SSOT recommendations define ownership/invalidation without creating a god object.
- Performance claims include expected N and measurement strategy; optimizations preserve correctness.
- Refactors are dependency-ordered, incremental, testable, and include removal of old paths.
- The final plan is executable without repeating the entire architectural investigation.

Before finalizing a substantial deliverable, read `references/output-contract.md`. For deep edge cases or rules not covered above, load only the relevant sections of `references/full-spec.md` and `references/practice-upgrades.md`.

## Skill maintenance

`references/full-spec.md` preserves the complete original master prompt. Do not delete rules from it during maintenance merely to shorten `SKILL.md`; move or consolidate detail through progressive disclosure instead.

`references/evals.md` contains activation and output-quality cases. Use it when changing this skill's `description`, activation boundary, workflow, or outputs.
