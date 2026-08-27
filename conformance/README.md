# Conformance corpus

This directory is the checked-in conformance corpus for the provider-neutral
Form source. The active corpus is `takoform-v1/families/`: one fixture set for
the current Edge-first versionless Form family and all 16 current Forms.
Fixtures are data, not a provider or Host implementation.

Container, Function, Table, PullQueue, Topic, Schedule, and Vector Forms were
unpublished candidates. They are deferred rather than released or deprecated
identities, so their catalogs and fixtures are intentionally absent from this
active corpus.

Each family directory contains the exact desired-schema and composition
fixtures used by the local catalog and package validators. The fixtures are
covered by `integrity/source-baseline.json`; `bun run check:integrity` checks
their bytes and `bun run check:generation` verifies that the current catalogs
reproduce the checked-in candidate packages.

Generic package verification, trust, and revocation conformance are owned by
the released public Core. Core remains publisher/family-neutral. This
repository keeps only the Edge-specific publisher corpus above; its checks
consume Core's embedded schemas and validators without a sibling checkout or
network access.

The corpus deliberately has no live endpoint, account, credential, provider,
Host persistence, reconcile, activation, deployment, release, or publication
workflow. Official and third-party consumers use the same package and
verification semantics; official status is provenance maintained outside this
data repository.

Useful local checks:

```console
bun run check:integrity
bun run check:generation
go test ./...
```
