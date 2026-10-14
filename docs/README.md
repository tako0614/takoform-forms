# Documentation map

Use the reader guides to choose and use a Form. Use the maintainer documents
for source, verification and publication work.

## Reader guides

| Start here | What it covers |
| --- | --- |
| [Form guide](../spec/guides/index.md) | Choose a resource and follow its exact Host API v2 specification. |
| [Use a Host](../spec/guides/use-host.md) | Support, Offering selection, operations and uncertain responses. |
| [Terraform and OpenTofu](../spec/guides/providers.md) | Provider scope, exact version checks and composition. |
| [Migration](../spec/guides/migration.md) | Existing v1 identities and state versus new URL-identified Forms. |
| [Form inventory](../forms/README.md) | Individual v2 source documents, selected v1 packages and retained history. |

The public reference is [edge.forms.takoform.com](https://edge.forms.takoform.com/).
The repository's frozen sources, a successful local build and a Host's support
are distinct facts; consult the Host's actual support and resource observations
before using a contract.

## Maintainer documents

- [Site maintenance](site.md): reader routes, source and signed build modes,
  local/browser checks, URL publication and public readback.
- [Package publication](publishing.md): v1 generation, signing, create-only
  installation, publication and forward repair.
- [Publisher trust evidence](../forms/trust/README.md): pinned authority,
  signature subjects and installed evidence.
- [Revocation advancement](revocation-advancement.md): append-only revocation,
  immutable publication, settlement and partial-install recovery.
- [Publisher continuation](publisher-continuation.md): source support for a
  successor signed roster without a new revocation.
- [Conformance corpus](../conformance/README.md): historical fixtures, selected
  candidate checks and executable-input evidence boundaries.

## Source and generated files

| Source or output | Role |
| --- | --- |
| `spec/forms/<Kind>/<version>/index.md` | Japanese normative source for each Host API v2 Form. The exact bytes are anchored by `spec/forms.freeze.json`; do not edit a frozen identity. |
| `spec/guides/` | Explanatory reader guides. These may change without allocating another Form version. |
| `internal/` and `cmd/current-form-source` | Go authoring and selection of v1 Form, Interface and Binding candidates. |
| `forms/candidates/`, `interfaces/candidates/`, `bindings/candidates/` | Generated selected v1 candidate output, checked by `check:generation`. |
| `forms/source-candidates/` | Generated additional Container source packages, outside the selected publication roster. |
| `forms/releases/` | Content-addressed v1 byte closures, including immutable historical and evidence-only roots. |
| `site/`, `scripts/edge-form-pages*.mjs` | Site configuration and rendering from verified packages and authored specifications. Builds use temporary directories. |

The v1 generator selects one version per Interface/Binding name. It refuses
versions that would share an output path. Retrieve an older exact contract
from its immutable Git source rather than replacing it with the selected version.

`check:integrity` checks the extraction baseline and compares published
Definitions against signed source `e7f8a39311dd011b8467e97e7f300cabb9a6b06c`.
`check:generation` checks selected candidate bytes independently.
`check:edge-form-freeze` verifies the v2 first-add anchors and append-only
history. These checks require Git 2.45 or newer, complete history and the
pinned objects locally; missing history fails without an automatic fetch.
See [local setup](../README.md#work-locally).

## Candidate and design notes

These describe development work, not additional released Forms or Host support.
Host API v2 specifications are linked above; v1 proposals do not amend them.

| Note | Status |
| --- | --- |
| [Actor and Workflow selection](proposals/actor-workflow-source-candidate.md) | The selected unpublished v1 successor closure; generated candidates and release copies exist. |
| [Workflow execution](proposals/workflow-execution-contract.md) | Semantic source used by that selected closure; independent Host execution remains unqualified here. |
| [Actor execution](proposals/actor-execution-contract.md) | Concrete proposal with a facade mapping adopted only in selected source; remaining semantics need independent qualification. |
| [Actor gap analysis](proposals/actor-runtime.md) | Historical analysis pinned to the earlier signed contracts. |
| [Container source readiness](proposals/container-form-source-readiness.md) | Two generated additional source packages, excluded from signing and release planning. |
| [Container Host API v2 candidate](proposals/container-host-v2.md) | Restores the source-only ContainerService lifecycle as Container, with container.http / module-worker.container and forward WorkerVersion/Deployment. Naming agreement and native Host conformance are pending; the frozen inventory is unchanged. |
| [VectorIndex proposal](proposals/vector-index-contract.md) | Development renderer and proposed profile, outside the selected roster. |
| [Aggregate runtime experiment](proposals/runtime-candidate.md) | Actor, Workflow and Vector closure printed by `cmd/runtime-candidate`; no publication input. |

Run the complete read-only repository gate before handing off a final candidate:

```console
bun run check
```

See the owning [AGENTS.md](../AGENTS.md) for workflow and policy; this map does
not redefine those rules.
