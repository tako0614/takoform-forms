# SQLite migration 0.4.0 shared vectors

`cases.json` contains complete SQL inputs, a common trusted `setupSql`, and
optional per-case trusted `setupSql`, and expected `allowed` admission results for the 0.4.0 Migration Application.
Selected metadata cases also provide `expectedRows`. Start each case with a
fresh database and foreign keys enabled. The fixture ledger is named
`migration_history`; a Host adapter should substitute its actual protected
ledger name. Run each input as one file inside the Host transaction, and check
the shared `observations` after success or rollback. A denied input must leave
no file effects and no ledger entry. Setup SQL is privileged fixture setup,
not admitted migration input.

`fixtures/manifest.json` identifies three unchanged complete Yurucommu
migration files (0003, 0011 and 0036), each ordered setup sequence, and the
SHA-256 of every local SQL fixture. `source.ref` fixes the upstream Git object;
`sourcePath` names the original file. The 0036 setup is a schema-only extract
of the required objects after the preceding migrations. Its `derivedFrom`
records the original schema inputs and digests. Fixtures contain no application
rows or operator database state. They do not depend on a sibling checkout.
Apply each setup file in a separate transaction, then pass the target file
through the adapter's normal migration validation and execution path.

`reference-check.mjs` and the Node test are **test-only** source conformance.
SQLite's parser exposes the exact statement boundaries (including trigger
bodies); its authorizer observes metadata calls, dynamic arguments, resolved
databases and side effects. The strict `defer_foreign_keys` exception also
checks the raw statement prefix because authorizer arguments normalize syntax.
Inter-statement comments are file trivia. The old 0.3.0 predicate remains a
standalone single-statement check with its original corpus.

The reference checker uses an in-memory SQLite database, a fixture ledger and
a limited effect policy. It does not implement artifact admission, resource
UID authorization, durable ledger reconciliation, backend isolation or a Host
adapter. In particular, a production adapter must authorize its own functions,
virtual tables, schema objects and backend-specific internal effects. It must
keep its authorizer effective during prepare, step, reprepare and trigger/view
execution. SQLite's internal catalog updates for legitimate DDL must be
distinguished from direct SQL writes. The reference has an explicit exception
for ALTER's internal temp catalog rewrite; it grants no user temp access.

Run the local source check with Node 24.10 or newer (CI pins 26.1.0):

```console
node --test scripts/sqlite-migration-readonly.node-test.mjs
```

The complete repository gate includes this test and fails if the required
SQLite authorizer API is unavailable. Bun 1.3.14 does not expose that API.
Host adapter tests belong to the consuming implementation; it can reuse the
JSON/SQL data without importing the reference checker. Passing these local
source checks does not establish Host support or hosted E2E recovery. These
files neither publish Forms nor deploy anything.
