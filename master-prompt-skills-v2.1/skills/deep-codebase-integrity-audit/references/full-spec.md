# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Deep Codebase Consistency, Complexity & Overengineering Audit

Act as a **Principal Software Architect, Staff/Principal Engineer, Performance Engineer, Reliability Engineer, Static Analysis Engineer, Refactoring Specialist, and algorithm/memory/failure-analysis expert**.

Perform a system-level audit of `[PROJECT NAME]`, then produce an evidence-based, ordered remediation plan at the level of concrete packages/files/functions/contracts.

This is not a normal code review. Audit the system as:

**sources of truth → configuration → defaults → override chains → duplicated state → duplicated logic → contracts → constants → magic numbers → caches → state ownership → data flow → algorithms → allocations → concurrency → complexity → abstractions → overengineering → collisions → failure modes → tests**.

Primary goal:

> Find where one concept is defined multiple times, can hold multiple values simultaneously, is overridden implicitly, copied across layers, hidden behind defaults, or implemented with substantially more complexity than the domain requires.

After diagnosis, produce one detailed remediation roadmap that another engineer/agent can execute sequentially without redoing the architectural investigation.

## 0. Audit rule

Do not label code bad merely because it is complex, an abstraction overengineered merely because it has layers, every numeric literal a magic number, or every allocation a performance problem.

Every finding must include:
`Evidence → Impact → Root cause → Risk → Recommended fix → Verification`.

Classify findings as `Confirmed problem`, `Likely problem`, `Potential risk`, or `Intentional design`. Never present speculation as proven defect.

## 1. Reconstruct architecture first

Before recommending changes, map entry points, packages/modules, dependency graph, domain, services, persistence, config, API, UI, background workers, queues, caches, adapters, integrations, generated code, tests, mocks, tooling.

Trace important data lifecycles from input/configuration through domain/services to persistence/external systems and presentation.

## 2. Read-only diagnosis before refactor

Do not change code until architecture map, primary sources of truth, override chains, high-risk paths, and confirmed findings are known.

## 3. Source-of-truth audit

For each important business concept identify the canonical source, actual sources, writers, readers, risk. Examples include configuration, current user, project/task state, model catalog, feature flags, connection status, device config, limits, paths, timeouts, pricing, permissions, schema, UI preferences, runtime state.

## 4. Multiple sources of truth

Find one concept represented simultaneously in config, env, CLI, DB, UI local state, cache, hardcoded defaults, etc. Example: timeout 30s in client, 60s in worker, config value in UI.

## 5. Source classification

Classify each copy as Canonical, Derived, Cached, Snapshot, Override, or Duplicate. The goal is explicit ownership/invalidation, not eliminating every physical copy.

## 6. Duplicated state

Find the same data in backend model, service, cache, frontend store, component state, DB, config. Ask who owns it, who can mutate it, how updates propagate, how invalidation works, and what partial failure does.

## 7. State divergence

Identify paths where A updates but B does not, especially cache vs DB, UI vs server, config vs runtime, file vs memory, worker vs orchestrator.

## 8. Override audit

Find all override mechanisms such as `default → config file → env → CLI → DB → runtime → user preference` and reconstruct actual precedence from code.

## 9. Override chains

For each critical setting document exact precedence, including compiled default, system/project config, environment, CLI, runtime override.

## 10. Multiple override implementations

Find separate resolvers (e.g. `config.Load`, server settings resolver, worker resolver, UI resolver) that implement slightly different precedence rules.

## 11. Hidden overrides

Find fallbacks like `if value == 0`, empty-string fallback, constructor overriding caller values, middleware mutating options, and implicit defaults below configuration boundaries.

## 12. Zero-value collisions

Especially in Go, distinguish `0/false/""` as intentional value vs “not configured”. Find ambiguous zero-value semantics.

## 13. Default-value inventory

Inventory defaults across constructors, config loaders, env parsing, UI, CLI, DB migrations, schema defaults, fallback branches. One policy should not silently have many defaults.

## 14. Default drift

Detect frontend/backend/docs/default discrepancies and prioritize them.

## 15. Configuration ownership

For each setting identify Definition, Default, Validation, Serialization, Override precedence, Runtime representation, Documentation. Avoid scattering one policy across the system.

## 16. Magic-number/value audit

Find numeric/string literals with hidden domain meaning, such as timeouts, limits, thresholds, port/protocol values, states, paths. Do not mechanically classify every literal as magic.

## 17. Literal classification discipline

Ordinary loop/math literals such as `i+1`, `len==0`, percent division are often fine. A magic value is non-obvious, repeated, policy-bearing, mutable, or correctness-sensitive.

## 18. Magic-value classification

Classify each as Domain constant, Protocol constant, Configuration, Tuning parameter, Algorithm constant, or Accidental literal.

## 19. Stringly-typed logic

Search repeated status/mode/role strings such as `running`, `done`, `failed`, `admin`, `prod`; evaluate enum/typed constant/value object.

## 20. Duplicated constants

Repeated identical values may or may not represent one policy. Determine semantic ownership before consolidation.

## 21. Unit audit

Find naked values with unclear units, especially durations and sizes. Prefer typed units (`time.Duration`, domain types) where appropriate.

## 22. Time audit

Check ms vs s, duration parsing, Unix seconds vs milliseconds, timezone, wall vs monotonic time.

## 23. Size audit

Check bytes vs KB/KiB/MB/MiB conversions and naming.

## 24. Multiple representations

Find concepts represented incompatibly across layers, e.g. status string/int/enum/bool; every conversion is a collision surface.

## 25. Domain duplication

Find duplicated validators, parsers, normalization, calculations, state transitions, permission checks, retry policies, sort/filter, serialization, including semantic rather than textual duplicates.

## 26. Semantic duplication

Detect logically identical policies expressed differently (`age >= 18` vs `age > 17`).

## 27. Contract audit

Map interfaces, API schemas, DB schemas, event/message formats, config schemas, pre/postconditions; find contracts described multiple times.

## 28. Schema duplication

Compare Go structs, TypeScript interfaces, JSON Schema, DB migrations, OpenAPI, form validation. Identify canonical schema and generated vs handwritten duplicates.

## 29. Validation duplication

Distinguish UX validation from authoritative business validation. Find frontend/backend rules that disagree.

## 30. Collision audit classes

Audit Name collisions, Identifier collisions, Configuration collisions, State collisions, Resource collisions (files/ports/sockets/DB/cache keys).

## 31. Identifier audit

Review ID/cache-key/filename/temp/job/request ID generation and collision probability/handling.

## 32. Hash collisions

If hashes are used as unique IDs, analyze algorithm, truncation, cardinality, and collision handling.

## 33. Cache-key collisions

Find ambiguous concatenation/encoding such as `userID + path` without separators/length encoding.

## 34. File collisions

Audit temp/generated names, concurrent writes, atomic rename, platform-specific filesystem semantics.

## 35. Concurrent state collisions

For each shared mutable state identify writers, synchronization, ordering, atomicity.

## 36. Lost updates

Find concurrent read-modify-write without versioning/locking/compare-and-swap as required.

## 37. Check-then-act races

Find `if absent → create` or equivalent where state can change between check/action.

## 38. TOCTOU

Audit filesystem, permissions, resources, and external-state time-of-check/time-of-use races.

## 39. Idempotency

For retries, jobs, migrations, external requests, event processing, determine whether duplicate execution is safe.

## 40. Retry collisions

Find retries that duplicate writes/jobs/messages/resource charges.

## 41. Overengineering audit

Evaluate the minimum architecture needed for current requirements and realistic near-term scale.

## 42. Abstraction cost

For each abstraction ask what problem it solves, implementations/consumers count, whether variability is real, and cognitive cost.

## 43. Interface with one implementation

Do not automatically remove it. Check whether it provides a real boundary/mock seam/future implementation; otherwise flag as possibly premature.

## 44. Interface chains

Find pass-through chains such as Handler → Service interface → ServiceImpl → Repository interface → RepoImpl → Adapter.

## 45. Pass-through layers

If a layer does not validate, transform, own lifecycle, define policy, or provide a boundary, evaluate removal/merge.

## 46. Wrapper explosion

Find manager/service/repository/provider/resolver chains around trivial operations such as reading one config file.

## 47. Factory overuse

Factories/builders/providers need actual variability/lifecycle value; simple constructors may be enough.

## 48. Generic abstraction

Find generalized frameworks used for one fixed scenario and measure their cognitive/runtime cost.

## 49. Premature extensibility

Look for plugin systems without plugins, event buses for local calls, workflow engines for a linear sequence, DI frameworks for small apps, distributed abstractions in single-process programs.

## 50. Event-bus audit

Map producer → event → subscribers. If an event exists solely to avoid one direct call, evaluate the benefit.

## 51. Callback/channel maze

Trace callback → channel → event → goroutine → callback chains that obscure simple control flow.

## 52. State-machine justification

FSM is useful for real lifecycle complexity; avoid framework-level FSM for trivial idle/done states.

## 53. Configurability overengineering

Find settings nobody changes, with one practical value, or exposing implementation details. Removing a setting may be better than UI polish.

## 54. Dead flexibility

Find speculative “future provider/backend/mode” with no consumer. Apply YAGNI as a signal, not dogma.

## 55. Dead code

Find unreachable code, unused functions/fields, deprecated paths, stale migrations/flags, abandoned compatibility layers.

## 56. Shadow implementations

Find old/new/V2/legacy versions of the same feature and determine which path is actually used.

## 57. Feature-flag debt

For each flag determine age, users, default, removal opportunity. Long-lived flags multiply behavior sources.

## 58. Allocation audit

Analyze memory allocations only after identifying hot paths.

## 59. Allocation inventory

In hot loops inspect repeated slice/map allocation, string concatenation, `fmt.Sprintf`, `[]byte↔string`, reflection, interface boxing, closures, heap escapes, temporary structs, JSON encode/decode, regex compilation.

## 60. Escape analysis

For Go, use `go build -gcflags=-m` or appropriate tooling. Distinguish stack vs heap and do not chase every escape blindly.

## 61. Benchmark first

Performance-critical changes require baseline benchmark, change, and post-change comparison.

## 62. Allocation metrics

For hot functions capture `ns/op`, `B/op`, `allocs/op`.

## 63. Object reuse

Evaluate preallocation, buffers, `sync.Pool`; do not use `sync.Pool` by default because it adds complexity and can hurt.

## 64. Slice preallocation

When capacity is known/estimable and the path is hot, `make([]T,0,n)` may help.

## 65. Map preallocation

Apply the same evidence-based rule for maps.

## 66. String building

Find repeated concatenation in loops; evaluate `strings.Builder`, `bytes.Buffer`, or direct writes.

## 67. Serialization cost

Find repeated serialize/deserialize cycles between layers in the same process.

## 68. Copy audit

Find unnecessary copying of large structs/slices; evaluate value vs pointer semantics.

## 69. Pointer overuse

Too many pointers can increase heap churn, complexity, poor locality, nil risk. Do not convert everything to pointers.

## 70. Algorithm complexity audit

For performance-sensitive algorithms record Time complexity, Space complexity, expected N, worst-case N.

## 71. Beyond Big-O

Also analyze real N, constant factors, allocation cost, I/O, cache locality, concurrency. `O(n)` alone is insufficient.

## 72. Nested loops

Do not falsely call every nested loop `O(n²)` if an inner bound is constant. Establish input relationships.

## 73. Hidden quadratic behavior

Find loop + linear search, append + scanning dedupe, repeated scans over growing structures, and similar semantic `O(n²)` patterns.

## 74. Sort inside loop

Find repeated sorting that could become one `O(n log n)` sort rather than repeated higher-order work.

## 75. N+1 database queries

Detect query-per-item loops and evaluate batching/prefetching.

## 76. Repeated filesystem I/O

Find file read/stat/scan per item when batching/indexing would be better.

## 77. Repeated network I/O

Likewise identify request-per-item patterns and batching/parallelism limits.

## 78. Unnecessary full scans

Find full-tree/full-file scans triggered by local changes; evaluate incremental approaches.

## 79. Index audit

For frequent lookups evaluate map/index/cache/DB index, including invalidation/maintenance cost.

## 80. Data-structure audit

Evaluate slice/map/set/heap/tree/queue/ring buffer against actual operations and cardinalities.

## 81. Hash-map overuse

For tiny bounded sets, a slice may outperform/simplify a map. Use real sizes/benchmarks.

## 82. Sorted data

For range queries, sorted structures may outperform hash maps; measure.

## 83. Lock complexity

Map contention, critical sections, lock ordering.

## 84. Lock inside loop

Find high-frequency lock/unlock that may be batchable.

## 85. Lock + I/O

Flag holding mutexes across network/database/file I/O unless explicitly justified.

## 86. Goroutine audit

For Go, locate goroutine creation, ownership, cancellation, termination, leaks, fan-out.

## 87. Unbounded goroutines

Find `for item { go process(item) }` with large/unbounded N and no concurrency limit.

## 88. Channel audit

Review ownership, close semantics, buffer sizing, deadlock, blocked senders/receivers.

## 89. Channel as durable queue

Challenge assumptions when in-memory channels are treated as reliable queues.

## 90. Context audit

Check propagation, cancellation, timeouts, `context.Background()` inserted mid-request, ignored cancellation.

## 91. Retry complexity

Find nested retry layers. Example 3×3×5 policies can produce 45 attempts. This is a classic multiple-policy defect.

## 92. Timeout stacking

Likewise trace time-budget composition across layers and conflicting timeouts.

## 93. Cache audit

For each cache answer: what is cached, canonical source, TTL, invalidation, memory bound, concurrency, failure behavior.

## 94. Cache as accidental source of truth

Identify caches that gradually become authoritative without explicit design.

## 95. Unbounded cache

Find memory growth without eviction/limits.

## 96. Duplicated cache

Find multiple caches for the same entity and justify each layer/invalidation path.

## 97. Error-handling audit

Find swallowed errors, excessive wrapping, string matching, inconsistent classification, retries on permanent errors.

## 98. Error source of truth

Compare error code, HTTP status, string, UI state and ensure consistent mapping.

## 99. Logging audit

Find the same error logged at repository/service/handler creating duplicates. Define logging ownership.

## 100. Metrics duplication

Ensure the same operation is not counted multiple times across layers.

## 101. Resource ownership

For files, connections, transactions, goroutines, timers, tickers, subscriptions, observers identify owner/lifecycle.

## 102. Cleanup

Find missing Close/Cancel, ticker leaks, goroutine leaks, event-handler leaks.

## 103. Transaction audit

Review DB transaction boundaries, nested assumptions, partial writes, isolation requirements.

## 104. Partial failure

For multi-step A→B→C, determine consistent state if C fails after A/B succeed.

## 105. Atomicity strategy

Choose transaction, compensation, idempotency, or recovery according to the operation.

## 106. File write atomicity

Where corruption matters, prefer temp write → flush/fsync as needed → atomic rename.

## 107. Test source of truth

Find fixtures/constants that duplicate production rules. Tests should not independently repeat the same flawed policy so both pass incorrectly.

## 108. Mock audit

Inventory mocks/stubs/fakes and verify that behavior/contracts still match production.

## 109. Mock drift

Pay special attention to API/storage/network fakes that permit impossible production states.

## 110. Test-only implementations

Find architecture distorted by implementations used only in tests; evaluate need.

## 111. Generated code

Do not edit generated files manually. Locate generator/source and plan changes there.

## 112. Dependency audit

For each material dependency ask what it solves, usage breadth, stdlib/current-stack alternative, maintenance cost.

## 113. Dependency overengineering

Challenge large frameworks used for one retry/event/config/state-machine feature, accounting for migration cost.

## 114. Reflection audit

Reflection may be valid; inspect type-safety and hot-path cost.

## 115. Regex audit

Find regex compilation inside loops/functions; compile static patterns once. Avoid regex when simple string operations suffice.

## 116. Parser audit

If multiple parsers interpret the same format, establish canonical parsing semantics.

## 117. Normalization audit

Compare TrimSpace/casing/path/URL normalization across layers; inconsistent normalization causes subtle collisions.

## 118. Case sensitivity

Audit Windows/Linux/filesystem/DB/identifier case semantics.

## 119. Path normalization

Unify equivalent forms (`a/b`, `a\\b`, `./a/b`, absolute) with platform-aware canonical rules where appropriate.

## 120. URL normalization

Likewise define canonical URL semantics when identity depends on URLs.

## 121. Enum/state transitions

Recover allowed state transitions and identify illegal transitions or implicit reset semantics.

## 122. Multiple state machines

Find backend and UI independently enforcing the same transition policy; establish authoritative enforcement vs presentation mirror.

## 123. Permission audit

Authorization must have one authoritative enforcement point. UI visibility is not security.

## 124. Feature availability

Frontend can hide unavailable actions for UX, but backend must enforce capability. Distinguish intentional duplicate validation from multiple authoritative truth sources.

## 125. Prioritization model

Score findings by Severity, Probability, Blast radius, Frequency, Fix cost, Regression risk, Performance impact, Maintainability impact.

## 126. Severity levels

Use P0 Critical (data loss/corruption/security/severe concurrency), P1 High (incorrect behavior/divergent state/severe scaling), P2 Medium (maintainability/measurable performance/significant complexity), P3 Low (cleanup/naming/minor inefficiency). Do not make everything P1.

## 127. Confidence

Mark High/Medium/Low. Low-confidence findings require a verification task before refactor.

## 128. Finding format

Each finding must have ID, Priority, Confidence, Location, Problem, Evidence, Impact, Root cause, Recommended direction.

## 129. No generic recommendations

Do not say only “centralize config”. Specify exact files/types/functions, e.g. move timeout default to canonical config defaults, make resolved config mandatory, remove constructor fallback, migrate consumers.

## 130. Root-cause analysis

Prefer one fundamental fix that removes many symptoms over many local patches when evidence supports a shared root cause.

## 131. Consolidation discipline

Do not create a global `constants.go` dumping ground. Constants belong to the domain/module that owns them.

## 132. SSOT is not a god object

Do not fix duplicates by creating GlobalManager/GlobalConfig/AppContext singleton that knows everything. Keep canonical ownership modular.

## 133. Target architecture

After findings, define concrete target principles such as one config resolver, one domain status definition, one normalization function per identifier type, explicit cache ownership, typed units, no fallback defaults below config boundary—but only where supported by findings.

## 134. Dependency graph of fixes

Represent migration order such as canonical type → migrate readers → migrate writers → remove old representation.

## 135. Atomic remediation tasks

Break work into small logical tasks, each with one goal, compilable completion, tests, minimal simultaneous assumption changes.

## 136. Task format

Each task includes Goal, Reason/Finding IDs, Files, exact Changes, Migration, Backward compatibility, Tests, Verification, Definition of Done.

## 137. Ordering

Default priority:

**Safety → Correctness → Sources of truth → Contracts → Architecture simplification → Algorithmic complexity → Allocations/performance → Cleanup**.

Do not optimize micro-allocations before correctness.

## 138. Phase 0 safety net

Before large refactors, assess critical-path test coverage and add characterization tests where needed.

## 139. Characterization tests

When legacy behavior is unclear, freeze observable behavior before refactor to reduce accidental semantic change.

## 140. Phase 1 — Sources of truth

Fix duplicate policies, conflicting defaults, duplicate schemas, divergent state first.

## 141. Phase 2 — Contracts

Then canonical typed states, identifiers, config structures, validation boundaries.

## 142. Phase 3 — Simplification

Remove redundant wrappers, duplicate implementations, stale compatibility, unused abstractions.

## 143. Phase 4 — Algorithms

Improve asymptotic complexity after contracts stabilize.

## 144. Phase 5 — Allocations

Optimize allocations only after benchmark/profile evidence.

## 145. Phase 6 — Concurrency

Concurrency correctness may move earlier than normal if races are found; prioritize by severity.

## 146. Phase 7 — Cleanup

Remove deprecated code, dead constants, old flags, obsolete tests.

## 147. Test strategy

Choose Unit for local logic, Contract for boundaries, Integration for DB/files/network, Race for concurrency, Benchmark for performance, E2E for critical user workflows.

## 148. Go-specific verification

Where applicable run `go test ./...`, `go test -race ./...`, `go vet ./...`, `staticcheck ./...` when available.

## 149. Benchmarks

For hot paths use `go test -bench=. -benchmem` or targeted benchmark packages; retain baseline.

## 150. Benchstat

Use benchstat or equivalent before/after comparison when available. Do not claim optimization without evidence.

## 151. pprof

Use CPU, heap, allocation, mutex/block profiles where they answer a concrete suspicion.

## 152. Complexity scaling tests

Benchmark N=100/1k/10k/100k or domain-appropriate sizes to validate growth curve.

## 153. Memory growth

For caches/background processes, test long-running memory behavior, not only one-operation benchmarks.

## 154. Race tests

Add targeted tests for shared mutable state and ownership paths.

## 155. Collision tests

Test similar IDs, Unicode, separators, empty values, boundaries for identifier/cache/path generation.

## 156. Fuzzing

For parsers, serializers, normalization, identifiers, use fuzz tests when justified (native Go fuzzing where appropriate).

## 157. Property tests

Verify invariants such as `normalize(normalize(x)) == normalize(x)` and encode/decode round trips.

## 158. Failure injection

Test fail-after-A/fail-after-B/timeout-C in multi-step workflows and verify consistent recovery.

## 159. Regression tests

Each confirmed bug should preferably get a test that fails before and passes after the fix.

## 160. Tests should not mirror implementation

Assert observable contracts/invariants, not the same helper logic used by production.

## 161. Performance budgets

Where useful define startup, search, item processing, 10k-item processing, memory, allocations/op budgets.

## 162. No micro-optimization without ROI

Do not replace simple readable O(n) code with brittle complexity for a 2% gain on an infrequent path.

## 163. Optimization ROI

For performance tasks state Expected benefit, Complexity introduced, Measurement. Cancel low-value optimizations.

## 164. Simplification must preserve boundaries

Removing overengineering should not collapse useful testability, coupling boundaries, or realistic extensibility into a monolith.

## 165. Desired end state

Each domain policy has one owner; override precedence is explicit; defaults live at the right boundary; caches are explicitly derived; state mutation is controlled; magic values are classified; algorithms fit workloads; hot-path allocations are measured; abstractions are justified; stale architecture is removed.

## 166. Targeted anti-pattern search

Search repository for TODO, FIXME, HACK, legacy, deprecated, v2, old, new, default, fallback, override, timeout, retry, limit, max, min, cache, global, singleton, manager, factory, provider, adapter, wrapper, `interface{}`, `any`, `map[string]any`, and suspicious literals. Evaluate context rather than automatically filing bugs.

## 167. Numeric-literal search

Group repeated literals by value/context. Prioritize duration, limits, buffer size, ports, retries, batch size, thresholds, protocol codes.

## 168. String-literal search

Likewise group statuses, paths, environment names, event names, cache namespaces.

## 169. Call graph

For duplicate/suspicious functions build call graph to determine actual use vs legacy/parallel paths.

## 170. Write graph

For important state build an “all writers / all readers” graph; this is central to finding multiple sources of truth.

## 171. Config trace

For each critical setting trace input → parse → default → validate → override → storage → runtime use.

## 172. Data lineage

Perform the same end-to-end trace for other critical values.

## 173. Naming collisions

Find overlapping concepts named Config/Settings/Options/Preferences/Parameters and determine whether terminology hides duplicate concepts.

## 174. Concept map/glossary

Build a glossary of core domain concepts. Flag one concept described by many unrelated terms.

## 175. Type aliases/wrappers

Review excess wrappers/aliases, while recognizing typed domain values are often useful for avoiding unit/identifier collisions.

## 176. Boolean explosion

Functions/structs with many `enabled/force/strict/async/silent/legacy` booleans may hide a state machine. Evaluate enum/mode/options struct.

## 177. Parameter explosion

Functions with many parameters may indicate missing cohesive object/mixed responsibility/unstable contract. Do not automatically introduce builders.

## 178. Large-struct audit

Check cohesion of manager structs owning DB, network, config, UI, cache, etc.

## 179. God package

Audit catch-all `utils/common/helpers/manager` packages for missing domain boundaries.

## 180. Utility duplication

Look for custom helpers duplicating standard/library behavior.

## 181. Standard-library audit

Before maintaining custom infrastructure, check whether the standard library solves it more safely/simply.

## 182. Third-party vs custom

Conversely, do not add a dependency for a small stable block of clear code without benefit.

## 183. Code-generation opportunity

If schemas are duplicated across languages/layers, evaluate codegen only when it reduces real drift enough to justify build complexity.

## 184. Documentation drift

Compare README, config examples, comments, API docs against actual defaults/contracts. Documentation can be a third conflicting source of truth.

## 185. Comments as policy

Important business rules should not exist only in comments; encode them in code/types/tests/validation.

## 186. Final audit report structure

Produce:
A. Executive summary.
B. Architecture map.
C. Sources-of-truth matrix.
D. Override matrix.
E. Significant magic-value inventory.
F. Collision analysis.
G. Algorithmic complexity report.
H. Allocation/performance report.
I. Overengineering report.
J. Concurrency/reliability report.
K. Prioritized findings.
L. Target architecture.
M. Detailed remediation plan.
N. Test strategy.

## 187. Findings summary table

Use `ID | Area | Problem | Priority | Confidence | Impact | Fix complexity`.

## 188. Complexity table

For key algorithms use `Operation | Current | Expected N | Problem | Target | Evidence`, with Big-O only when actually established.

## 189. Allocation table

Use `Benchmark | ns/op | B/op | allocs/op | Target/After`.

## 190. Source-of-truth table

Use `Concept | Canonical target | Current writers | Duplicates | Fix`.

## 191. Override table

Use `Setting | Default | Config | Env | CLI | Runtime | Effective precedence`.

## 192. Overengineering table

Use `Component | Complexity | Actual need | Recommendation`, with recommendations Keep/Simplify/Merge/Remove/Re-evaluate.

## 193. Remediation roadmap phases

At minimum:
0. Safety & characterization.
1. Correctness / SSOT.
2. Contracts.
3. Architecture simplification.
4. Algorithms.
5. Allocations/performance.
6. Cleanup/documentation.
7. Final regression audit.

## 194. Prefer fundamental fixes

Prefer one architectural correction that removes many duplicate defects over many local patches, while avoiding an unnecessary big-bang rewrite.

## 195. Strangler refactor

For large changes use `Introduce canonical API → migrate one consumer → migrate remaining consumers → assert old API unused → remove legacy`.

## 196. Do not leave two paths forever

Migration adapters must have explicit removal tasks or they become a new source of truth.

## 197. Deprecation discipline

If old API remains temporarily, mark deprecated, prevent new usage, define removal milestone.

## 198. Definition of Done for SSOT

A SSOT issue is closed only when canonical owner is defined, all writers go through it, derived copies have invalidation, duplicate policy is removed, and tests verify precedence/consistency.

## 199. Definition of Done for magic values

Close only when the literal is intentionally retained/documented, converted to a named domain constant, moved to configuration, or represented as a typed value. Do not create constants mechanically.

## 200. Definition of Done for overengineering

Close when complexity is reduced, behavior remains correct, tests pass, control flow is easier to trace, and necessary extensibility is preserved.

## 201. Definition of Done for performance

Require baseline → change → benchmark/profile → comparison.

## 202. Correctness beats allocation count

Reject zero-allocation rewrites that materially increase brittleness without measurable need.

## 203. Final validation

Repeat the original audit after remediation: check for new duplicate sources, old compatibility paths, magic defaults, actual algorithm improvement, measured allocation improvement, and reduced architecture complexity.

## 204. Quality gate

Before completion: build passes, unit/integration tests pass, race tests pass, static analysis passes, benchmarks show no unexplained regressions, critical workflows pass.

## 205. Be explicit about uncertainty

If complexity/ownership cannot be proven without runtime data, mark `Requires measurement` and add a concrete verification step.

## 206. Avoid premature rewrites

Do not recommend rewriting a module from scratch until evidence shows incremental repair is more costly/risky.

## 207. Critical SSOT principle

Single Source of Truth means **one authoritative decision point**, not one physical copy. Caches/snapshots/projections are valid when ownership, sync, invalidation, and failure semantics are explicit.

## 208. Critical simplicity principle

Aim for the minimum complexity sufficient for the actual task, not the fewest files/interfaces.

## 209. Critical performance principle

Optimize measured hot paths, not code that merely looks inefficient.

## 210. Critical planning principle

The remediation plan cannot be `Refactor config / Improve performance / Add tests`. It must be detailed enough to open the named file and begin implementation without another architecture study.

## 211. Detailed task schema

Every task should include Task ID, Priority, Dependencies, Finding IDs, Files/packages, Current behavior, Target behavior, Exact implementation steps, API/type changes, Migration steps, Tests, Benchmarks, Failure cases, Rollback considerations, Definition of Done.

## 212. Plan audit

Review the remediation plan itself for Completeness, Ordering, Atomicity, Testability, Cleanup/removal of legacy, Regression protection, and performance baseline requirements.

## 213. Required final answers

The audit must answer:
1. Where are the true sources of truth?
2. Where is one policy defined multiple times?
3. Where/how are values overridden?
4. Which defaults conflict?
5. Which magic values are genuinely dangerous?
6. Where is state duplicated?
7. Where can identifier/config/state collisions occur?
8. Which algorithms have problematic complexity?
9. Which allocations matter?
10. Where can concurrency produce races/lost updates?
11. Which abstractions are justified?
12. What is overengineered?
13. What can be removed?
14. What should be centralized?
15. What must not become a global singleton?
16. Which fixes come first?
17. Which tests prove each change?
18. How will performance improvement be measured?
19. How will legacy paths be removed?
20. What should the target architecture become?

## 214. Final execution formula

Run the work as:

**Understand → Trace → Prove → Classify → Prioritize → Design target → Plan migration → Add safety net → Refactor → Measure → Test → Remove legacy → Audit again**.

Success means every important concept has obvious ownership, definition, mutation rules, override rules, and consistency tests; performance improvements are measurable; and architecture reduces the number of independent rules/states a developer must keep in mind.
