# World-class practice upgrades

## Content model and governance
- Make entity identity, canonical fields, relationships, ownership, lifecycle, localization, and schema evolution explicit before UI design.
- Distinguish content ownership from workflow roles.
- Version schemas and plan migrations for long-lived systems.

## Publishing safety
- Treat publication as an externally visible transaction and make retries idempotent where needed.
- Use optimistic concurrency/version checks to prevent silent overwrite.
- Audit publication, permission, destructive, migration, and restore actions.
- Test partial failure across write/deploy/index/cache invalidation.

## Preview, permissions, operations
- Reuse production renderer/components when possible; disable side-effecting preview actions.
- Keep authorization authoritative server-side; hidden/disabled UI is only convenience.
- Prefer capability-oriented permissions over role explosion.
- Surface background publish/index/media failures with actionable observability.

## Media and evaluation
- Validate/sanitize uploads per threat model and track usage before replace/delete.
- Test realistic role combinations, stale versions, failed network/deploy, large libraries, and mobile emergency edits.
- Measure common-task completion time and error opportunities, not only visual satisfaction.
