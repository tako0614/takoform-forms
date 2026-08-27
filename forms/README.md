# Form inventory

This page lists every Form in the current `edge.forms.takoform.com` family for
JavaScript edge runtimes.
For the model, package format, and repository commands, start with the
[root README](../README.md).

## Versioning

Takoform has exactly two version axes:

| Axis | Meaning |
| --- | --- |
| API/Core SemVer | `1.x` is the public API release line; compatible releases keep the literal `/v1` wire lane. |
| Form definition | Each Form has its own `definitionVersion`; a semantic change creates a new Form identity. |

Other identifiers are pinned metadata, not release clocks.

See the [Core Form Package spec](https://github.com/tako0614/takoform/blob/v1.0.1/spec/form-package/README.md)
and [Core versioning spec](https://github.com/tako0614/takoform/blob/v1.0.1/spec/versioning.md)
for the shared rules.

## Edge Forms

| Kind (definition) | Package index | Role | `definitionVersion` | Intent |
| --- | --- | --- | --- | --- |
| [ModuleWorker](candidates/edge.forms.takoform.com/module-worker/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/module-worker/package-index.json) | identity | `0.1.0` | Worker application identity and handler ABI. |
| [WorkerBundle](candidates/edge.forms.takoform.com/worker-bundle/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-bundle/package-index.json) | revision | `0.1.0` | Points to a module manifest by `manifestDigest`; the manifest inventories bytes. |
| [StaticAssetBundle](candidates/edge.forms.takoform.com/static-asset-bundle/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/static-asset-bundle/package-index.json) | revision | `0.1.0` | Points to an asset manifest by `manifestDigest`; the manifest inventories bytes. |
| [WorkerVersion](candidates/edge.forms.takoform.com/worker-version/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-version/package-index.json) | revision | `0.2.0` | Executable worker snapshot and bindings. |
| [WorkerDeployment](candidates/edge.forms.takoform.com/worker-deployment/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-deployment/package-index.json) | deployment | `0.1.0` | Traffic weights across worker versions. |
| [WorkerCustomDomain](candidates/edge.forms.takoform.com/worker-custom-domain/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-custom-domain/package-index.json) | attachment | `0.1.0` | Attach a custom hostname to a worker. |
| [WorkerEndpoint](candidates/edge.forms.takoform.com/worker-endpoint/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-endpoint/package-index.json) | attachment | `0.1.0` | Give a worker a host-assigned HTTPS address. |
| [WorkerCronTrigger](candidates/edge.forms.takoform.com/worker-cron-trigger/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/worker-cron-trigger/package-index.json) | attachment | `0.1.0` | Run a worker on a UTC cron schedule. |
| [EdgeKVNamespace](candidates/edge.forms.takoform.com/edge-kv-namespace/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/edge-kv-namespace/package-index.json) | identity | `0.1.0` | Eventually consistent byte key/value store. |
| [SQLiteDatabase](candidates/edge.forms.takoform.com/sqlite-database/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/sqlite-database/package-index.json) | identity | `0.1.0` | SQLite database with Edge SQL semantics. |
| [SQLiteMigrationSet](candidates/edge.forms.takoform.com/sqlite-migration-set/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/sqlite-migration-set/package-index.json) | revision | `0.1.0` | Ordered immutable migration files. |
| [SQLiteMigrationApplication](candidates/edge.forms.takoform.com/sqlite-migration-application/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/sqlite-migration-application/package-index.json) | attachment | `0.1.0` | Apply one migration set to one database. |
| [AtLeastOnceQueue](candidates/edge.forms.takoform.com/at-least-once-queue/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/at-least-once-queue/package-index.json) | identity | `0.1.0` | Unordered queue with at-least-once delivery. |
| [QueueConsumer](candidates/edge.forms.takoform.com/queue-consumer/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/queue-consumer/package-index.json) | attachment | `0.1.0` | Worker batch consumer and retry policy. |
| [DurableWorkflow](candidates/edge.forms.takoform.com/durable-workflow/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/durable-workflow/package-index.json) | identity | `0.1.0` | Durable replayed workflow class. |
| [ActorNamespace](candidates/edge.forms.takoform.com/actor-namespace/definition.json) | [package-index.json](candidates/edge.forms.takoform.com/actor-namespace/package-index.json) | identity | `0.1.0` | Addressable actor class and ID space. |

## Package identity

For a verified `package-index.json`, Core v1.0.1's `PublicationLocatorFor`
derives `releaseId` from the FormRef group and kind and `artifactId` from
`packageDigest` (`sha256:` becomes `sha256-`), then combines them into the
content-addressed path and tag:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

See the [root README](../README.md#preparing-and-publishing-packages) for
generation, verification, and publication commands. Tags provide unsigned
Git provenance; official and external packages use the same bytes and
verification semantics.
