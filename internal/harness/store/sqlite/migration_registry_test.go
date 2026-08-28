package sqlite

import "testing"

func TestMigrationRegistryMatchesSchemaVersion(t *testing.T) {
	seen := make(map[int]string, len(migrations))
	maxVersion := 0
	for _, migration := range migrations {
		if migration.Version <= 0 {
			t.Fatalf("migration %q has invalid version %d", migration.Name, migration.Version)
		}
		if previous, ok := seen[migration.Version]; ok {
			t.Fatalf("duplicate migration version %d: %q and %q", migration.Version, previous, migration.Name)
		}
		seen[migration.Version] = migration.Name
		if migration.Version > maxVersion {
			maxVersion = migration.Version
		}
	}
	if maxVersion != SchemaVersion {
		t.Fatalf("maximum compiled migration=%d but SchemaVersion=%d", maxVersion, SchemaVersion)
	}
	for version := 1; version <= SchemaVersion; version++ {
		if _, ok := seen[version]; !ok {
			t.Fatalf("missing compiled migration version %d", version)
		}
	}
}
