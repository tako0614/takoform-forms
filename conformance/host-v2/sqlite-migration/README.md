# SQLite migration PRAGMA corpus — source only

This corpus targets the unpublished Host API v2
[SQLiteMigrationApplication 0.3.0](../../../spec/forms/SQLiteMigrationApplication/0.3.0/index.md)
and [SQLiteMigrationSet 0.3.0](../../../spec/forms/SQLiteMigrationSet/0.3.0/index.md)
source contracts. Application owns SQL execution and its narrow PRAGMA
exception. Set retains ordered bytes without validating SQL syntax or effects.
Both 0.2.0 identities retain their original bytes and behavior.

`cases.json` is data-only input and expected admission results for a **single
PRAGMA statement**. The two Yurucommu examples copy only their relevant SQL
statement and name the repository-relative source locations; tests do not need
a sibling checkout. The accepted values are `ON`, `OFF`, `TRUE`, `FALSE`, `1`
and `0`, with ASCII case-insensitive spelling, ASCII whitespace and an optional
terminal semicolon. All other PRAGMAs, read-only forms and alternate SQL
spellings remain denied.

`allowed-pragma.mjs` is a source-only reference predicate for that narrow
statement shape. It does not parse a migration file, authorize other SQL or
run a Host. A Host must use its SQLite parser and authorizer to enforce all
execution restrictions, isolate the file transaction and protect its ledger.
Passing an entire file to this predicate is not a conformance adapter. In a
multi-statement file, each statement still requires its own admission check;
an allowed PRAGMA does not admit any neighboring statement.

The local test executes every accepted case against Bun's in-memory SQLite,
observes both setting directions inside a transaction, and checks reset after
rollback. It also exercises deferred foreign-key enforcement, commit reset,
and SQL/history rollback after a failed commit. Read-only PRAGMAs and
`foreign_keys = ON` appear only in privileged test setup and observations;
they are not accepted migration inputs.

```console
bun test scripts/sqlite-migration-pragma.test.mjs
```

Rejection tests exercise the reference predicate. This result does not prove
that any Host implements these new Forms, that hosted E2E has recovered, or
that the Form URLs are publicly available. Host-specific admission and E2E
must be verified by the owning implementation. No publication or deployment
is performed by this corpus or its tests.

## 0.4.0 read-only introspection vectors

The immutable 0.3.0 behavior above remains the scope of `cases.json` and
`allowed-pragma.mjs`. The [0.4.0 cases](v0.4.0/cases.json) and
[fixture manifest](v0.4.0/fixtures/manifest.json) are separate data-only
inputs for SQLiteMigrationApplication/Set 0.4.0. They cover four read-only
metadata PRAGMAs and their table-valued forms, main catalog reads, denied
configuration/catalog/ledger mutations and complete migration SQL. These
cases require a real SQLite parser/authorizer in the consuming adapter;
the 0.3.0 single-statement predicate does not validate them.

Read the [introspection guide](../../../docs/sqlite-migration-introspection.md)
for dynamic object arguments, database scope, the distinction between direct
catalog writes and legitimate DDL, and the boundary between local source
checks and actual Host conformance. These additions do not change the Worker
SQLiteDatabase 0.2.0 Binding or establish public Form availability.
