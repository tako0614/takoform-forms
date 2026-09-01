# Takoform Forms

Takoform Forms is a provider-neutral catalog of resource contracts for
JavaScript edge runtimes. The model covers Worker applications and revisions,
traffic and endpoint attachments, KV, SQLite, queues, durable workflows, and
actors. A Form is a machine-readable desired-state contract that a host can
implement without changing its meaning.

**Current roster:** one family (`edge.forms.takoform.com`), 17 Forms, 8
Interfaces, and 7 Bindings.

The publisher signs 17 current package subjects. The append-only release tree
contains those 17 current roots plus the two explicitly retained historical
roots listed in [`forms/retained-packages.json`](forms/retained-packages.json),
for 19 release roots and immutable tags in total.

The current `module-worker.object-bucket@1.1.0` Binding projects the
`edge.objects@1.0.0` API with length-aware streaming: `put` and `uploadPart`
require an exact `contentLength` for `ReadableStream` bodies, while intrinsic
string and `ArrayBuffer` lengths may be checked automatically. The prior
1.0.0 candidate bytes remain recoverable from immutable repository and
Provider history; no retained Form Package contains that Binding, and no
published bytes are rewritten or reidentified.

## The four pieces

| Piece        | In plain language                                                                         |
| ------------ | ----------------------------------------------------------------------------------------- |
| Form         | The contract for one resource kind: its identity, desired state, and lifecycle shape.     |
| Interface    | A capability contract: operations and semantics exposed by a resource or runtime.         |
| Binding      | A named capability made available to worker code, resolved to an Interface.               |
| Form Package | One Form Definition, its package index, and data-only fixtures, verified as one byte set. |

Official and external publishers use the same package format and verification
rules. See the [Edge inventory](forms/README.md) and
[documentation map](docs/README.md).

For example, this four-field FormRef identifies an SQLite database:

```json
{
  "apiVersion": "edge.forms.takoform.com",
  "kind": "SQLiteDatabase",
  "definitionVersion": "0.1.0",
  "schemaDigest": "sha256:c72eeb66ef96c4679b5c724fa1219d71c89bb7eeb9e543d73d868ec41bddddfe"
}
```

A Form Package binds one Definition and its fixtures to that identity; its
digest covers the complete package byte set.

A minimal real desired state is a WorkerBundle manifest reference:

```json
{
  "manifestDigest": "sha256:6a5cbf24f5d0c86479ae13b9d1731a626a1729f01aef65403c5c8ac82ed85f43"
}
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

Focused checks: `bun run check:generation`, `bun run check:publication`, and
`bun run check:trust`.

## Preparing and publishing packages

For a verified `package-index.json`, Core v1.1.0 `PublicationLocatorFor` derives
`releaseId` from the FormRef group and kind, `artifactId` from `packageDigest`
(`sha256:` becomes `sha256-`), then combines them into the release path and tag:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

`bun run write:publication` materializes missing release directories. It does
not sign or publish them, and never rewrites the two retained roots. An exact
protected-main commit is prepared for
external keyless signing with:

```console
bun run prepare:trust -- --output <empty-external-directory>
```

The manual `form-package-signing.yml` workflow is the publisher authority. It
uses GitHub Actions OIDC to produce 17 exact current package-index Sigstore bundles and
one signed Core API v1 revocation checkpoint, reruns the complete bounded
checkpoint chain, and uploads a one-day candidate. With blank revocation
inputs it permits only the first genesis set. With `previous_set` and
`statement_version` it anonymously reads the exact public predecessor before
signing an append-only advancement. It has no repository write, tag, or
publish permission. An operator then verifies and imports that candidate
create-only:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

The deploy surface requires the imported set's exact signed source commit:

```console
bun run deploy -- form-packages-edge --trust-set <source-commit> --dry-run
bun run deploy -- form-packages-edge --trust-set <source-commit>
bun run deploy -- form-packages-edge --trust-set <source-commit> --verify
```

`--dry-run` checks preconditions without mutation. The publish command pushes
`main` and the matching tags; run `--verify` afterwards for anonymous public
readback. Existing immutable package tags may point to an older commit only
when their package paths are byte-identical to the signed source. Anonymous
readback fetches every tag, compares those bytes, and reruns Core v1.1.0 over
all 19 release roots (17 current plus two retained), bundles, the pinned publisher policy and trusted root, the signed
checkpoint, and every not-revoked decision. Changing package bytes creates a
new digest, path, and package tag; changing publisher evidence creates a new
`forms/sets/<source-commit>` identity.

Revocation advancement additionally creates exactly one immutable
`forms/revocations/v<statementVersion>` tag in the same atomic push as the new
set. A lost push acknowledgement is settled only by an exact anonymous
readback; rerunning the same command does not push again when every public ref
and byte already matches. Fork, rollback, missing history, tag insertion,
retagging, update, and deletion all fail closed. See the
[revocation advancement runbook](docs/revocation-advancement.md), including
the safe partial-install recovery procedure.

No signed set is checked in yet. Until an authorized OIDC signing run is
verified and imported, `bun run check` verifies the empty pre-signing state and
the deploy surface refuses every publication request.

Core defines verification; this repo defines Edge contracts; providers map them;
hosts implement them. Publication proves package bytes and identity, not Host
support or Host admission policy.
