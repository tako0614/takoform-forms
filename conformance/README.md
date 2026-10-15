# Conformance corpus

This directory is the checked-in conformance corpus for provider-neutral Form
source. `takoform-v1/families/` preserves the desired-schema and composition
fixtures for the 17 Forms in the signed published-era package roster. These
fixtures are a fixed integrity baseline; they do not follow the newer selected
Form definitions.

The current Form, Interface, and Binding selections are rendered separately
by `cmd/current-form-source`. `bun run check:generation` regenerates and checks
their candidate package closures; it does not regenerate this historical
corpus. For current package and source-history counts, see the
[publisher inventory](../forms/README.md#v1-package-history) and
[publishing overview](../docs/publishing.md).

Container, Function, Table, PullQueue, Topic, Schedule, and Vector Forms were
unpublished candidates. They are deferred rather than released or deprecated
identities, so their catalogs and fixtures are intentionally absent from this
active corpus.

Each family directory contains data fixtures used by local catalog and
package validation. `integrity/source-baseline.json` pins their bytes, and
`bun run check:integrity` verifies that fixed baseline. A passing integrity or
generation check establishes fixture and package-source consistency; neither
is an executed Host conformance result.

Generic package verification, trust, and revocation conformance are owned by
the released public Core. Core remains publisher/family-neutral. This
repository keeps the Edge-specific publisher corpus; its checks
consume Core's embedded schemas and validators without a sibling checkout or
network access.

The source-only [Host API v2 SQLite migration PRAGMA corpus](host-v2/sqlite-migration/README.md)
covers the SQLiteMigrationApplication and SQLiteMigrationSet 0.3.0 successors.
It checks a narrow single-statement reference predicate and observes accepted
inputs using in-memory SQLite. These are local source tests, not executed
Host admission or hosted E2E evidence. The [0.4.0 introspection vectors](host-v2/sqlite-migration/v0.4.0/cases.json)
and [SQL fixture manifest](host-v2/sqlite-migration/v0.4.0/fixtures/manifest.json)
add shareable inputs for read-only metadata, catalog/ledger protection and
complete migration-file checks. See the [introspection guide](../docs/sqlite-migration-introspection.md)
for their scope and downstream adapter requirements. The historical v1 corpus
remains fixed.

The corpus deliberately has no live endpoint, account, credential, provider,
Host persistence, reconcile, activation, deployment, release, or publication
workflow. All publishers and consumers use the same package and verification semantics.
Publisher provenance does not grant a privileged verification path.

Useful local checks:

```console
bun run check:integrity
bun run check:generation
go test ./...
```
