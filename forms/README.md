# Form inventory

The Host API v2 specifications and the selected v1 packages have separate
identities. Use the [reader guide](../spec/guides/index.md) to choose a Form;
use the tables below to inspect its exact source. Repository setup is in the
[root README](../README.md#work-locally).

## Host API v2 specifications

These 21 Japanese normative documents cover 17 resource kinds and are anchored in
[`spec/forms.freeze.json`](../spec/forms.freeze.json). Frozen source is
eligible for version-fixed URL publication; it does not prove public
availability, Provider compatibility or Host support. The trailing slash in
`https://edge.forms.takoform.com/forms/<Kind>/<version>/` is part of the
identity. V1 packages below are not aliases for these URLs.

| Kind | Version | Normative source |
| --- | --- | --- |
| ActorNamespace | `0.3.0` | [Specification](../spec/forms/ActorNamespace/0.3.0/index.md) |
| AtLeastOnceQueue | `0.2.0` | [Specification](../spec/forms/AtLeastOnceQueue/0.2.0/index.md) |
| DurableWorkflow | `0.3.0` | [Specification](../spec/forms/DurableWorkflow/0.3.0/index.md) |
| EdgeKVNamespace | `0.2.0` | [Specification](../spec/forms/EdgeKVNamespace/0.2.0/index.md) |
| ModuleWorker | `0.3.0` | [Specification](../spec/forms/ModuleWorker/0.3.0/index.md) |
| ObjectBucket | `0.2.0` | [Specification](../spec/forms/ObjectBucket/0.2.0/index.md) |
| QueueConsumer | `0.3.0` | [Specification](../spec/forms/QueueConsumer/0.3.0/index.md) |
| SQLiteDatabase | `0.2.0` | [Specification](../spec/forms/SQLiteDatabase/0.2.0/index.md) |
| SQLiteMigrationApplication | `0.2.0` | [Specification](../spec/forms/SQLiteMigrationApplication/0.2.0/index.md) |
| SQLiteMigrationSet | `0.2.0` | [Specification](../spec/forms/SQLiteMigrationSet/0.2.0/index.md) |
| StaticAssetBundle | `0.2.0` | [Specification](../spec/forms/StaticAssetBundle/0.2.0/index.md) |
| WorkerBundle | `0.2.0` | [Specification](../spec/forms/WorkerBundle/0.2.0/index.md) |
| WorkerCronTrigger | `0.3.0` | [Specification](../spec/forms/WorkerCronTrigger/0.3.0/index.md) |
| WorkerCustomDomain | `0.3.0` | [Specification](../spec/forms/WorkerCustomDomain/0.3.0/index.md) |
| WorkerDeployment | `0.4.0` | [Specification](../spec/forms/WorkerDeployment/0.4.0/index.md) |
| WorkerEndpoint | `0.3.0` | [Specification](../spec/forms/WorkerEndpoint/0.3.0/index.md) |
| WorkerVersion | `0.5.0` | [Specification](../spec/forms/WorkerVersion/0.5.0/index.md) |

### SQLite migration 0.3.0 source successors

The following new identities are source-only; this change does not publish
their URLs or establish Host support. The 0.2.0 specifications above retain
their original bytes and behavior.

| Kind | Version | Normative source |
| --- | --- | --- |
| SQLiteMigrationApplication | `0.3.0` | [Specification](../spec/forms/SQLiteMigrationApplication/0.3.0/index.md) |
| SQLiteMigrationSet | `0.3.0` | [Specification](../spec/forms/SQLiteMigrationSet/0.3.0/index.md) |

Application 0.3.0 permits only `PRAGMA defer_foreign_keys = ON|OFF|TRUE|FALSE|1|0`
among PRAGMAs, with ASCII case-insensitive spelling and an optional final
semicolon. Other PRAGMAs, `foreign_keys` and read-only forms remain denied.
Set 0.3.0 retains bytes and references that execution contract; it does not
validate or execute SQL. See the [source conformance corpus](../conformance/host-v2/sqlite-migration/README.md)
for accepted and rejected inputs and the limits of local verification. Host
adoption and hosted E2E recovery require separate implementation evidence.

### SQLite migration 0.4.0 source successors

These source-only identities preserve the 0.3.0 `defer_foreign_keys`
assignment and add read-only schema introspection. The 0.2.0 and 0.3.0
specifications retain their original bytes and behavior.

| Kind | Version | Normative source |
| --- | --- | --- |
| SQLiteMigrationApplication | `0.4.0` | [Specification](../spec/forms/SQLiteMigrationApplication/0.4.0/index.md) |
| SQLiteMigrationSet | `0.4.0` | [Specification](../spec/forms/SQLiteMigrationSet/0.4.0/index.md) |

Application 0.4.0 allows `table_info`, `foreign_key_list`, `index_list` and
`index_info` as read-only PRAGMA statements and their `pragma_*` table-valued
forms, plus reads of `main.sqlite_master`/`main.sqlite_schema`. Dynamic object
arguments and correlated queries are supported. Other PRAGMAs and their
table-valued forms, configuration writes, direct catalog writes and Host
ledger access remain denied. Ordinary authorized DDL still updates its
catalog through SQLite. Set continues to own byte custody; SQLiteDatabase
0.2.0's Worker Binding restrictions are unchanged.

The [introspection guide](../docs/sqlite-migration-introspection.md) explains
main-only scope, shareable vectors and the local verification boundary.
Source changes do not publish either Form or prove Host implementation or
hosted E2E recovery.

## Independent version tracks

These versions answer different questions and do not move in lockstep:

| Track | What it versions |
| --- | --- |
| Host API protocol | The shared request/response contract: v2 for the URL-identified specifications above, and the retained `/v1` lane for packages below. See the [Host API v2 reference](https://takoform.com/spec/host-api/v2/http). |
| Form definition | Each Form has its own version: a version-fixed URL in v2, or `definitionVersion` in v1. A semantic change requires a new identity. |
| Go client library | The `github.com/tako0614/takoform` module release, published separately from the API protocol and individual Forms. |
| Terraform/OpenTofu Provider | The provider's own release stream for its Form mappings. |

There is no separate `Specification` SemVer that synchronizes these tracks.
Equal version numbers, when they occur, do not imply a coordinated release.
Other identifiers such as package digests, schema digests and publication tags
pin specific content; they are not release clocks.

See the [Core Form Package spec](https://github.com/tako0614/takoform/blob/v1.1.0/spec/form-package/README.md)
and [Core versioning spec](https://github.com/tako0614/takoform/blob/v1.1.0/spec/versioning.md)
for the shared rules.

## Selected v1 source packages

The generator selects 17 Forms, 8 Interfaces and 7 Bindings. Nine Form
successors, three Interface successors and two Binding successors remain
unsigned and unpublished. This source selection is not the earlier signed
17-package roster used by the site build. The table lists the selected
candidate Definitions, not Host API v2 specifications or installed Host resources.

| Kind (definition)                                                                                             | Package index                                                                                            | Role       | `definitionVersion` | Intent                                                                           |
| ------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ---------- | ------------------- | -------------------------------------------------------------------------------- |
| [ModuleWorker](candidates/edge.forms.takoform.com/module-worker/definition.json)                              | [package-index.json](candidates/edge.forms.takoform.com/module-worker/package-index.json)                | identity   | `0.2.0`             | Worker application identity and handler ABI.                                     |
| [WorkerBundle](candidates/edge.forms.takoform.com/worker-bundle/definition.json)                              | [package-index.json](candidates/edge.forms.takoform.com/worker-bundle/package-index.json)                | revision   | `0.1.0`             | Points to a module manifest by `manifestDigest`; the manifest inventories bytes. |
| [StaticAssetBundle](candidates/edge.forms.takoform.com/static-asset-bundle/definition.json)                   | [package-index.json](candidates/edge.forms.takoform.com/static-asset-bundle/package-index.json)          | revision   | `0.1.0`             | Points to an asset manifest by `manifestDigest`; the manifest inventories bytes. |
| [WorkerVersion](candidates/edge.forms.takoform.com/worker-version/definition.json)                            | [package-index.json](candidates/edge.forms.takoform.com/worker-version/package-index.json)               | revision   | `0.4.0`             | Executable worker snapshot and bindings.                                         |
| [WorkerDeployment](candidates/edge.forms.takoform.com/worker-deployment/definition.json)                      | [package-index.json](candidates/edge.forms.takoform.com/worker-deployment/package-index.json)            | deployment | `0.3.0`             | Traffic weights across worker versions.                                          |
| [WorkerCustomDomain](candidates/edge.forms.takoform.com/worker-custom-domain/definition.json)                 | [package-index.json](candidates/edge.forms.takoform.com/worker-custom-domain/package-index.json)         | attachment | `0.2.0`             | Attach a custom hostname to a worker.                                            |
| [WorkerEndpoint](candidates/edge.forms.takoform.com/worker-endpoint/definition.json)                          | [package-index.json](candidates/edge.forms.takoform.com/worker-endpoint/package-index.json)              | attachment | `0.2.0`             | Give a worker a host-assigned HTTPS address.                                     |
| [WorkerCronTrigger](candidates/edge.forms.takoform.com/worker-cron-trigger/definition.json)                   | [package-index.json](candidates/edge.forms.takoform.com/worker-cron-trigger/package-index.json)          | attachment | `0.2.0`             | Run a worker on a UTC cron schedule.                                             |
| [EdgeKVNamespace](candidates/edge.forms.takoform.com/edge-kv-namespace/definition.json)                       | [package-index.json](candidates/edge.forms.takoform.com/edge-kv-namespace/package-index.json)            | identity   | `0.1.0`             | Eventually consistent byte key/value store.                                      |
| [ObjectBucket](candidates/edge.forms.takoform.com/object-bucket/definition.json)                              | [package-index.json](candidates/edge.forms.takoform.com/object-bucket/package-index.json)                | identity   | `0.1.0`             | Strongly consistent streaming object store fixed by `edge.objects`.              |
| [SQLiteDatabase](candidates/edge.forms.takoform.com/sqlite-database/definition.json)                          | [package-index.json](candidates/edge.forms.takoform.com/sqlite-database/package-index.json)              | identity   | `0.1.0`             | SQLite database with Edge SQL semantics.                                         |
| [SQLiteMigrationSet](candidates/edge.forms.takoform.com/sqlite-migration-set/definition.json)                 | [package-index.json](candidates/edge.forms.takoform.com/sqlite-migration-set/package-index.json)         | revision   | `0.1.0`             | Ordered immutable migration files.                                               |
| [SQLiteMigrationApplication](candidates/edge.forms.takoform.com/sqlite-migration-application/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/sqlite-migration-application/package-index.json) | attachment | `0.1.0`             | Apply one migration set to one database.                                         |
| [AtLeastOnceQueue](candidates/edge.forms.takoform.com/at-least-once-queue/definition.json)                    | [package-index.json](candidates/edge.forms.takoform.com/at-least-once-queue/package-index.json)          | identity   | `0.1.0`             | Unordered queue with at-least-once delivery.                                     |
| [QueueConsumer](candidates/edge.forms.takoform.com/queue-consumer/definition.json)                            | [package-index.json](candidates/edge.forms.takoform.com/queue-consumer/package-index.json)               | attachment | `0.2.0`             | Worker batch consumer and retry policy.                                          |
| [DurableWorkflow](candidates/edge.forms.takoform.com/durable-workflow/definition.json)                        | [package-index.json](candidates/edge.forms.takoform.com/durable-workflow/package-index.json)             | identity   | `0.2.0`             | Durable replayed workflow class.                                                 |
| [ActorNamespace](candidates/edge.forms.takoform.com/actor-namespace/definition.json)                          | [package-index.json](candidates/edge.forms.takoform.com/actor-namespace/package-index.json)              | identity   | `0.2.0`             | Addressable actor class and ID space.                                            |

## V1 package history

[`scripts/form-publication.mjs`](../scripts/form-publication.mjs) derives the
complete local `forms/releases/` closure from the selected candidates,
verified signed sets and pinned history inventories:

| Roots | Source | Publication boundary |
| --- | --- | --- |
| 17 selected | Current family index and candidate packages | Eight match the signed roster; nine are unsigned Actor/Workflow successors. |
| 11 retained | Signed history plus [`retained-packages.json`](retained-packages.json) | Preserve exact bytes and package tags. The explicit inventory contains two roots. |
| 3 abandoned | [`abandoned-prepublication.json`](trust/abandoned-prepublication.json) | Evidence-only; neither their package tags nor the abandoned set tag may be published. |
| 9 prior-source | [`source-history.json`](source-history.json) | Unsigned source history, checked against commit `85b2f755a0cf104caf1ae8cb3738e475d55fe988`; excluded from signing and package tags. |
| **40 total** | Complete checked-in release tree | This total is not a signed set, release version or Host support count. |

The earlier signed roster and the two explicitly retained roots account for
19 package identities. A signed successor for this selected source would add
nine new package tags, for 28 publishable roots. The abandoned and prior-source
roots remain excluded. Confirm public availability through anonymous readback
rather than inferring it from the local tree.

For v1, Core v1.1.0's `PublicationLocatorFor` maps the FormRef and package digest
to immutable paths and tags:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

See [package publication](../docs/publishing.md) for generation, signing,
installation, publication and repair. Each publisher uses the same Core API
v1 verification rules. Package publication is separate from Host trust,
installation, support and activation.
