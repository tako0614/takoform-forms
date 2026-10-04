# Takoform Forms

Takoform Forms is a provider-neutral catalog of resource contracts for
JavaScript edge runtimes. The model covers Worker applications and revisions,
traffic and endpoint attachments, KV, SQLite, queues, durable workflows, and
actors. A Form is a machine-readable desired-state contract that a host can
implement without changing its meaning.

**Selected source roster (unpublished):** one family
(`edge.forms.takoform.com`), 17 Forms, 8 Interfaces, and 7 Bindings. Nine Form
versions, three Interface versions, and two Binding versions are prepared but
not yet signed, published, or qualified as Host support. The last published
set still pins its earlier 17-package roster.

Read the [individual Form reference](https://edge.forms.takoform.com/) to choose
a contract and inspect its purpose, related Forms, fields, constraints and exact
package examples. [How to read the model](docs/site.md#read-the-reference) explains
Worker delivery, capability bindings and SQLite migrations. Older versions stay
available under **Retained versions**, with their own historical definitions.

The last published set signed 17 package subjects. The selected source closure
has 40 release roots: 17 current roots (eight unchanged and nine new
Actor/Workflow successors), 11 signed-history/explicitly-retained roots, three
abandoned evidence-only roots, and nine exact prior-source roots recorded in
[`forms/source-history.json`](forms/source-history.json). That inventory is
anchored to source commit `85b2f755a0cf104caf1ae8cb3738e475d55fe988`; the
publisher verifies those package bytes against that Git tree and Core's
package locator. These nine roots are unsigned source history only: they are
included in local/anonymous full-tree closure, never in a signed set, package
tag roster, or trust decision. The existing 19 publishable package tags belong
to the earlier 17 plus the two explicitly retained roots. Only after an
independently reviewed successor set and publication could the nine new
current roots bring that total to 28; the three abandoned roots never receive
tags.

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

All publishers use the same package format and verification
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

A canonical schema-valid desired-state example is a WorkerBundle manifest reference:

```json
{
  "manifestDigest": "sha256:6a5cbf24f5d0c86479ae13b9d1731a626a1729f01aef65403c5c8ac82ed85f43"
}
```

It is the [checked-in desired fixture](forms/candidates/edge.forms.takoform.com/worker-bundle/fixtures/desired.json).
This example digest does not provide a live artifact. It is a contract example,
not a complete Host API request, OpenTofu configuration or runnable deployment.

## Local flow

Install the pinned tools, then run the complete read-only gate:

```console
bun install --frozen-lockfile
go mod download
bun run check
```

To inspect one package directly with the released Core verifier:

```console
go run ./cmd/form-package verify forms/candidates/edge.forms.takoform.com/module-worker
```

Focused checks: `bun run check:generation`, `bun run check:publication`, and
`bun run check:trust`.
For unpublished selected source, `bun run check:edge-form-pages:source` builds
and discards an honest preview. `bun run build:edge-form-pages` and the explicit
browser lane `bun run check:edge-form-pages:browser` apply only after an exact
signed set is installed; see [site maintenance](docs/site.md).

## Preparing and publishing packages

For a verified `package-index.json`, Core v1.1.0 `PublicationLocatorFor` derives
`releaseId` from the FormRef group and kind, `artifactId` from `packageDigest`
(`sha256:` becomes `sha256-`), then combines them into the release path and tag:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

`bun run write:publication` materializes missing current release directories.
It does not sign or publish them, and never rewrites historical, explicitly
retained, source-history, or abandoned evidence-only roots. An exact
protected-main commit is prepared for
external keyless signing with:

```console
bun run prepare:trust -- --output <empty-external-directory>
```

The manual `form-package-signing.yml` workflow is the publisher authority. It
uses GitHub Actions OIDC to sign every exact selected package-index subject
and the required lineage/checkpoint subjects for the chosen transition, reruns the complete bounded
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

The source also has a separate, not-yet-published continuation preparation
path for an explicit complete active release roster without a revocation. The
requested roots must exactly match the digest-pinned current-family index and
Edge candidate selection; caller flags cannot promote retained or abandoned
release roots.
It inherits the latest signed checkpoint bundle and chain byte-for-byte, and
prepares every active package index plus one publisher-owned signed lineage
subject from the same new commit. Later no-revocation successors may select
another separately promoted publisher-owned active roster while all old
release, tag, and set bytes stay immutable. This does not add the source-only
Container candidates to the selected 17-Form roster or make a 19-package
release ready. The current
publication plan and deploy surface continue to require the exact current
roster. See the
[continuation source note](docs/publisher-continuation.md).

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
readback fetches every exact publishable current/signed-historical package
tag, compares their bytes, separately checks the untagged source-history roots
from the anonymous main-tree readback, and reruns Core v1.1.0 over all local
release roots. The currently public predecessor still has 19 package tags;
the unsigned selected source has 40 local roots and cannot pass this
publication step.
Readback also verifies the
signature bundles, pinned publisher policy and trusted root, signed checkpoint,
and every not-revoked decision. Changing package bytes creates a
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

The checked-in `cdd30b711e2c6857b1b4d247b1471f5676904933` signed set is
cryptographically verified but explicitly abandoned as evidence-only because
three package identities were superseded before publication. Its set tag and
the three old package tags must remain absent, and the deploy surface refuses
that set. The repository also contains its previously deployed successor,
`e7f8a39311dd011b8467e97e7f300cabb9a6b06c`, with the last published package identities. The selected Actor/Workflow source successors are newer, unsigned candidates and cannot be deployed from that set.
Do not confuse the retained abandoned evidence with the selected publication
set. Use `--verify` with the exact successor set to establish current public
tag and byte availability; checked-in evidence alone does not prove it.

Core defines verification; this repo defines Edge contracts; providers map them;
hosts implement them. Publication proves package bytes and identity, not Host
support or Host admission policy.
