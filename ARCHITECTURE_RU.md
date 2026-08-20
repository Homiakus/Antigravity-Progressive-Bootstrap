# Архитектура agctl 3.2.1

## Цель

`agctl 3.2.1` — audited adaptive control-plane над Antigravity/AGY. Он использует нативный runtime Antigravity для reasoning/tools/subagents, но управляет discovery, composition, governance, scheduling, verified completion и runtime replanning.

```text
                          USER GOAL
                             │
                             ▼
                    Antigravity / AGY
                             │
             ┌───────────────┼───────────────┐
             │               │               │
           /goal         subagents         MCP/tools
             │               │               │
             └───────────────┼───────────────┘
                             ▼
                       agctl 3.2
                     CONTROL PLANE
                             │
      ┌──────────────────────┼──────────────────────┐
      │                      │                      │
  DISCOVERY              ORCHESTRATION          GOVERNANCE
      │                      │                      │
 skills                 initial DAG             permissions
 MCP                    adaptive DAG            semantic risk
 plugins                task supervisor         supply-chain
 agents                 worktree lanes          security score
 workflows              retries/recovery        provenance
 capabilities           no-progress detector    budgets
      │                      │                      │
      └──────────────────────┼──────────────────────┘
                             ▼
                         EXECUTION
                             │
                 evidence / failure / findings
                             │
                             ▼
                    ADAPTIVE REPLANNER
                             │
             ┌───────────────┼────────────────┐
             │               │                │
      proposal accepted   failure         no progress
             │               │                │
             ▼               ▼                ▼
       add DAG nodes    diagnose/repair     BLOCK
             │          /reverify chain
             └───────────────┼────────────────┘
                             ▼
                    dependency rewiring
                             │
                             ▼
                        scheduler
                             │
                             ▼
                      VERIFIED DONE
```

## 1. Initial DAG Planner

`planner.Create()` строит explainable initial DAG по task text, project profile и Capability Registry.

Это стартовая гипотеза, а не immutable workflow.

План содержит:

```text
revision
status
updatedAt
dynamicNodeCount
revisionHistory
nodes[]
```

Узел содержит:

```text
agent
dependsOn
verification
tags
resources
risk
workspace/worktree metadata
dynamic flag
parent node
depth
```

## 2. Adaptive proposal protocol

Plan task получает `replanProposalPath`.

Выполняющий agent может предложить новые actions только для material requirements, обнаруженных после старта DAG.

Proposal проходит:

```text
identity validation
→ evidence gate
→ confidence gate
→ action DAG cycle validation
→ semantic risk threshold
→ dynamic growth budget
→ materialization
```

Reject proposal не превращает успешный parent task в failure.

## 3. Failure recovery

После исчерпания обычных task retries final failure не обязан уничтожать весь DAG.

Replanner создаёт:

```text
failed node
    ↓
diagnose
    ↓
repair
    ↓
reverify
```

Старый task получает:

```text
status = superseded
```

Queued downstream tasks rewired на новый verification gate.

Это позволяет plan завершиться `completed`, сохраняя исторический факт initial failure.

## 4. No-progress detector

Failure signature строится из error + tail execution log и хранится в revision history.

Автономное восстановление останавливается при:

```text
maxRepairDepth
maxSameFailure
maxRevisions
maxDynamicNodes
```

Plan переводится в `blocked` с `blockReason`, вместо бесконечного self-healing loop.

## 5. Revision model

Каждое изменение DAG увеличивает `plan.revision`.

`PlanRevision` хранит:

```text
number
createdAt
triggerTaskId
triggerNodeId
reason
kind
addedNodes
rewiredNodes
failureSignature
fingerprints
```

`kind` сейчас:

```text
proposal
failure-recovery
no-progress / budget state event
manual
```

## 6. Dependency rewiring

Replanner изменяет одновременно два слоя:

```text
ExecutionPlan node dependencies
+
persisted TaskRecord dependencies
```

Running/finished downstream tasks не мутируются; queued/blocked tasks могут быть rewired.

Dynamic tasks с `parentTaskId` исключаются из blanket replacement, чтобы не создать self-cycle.

## 7. Worktree lanes

Автоматические parallel worktrees разрешены только когда:

```text
preferWorktrees = true
parent node is read-only
>=2 independent parallelizable write actions
```

Это ограничение важно: новый worktree создаётся от Git `HEAD` и не должен потерять uncommitted изменения write-parent.

Каждый isolated write action обязан локально commit-нуть изменения. После завершения ветвей создаётся integration node на base workspace.

## 8. Resource-aware scheduler

Scheduler 3.2 наследует 3.1 budgets:

```json
{
  "maxParallel": 2,
  "cpuWeight": 100,
  "buildSlots": 1,
  "browserSlots": 1,
  "maxTaskMinutes": 120,
  "maxRetries": 1
}
```

Adaptive nodes используют тот же scheduler и не обходят ресурсные ограничения.

## 9. Concurrency model

Каждый task получает atomic `.run.lock`.

Observer callbacks replanner-а сериализуются, чтобы параллельно завершившиеся DAG nodes не записывали одну plan revision одновременно.

Task results и `firstErr` синхронизируются единым mutex; race-test входит в release gate.

## 10. Completion semantics

Plan может иметь historical `superseded` tasks.

Для итогового completed state допустимы:

```text
succeeded
superseded
```

но не:

```text
failed
blocked
cancelled
queued
running
```

`superseded` означает: original execution failed, но его dependency role была заменена verified recovery chain.

## 11. Security boundary

Adaptive proposal не является безусловным приказом модели.

Control-plane применяет собственные gates:

```text
confidence
concrete evidence
risk ceiling
growth budget
repair depth
failure repetition
```

Default `autoApplyRiskMax=write-medium`; destructive/external-high actions не добавляются автоматически.

Runtime tool calls дополнительно проходят Semantic Risk Engine / Antigravity permission policy.

## 12. Observability

Telemetry включает:

```text
plan.created
plan.enqueued
plan.status
replan.applied
replan.failure_recovery
replan.proposal.rejected
replan.proposal.invalid
replan.blocked
task.queued
task.started
task.retry
task.blocked
task.finished
```

Dashboard агрегирует:

```text
plan count
revision count
dynamic node count
replan inbox
task states
security score
capabilities
recent events
```

## 13. Storage

```text
~/.gemini/config/agctl/
├── router.json
├── loop.json
├── orchestrator.json
├── task-supervisor.json
├── capabilities.json
├── risk-policy.json
├── plans/
├── tasks/
├── replan/
│   ├── config.json
│   ├── inbox/
│   └── archive/
├── locks/
├── registry-cache/
├── security/
└── telemetry/
```

## 14. Design principle

3.2 отделяет три вида автономности:

```text
Antigravity /goal
= продолжать agent execution

agctl Completion Gate
= не принимать непроверенную готовность

agctl Adaptive Replanner
= менять сам план, когда реальность расходится с initial DAG
```

Именно это позволяет системе не только «дольше работать», а **адаптировать стратегию выполнения без вмешательства пользователя**.

---

## Compatibility contract 3.2.1

`agctl` намеренно разделяет четыре поверхности Antigravity, у которых различаются discovery/runtime contracts:

1. **Antigravity 2.0 / IDE** — `~/.gemini/config/*` и workspace `.agents/*`;
2. **Antigravity CLI** — shared MCP/custom agents плюс `~/.gemini/antigravity-cli/*` для CLI settings/plugins/flat skill mirrors;
3. **MCP protocol** — 2026-07-28 stateless probe, legacy fallback на 2025-11-25;
4. **agctl control-plane** — task/DAG/replan/provenance/telemetry state под `~/.gemini/config/agctl`.

Ни один недокументированный внутренний storage IDE не патчится. В частности, Project-specific GUI security settings не подменяются записью в неизвестную БД: `agctl` управляет документированными файлами и выдаёт audit/checklist для GUI-only scope.

См. `AUDIT_RU.md` для полной матрицы.

