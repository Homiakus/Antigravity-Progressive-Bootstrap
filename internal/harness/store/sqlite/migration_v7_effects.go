package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 7,
		Name:    "durable_effect_intents_and_reconciliation",
		SQL: `
CREATE TABLE effect_intents (
    effect_intent_id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_run_id TEXT NOT NULL,
    origin_attempt_id TEXT NOT NULL,
    last_attempt_id TEXT NOT NULL,
    operation_namespace TEXT NOT NULL,
    operation TEXT NOT NULL,
    effect_class TEXT NOT NULL CHECK(effect_class IN (
        'PURE','IDEMPOTENT','IDEMPOTENT_WITH_KEY','QUERYABLE','COMPENSATABLE','NON_IDEMPOTENT_UNKNOWN'
    )),
    idempotency_key TEXT NOT NULL UNIQUE,
    semantic_input_digest TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN (
        'PREPARED','DISPATCHED','CONFIRMED','FAILED','IN_DOUBT','COMPENSATED'
    )),
    prepared_at TEXT NOT NULL,
    dispatched_at TEXT,
    resolved_at TEXT,
    provider_ref TEXT NOT NULL DEFAULT '',
    result_digest TEXT NOT NULL DEFAULT '',
    error_class TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    reconcile_count INTEGER NOT NULL DEFAULT 0 CHECK(reconcile_count >= 0),
    last_reconciled_at TEXT,
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(node_run_id) REFERENCES node_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(origin_attempt_id) REFERENCES attempts(id) ON DELETE RESTRICT,
    FOREIGN KEY(last_attempt_id) REFERENCES attempts(id) ON DELETE RESTRICT
);

CREATE INDEX effect_intents_by_node
    ON effect_intents(node_run_id, prepared_at, effect_intent_id);
CREATE INDEX effect_intents_uncertain
    ON effect_intents(state, prepared_at, effect_intent_id)
    WHERE state IN ('DISPATCHED','IN_DOUBT');
CREATE INDEX effect_intents_by_workflow_state
    ON effect_intents(workflow_run_id, state, prepared_at, effect_intent_id);

CREATE TABLE effect_attempt_bindings (
    effect_intent_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    bound_at TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN (
        'PREPARED','DISPATCHED','CONFIRMED','FAILED','IN_DOUBT','COMPENSATED'
    )),
    dispatched_at TEXT,
    resolved_at TEXT,
    provider_ref TEXT NOT NULL DEFAULT '',
    result_digest TEXT NOT NULL DEFAULT '',
    error_class TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(effect_intent_id, attempt_id),
    FOREIGN KEY(effect_intent_id) REFERENCES effect_intents(effect_intent_id) ON DELETE CASCADE,
    FOREIGN KEY(attempt_id) REFERENCES attempts(id) ON DELETE RESTRICT
);

CREATE INDEX effect_attempt_bindings_by_attempt
    ON effect_attempt_bindings(attempt_id, effect_intent_id);
CREATE INDEX effect_attempt_bindings_uncertain
    ON effect_attempt_bindings(state, bound_at, effect_intent_id, attempt_id)
    WHERE state IN ('DISPATCHED','IN_DOUBT');
`,
	})
}
