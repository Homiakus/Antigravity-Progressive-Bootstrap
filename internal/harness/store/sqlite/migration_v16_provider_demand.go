package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 16,
		Name:    "provider_demand_history",
		SQL: `
CREATE TABLE provider_demand_dimensions (
    usage_key TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL,
    metric TEXT NOT NULL CHECK(metric IN ('TOKENS','REQUESTS','COST','FRACTION')),
    task_class TEXT NOT NULL CHECK(length(task_class) BETWEEN 1 AND 128),
    repository_class TEXT NOT NULL CHECK(length(repository_class) BETWEEN 1 AND 128),
    context_class TEXT NOT NULL CHECK(length(context_class) BETWEEN 1 AND 128),
    usage_observed_at_ns INTEGER NOT NULL,
    UNIQUE(assignment_id, metric),
    FOREIGN KEY(usage_key) REFERENCES provider_usage_samples(sample_key) ON DELETE CASCADE,
    FOREIGN KEY(assignment_id) REFERENCES provider_assignments(id) ON DELETE CASCADE
);

CREATE TRIGGER provider_demand_dimensions_require_settled_usage
BEFORE INSERT ON provider_demand_dimensions
FOR EACH ROW
WHEN NOT EXISTS (
    SELECT 1
    FROM provider_usage_samples u
    JOIN provider_reservations r
      ON r.id=u.reservation_id AND r.assignment_id=u.assignment_id
    WHERE u.sample_key=NEW.usage_key
      AND u.assignment_id=NEW.assignment_id
      AND u.metric=NEW.metric
      AND r.state='SETTLED'
)
BEGIN
    SELECT RAISE(ABORT, 'provider demand usage must reference a settled reservation');
END;

CREATE INDEX provider_demand_dimensions_by_classes_time
    ON provider_demand_dimensions(task_class, repository_class, context_class, usage_observed_at_ns DESC, usage_key DESC);

CREATE INDEX provider_usage_samples_by_model_metric_account
    ON provider_usage_samples(model_id, metric, account_id, sample_key);
`,
	})
}
