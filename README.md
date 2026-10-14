# Takoform Forms

Resource contracts for JavaScript edge applications: Workers, storage,
queues, durable workflows and actors. Each Form defines a resource's inputs,
runtime interfaces, ownership, updates, deletion and recovery.

The publisher is `tako0614`. These Forms use the same Takoform rules as those
from any other publisher. The reference site is
[edge.forms.takoform.com](https://edge.forms.takoform.com/); the shared Host API
is defined by [Takoform](https://takoform.com/).

## Choose a Form

Start with the [Form guide](spec/guides/index.md). It connects common tasks to
the individual specifications: delivering a Worker, connecting storage,
processing queued work, running a durable workflow and calling an actor.

- [Use a Host](spec/guides/use-host.md): check support and Offerings, then
  create, observe, update and delete resources.
- [Terraform and OpenTofu](spec/guides/providers.md): choose a Provider that
  maps the exact Form and API versions you need.
- [Migration](spec/guides/migration.md): keep existing v1 identities and state
  distinct from the new URL-identified Forms.
- [Form inventory](forms/README.md): find the exact v2 specification or v1
  candidate package for each resource kind.

The Host API v2 specifications are Japanese normative documents under
`spec/forms/<Kind>/<version>/index.md`. Each Form has its own version-fixed
HTTPS URL; there is no aggregate Edge release or separately numbered
Specification stream. English site navigation can present the Japanese
original with a language notice; it is not a translated specification.

## What this repository contains

| Material | Status and use |
| --- | --- |
| Host API v2 Form specifications | 19 version-fixed Markdown documents for 17 resource kinds, with immutable anchors in [`spec/forms.freeze.json`](spec/forms.freeze.json). The SQLite migration 0.3.0 successors are source-only. Freezing makes exact bytes eligible for URL publication; it does not establish public availability or Host support. |
| Selected v1 source | 17 Forms, 8 Interfaces and 7 Bindings. The Actor/Workflow successors are unsigned and unpublished. |
| Installed v1 publisher evidence | The signed set [`e7f8a39311dd011b8467e97e7f300cabb9a6b06c`](forms/trust/sets/e7f8a39311dd011b8467e97e7f300cabb9a6b06c/) pins its earlier 17-package roster. The signed site build uses that roster, not the newer source selection. |
| Additional candidates | Container source packages and Vector/runtime experiments remain outside the selected publisher roster. See the [documentation map](docs/README.md#candidate-and-design-notes). |

The [inventory](forms/README.md#v1-package-history) explains the retained release
roots and evidence-only material. Local verification checks bytes and identities.
Anonymous readback is a separate observation of public content. Neither proves
that a Host has installed, admitted or activated a Form, or that a resource is
available to use.

## Forms, Interfaces and Bindings

| Term | Meaning |
| --- | --- |
| Form | The contract for one resource kind, including its identity and lifecycle. |
| Interface | The operations and semantics exposed by a resource or runtime. |
| Binding | A named capability made available to application code through an Interface. |
| Form Package (v1) | One Form Definition, its package index and data-only fixtures, verified as one byte set. |

In v1, a four-field FormRef identifies the Definition, while a package digest
identifies its full byte closure. For example, this is the selected
[SQLiteDatabase package's](forms/candidates/edge.forms.takoform.com/sqlite-database/package-index.json)
FormRef:

```json
{
  "apiVersion": "edge.forms.takoform.com",
  "kind": "SQLiteDatabase",
  "definitionVersion": "0.1.0",
  "schemaDigest": "sha256:c72eeb66ef96c4679b5c724fa1219d71c89bb7eeb9e543d73d868ec41bddddfe"
}
```

Host API v2 uses the exact Form URL instead. The
[migration guide](spec/guides/migration.md) explains why these are distinct
identities.

## Work locally

Use the tool versions in [`package.json`](package.json), [`go.mod`](go.mod) and
the [quality workflow](.github/workflows/quality.yml): Bun 1.3.14 and Go 1.25.8.
Node.js and Git 2.45 or newer must also be on `PATH`. Keep complete Git history;
the integrity and freeze checks need the pinned historical objects locally.

```console
bun install --frozen-lockfile
go mod download
bun run check
```

The complete gate checks formatting, owner boundaries, package generation,
history, trust evidence, frozen Form sources, rendered site candidates,
static analysis, compilation, portable tests and build. It uses disposable
site directories and performs no publication. Focused checks and browser
verification are described in [site maintenance](docs/site.md#build-and-verify).

To inspect one v1 candidate using the pinned Core v1.1.0 verifier:

```console
go run ./cmd/form-package verify forms/candidates/edge.forms.takoform.com/module-worker
```

The repository defines contracts and publishes their reference material.
Hosts implement them; Providers map them to client configuration and state.
For maintainers, the [documentation map](docs/README.md) links the source
boundaries, [package publication procedure](docs/publishing.md) and
[site maintenance procedure](docs/site.md).
