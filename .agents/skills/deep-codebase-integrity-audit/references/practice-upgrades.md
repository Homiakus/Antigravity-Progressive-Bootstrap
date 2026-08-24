# World-class practice upgrades

## Evidence hierarchy
Use the strongest available evidence:
1. Reproducible failing test/data-loss/race reproduction.
2. Runtime profile/benchmark/query plan/heap/race evidence.
3. Static call/write/read graph plus concrete conflicting implementation.
4. Git history/ADR/docs showing intended ownership or migration.
5. Smell/hypothesis requiring measurement.
Never promote level 5 to confirmed correctness/performance without verification.

## History and ownership artifacts
- Use git history/blame selectively for legacy/new paths, defaults, feature flags, and compatibility layers.
- Search ADRs, schemas, migrations, and API docs before deleting boundaries.
- Build writer/readers graph, config/default/override trace, canonical/derived/cache/snapshot classification, state transition map, retry/timeout budget tree, and resource cleanup map for critical concepts.

## Performance rigor
- Record runtime/toolchain/machine context for benchmark comparisons.
- Use repeated runs/statistical comparison when noise matters.
- Measure I/O counts/round trips/query plans for DB/network/file bottlenecks, not only CPU Big-O.
- Include realistic cardinalities; O(n²) over fixed N=8 may be acceptable.
- Optimize allocation lifetime/hot-path impact, not alloc counts as a vanity metric.

## Concurrency/refactoring safety
- Build retry multiplication trees and define idempotency/deduplication for retried side effects.
- Test cancellation, shutdown, lost update, check-then-act, timeout, and partial failure.
- Add characterization tests before changing poorly understood legacy behavior.
- Introduce target owner/contract, migrate consumers, assert old-path non-use, then remove adapters/flags.
- Avoid replacing duplication with a god config/context/service; keep ownership at the narrowest correct boundary.
- Record significant ownership decisions in an ADR or equivalent durable artifact.

## Final proof
Re-run the original reproducer/audit, tests, static/race tools, benchmarks/profiles, and verify legacy paths are actually gone.
