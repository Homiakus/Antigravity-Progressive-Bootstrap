package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 16,
		Name:    "provider_demand_history",
		SQL: `
CREATE TABLE provider_demand_dimensions (
    usage_key TEXT PRIMARY KEY,
    task_class TEXT NOT NULL CHECK(length(task_class) BETWEEN 1 AND 128),
    repository_class TEXT NOT NULL CHECK(length(repository_class) BETWEEN 1 AND 128),
    context_class TEXT NOT NULL CHECK(length(context_class) BETWEEN 1 AND 128),
    FOREIGN KEY(usage_key) REFERENCES provider_usage_samples(sample_key) ON DELETE CASCADE
);

CREATE INDEX provider_demand_dimensions_by_classes
    ON provider_demand_dimensions(task_class, repository_class, context_class, usage_key);

CREATE INDEX provider_usage_samples_by_model_metric_observed
    ON provider_usage_samples(model_id, metric, observed_at DESC, sample_key DESC);
`,
	})
}
