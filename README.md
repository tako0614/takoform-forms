# Takoform Forms

Takoform Forms is a provider-neutral catalog of resource contracts for
JavaScript edge runtimes. The model covers Worker applications and revisions,
traffic and endpoint attachments, KV, SQLite, queues, durable workflows, and
actors. A Form is a machine-readable desired-state contract that a host can
implement without changing its meaning.

**Current roster:** one family (`edge.forms.takoform.com`), 16 Forms, 7
Interfaces, and 6 Bindings.

## The four pieces

| Piece | In plain language |
| --- | --- |
| Form | The contract for one resource kind: its identity, desired state, and lifecycle shape. |
| Interface | A capability contract: operations and semantics exposed by a resource or runtime. |
| Binding | A named capability made available to worker code, resolved to an Interface. |
| Form Package | One Form Definition, its package index, and data-only fixtures, verified as one byte set. |

Official and external publishers use the same package format and verification
rules. See the [Edge inventory](forms/README.md) and
[documentation map](docs/README.md).

For example, this four-field FormRef identifies an SQLite database:

```json
{"apiVersion":"edge.forms.takoform.com","kind":"SQLiteDatabase","definitionVersion":"0.1.0","schemaDigest":"sha256:c72eeb66ef96c4679b5c724fa1219d71c89bb7eeb9e543d73d868ec41bddddfe"}
```

A Form Package binds one Definition and its fixtures to that identity; its
digest covers the complete package byte set.

A minimal real desired state is a WorkerBundle manifest reference:

```json
{"manifestDigest":"sha256:6a5cbf24f5d0c86479ae13b9d1731a626a1729f01aef65403c5c8ac82ed85f43"}
```

It is the [checked-in desired fixture](forms/candidates/edge.forms.takoform.com/worker-bundle/fixtures/desired.json).

## Local flow

Install the pinned tools, then run the complete read-only gate:

```console
bun install --frozen-lockfile
bun run check
```

To inspect one package directly with the released Core verifier:

```console
go run ./cmd/form-package verify forms/candidates/edge.forms.takoform.com/module-worker
```

Focused checks: `bun run check:generation` and `bun run check:publication`.

## Preparing and publishing packages

For a verified `package-index.json`, Core v1.0.1 `PublicationLocatorFor` derives
`releaseId` from the FormRef group and kind, `artifactId` from `packageDigest`
(`sha256:` becomes `sha256-`), then combines them into the release path and tag:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

`bun run write:publication` materializes missing release directories. The
deploy surface is:

```console
bun run deploy -- form-packages-edge --dry-run
bun run deploy -- form-packages-edge
bun run deploy -- form-packages-edge --verify
```

`--dry-run` checks preconditions without mutation. The publish command pushes
`main` and the matching tags; run `--verify` afterwards for anonymous public
readback. A package is public only as part of a set whose public `main` and all
16 Core-derived tags point to one commit; anonymous readback must verify every
release path byte-for-byte with Core v1.0.1. Git tags provide unsigned
provenance; changing package bytes creates a new digest, path, and tag.

Core defines verification; this repo defines Edge contracts; providers map them;
hosts implement them. Publication proves package bytes and identity, not Host
support.
