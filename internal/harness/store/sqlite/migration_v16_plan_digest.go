package sqlite

func init() {
	migrations = append(migrations, migration{
		Version: 16,
		Name:    "provider_assignment_plan_digest",
		SQL: `
ALTER TABLE provider_assignments ADD COLUMN plan_digest TEXT NOT NULL DEFAULT '';

CREATE INDEX provider_assignments_by_plan_digest
    ON provider_assignments(plan_digest, state, updated_at, id);
`,
	})
}
