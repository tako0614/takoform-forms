# Portable actor execution — proposal

Status: **unaccepted, non-normative draft**, updated 2026-09-08. No version is allocated.
This document records a gap and a proposed decision boundary; it does not add
meaning to an existing Form, Interface, or Binding. It is outside all candidate
and release package closures. The existing API v1 and published bytes remain
unchanged. A Host must not advertise actor support on the strength of this draft.
The current per-actor serialization guarantee remains the implementation target.
The earlier proposal to weaken that guarantee or introduce an independent
ActorDeployment is withdrawn: ordinary native code replacement was too narrow
a backend assumption. A stable supervisor with replaceable child execution is
another implementation candidate, not yet a verified conforming backend.

The [concrete execution proposal](actor-execution-contract.md) now supplies
class/context signatures, private SQL and alarm settlement, a producer-drain
admission model, and sockets retained outside replaceable class contexts.
It is still unaccepted and outside every release closure; the evidence and
unresolved published-contract boundary below remain applicable.

## What is missing

The current contracts define actor identity and useful execution semantics, but
not enough of the JavaScript class interface for independent Hosts to run the
same actor bundle:

| Exact current contract | Defined | Missing for execution |
| --- | --- | --- |
| [ActorNamespace 0.1.0](../../forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2bmn2g64somfwwk43qmfrwk/sha256-eb174b666890ef630e82746374c8a91aec8692c64e98e82dc580a85d17307743/definition.json) | Immutable class/worker relation, per-namespace IDs, active deployment code, class export in every weighted version | Valid executable class shape, constructor arguments/types and inspection fixture |
| [worker.actor 1.0.0](https://github.com/tako0614/takoform-forms/blob/e7f8a39311dd011b8467e97e7f300cabb9a6b06c/interfaces/candidates/v1alpha1/worker.actor/definition.json) | Serialized invocation, private SQL storage, one durable alarm, eviction, HTTP-shaped invocation | JavaScript context/storage/alarm shape, method receiver/signatures, environment, context lifetime and WebSocket execution |
| [module-worker.actor 1.0.0](https://github.com/tako0614/takoform-forms/blob/e7f8a39311dd011b8467e97e7f300cabb9a6b06c/bindings/candidates/v1alpha2/module-worker.actor/definition.json) | Caller `idFromName`, `newUniqueId`, `get(id).fetch(...)`; intentionally no private callee context | Caller-visible WebSocket upgrade semantics; a changed callee Interface identity also needs an exact-ref update |
| [worker.runtime 1.1.0](https://github.com/tako0614/takoform-forms/blob/e7f8a39311dd011b8467e97e7f300cabb9a6b06c/interfaces/candidates/v1alpha1/worker.runtime/definition.json) | Default plain-object `fetch`, `scheduled`, `queue` handlers and their environment | Actor class ABI and WebSocket events |

This analysis is fixed to publisher set/source commit
`e7f8a39311dd011b8467e97e7f300cabb9a6b06c`, not future candidate contents.
The ActorNamespace schema digest for this analysis is
`sha256:3669f3bebf7b4c6169d5a3cdf7e7ca4e8d1851516702a437be6ef69c2b9b98d7`.
Its [package index](../../forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2bmn2g64somfwwk43qmfrwk/sha256-eb174b666890ef630e82746374c8a91aec8692c64e98e82dc580a85d17307743/package-index.json)
contains data-only fixtures, not an executable actor class fixture. Description
prose and abstract operation fixtures do not determine constructor arity, a
storage method's JavaScript shape, or WebSocket callback arguments.

Cloudflare's `DurableObjectState` is one implementation API, not an implicit
definition of these missing portable surfaces. Equally, an abstract SQL
operation is not a license to choose a private SQL or KV JavaScript API.

## Consumer evidence, not consumer-specific contracts

The requirements were checked against Takos source at commit
`07eac21611728890b6426c0e7ce9f8105f0d134e` and Yurucommu source at
`a4e23de816db47ade69542837326606e43cd0650`, consuming published
`@takosjp/yurucommu-core@4.1.5`:

- Takos has five actor classes. Session, routing and rate limiting need
  persistent private state, serialized updates, HTTP calls and alarms.
  Notification/run notifiers additionally need WebSocket upgrade, connection
  state, hibernation callbacks and version-scoped bindings.
- Yurucommu Core has realtime and call-signaling classes. They need private
  one-time tickets, persistent state, HTTP invocation, WebSocket connections and
  callbacks; signaling also needs alarms and database/environment bindings.
  The inspected Yurucommu entrypoint and deployment modules do not bind those
  classes. Its REST/polling/503 behavior is not actor interoperability evidence.
- These implementations use KV-shaped state and Cloudflare-shaped class APIs.
  The current portable actor contract instead describes private SQL. A future
  app adapter may translate app-owned state access to an agreed portable API;
  no Host should recognize an app name or copy its private methods as a contract.

The target is one generic contract implemented independently by self-host and
managed runtimes. It is not an obligation to reproduce every Cloudflare method.

## Preserve the current resource relationships

ActorNamespace retains its immutable worker/class relation and stable durable
identity. Its code comes from that worker's active weighted WorkerDeployment;
every active weighted WorkerVersion must export the declared class. An
implementation must not silently replace that rule with an independent actor
activation resource or attach new storage to every WorkerVersion.

The current contract requires exclusion per actor ID. It does **not** require
every ID in a namespace to switch code simultaneously, a global inventory of
IDs, or a namespace-wide instantaneous execution barrier. Those were additional
assumptions in the earlier draft, not missing publisher requirements.

Backend selection, context reuse and transition need to be designed against
the actual weighted rule. A candidate may pin a selected immutable version and
its declared environment for an invocation and perform a per-ID handoff before
using another version. This sketch does not settle alarm, socket or context
lifetime rules that the current executable contract leaves undefined. It is
not permission to choose a different public code-selection rule.

### Class interface under consideration

The preferred interface shape is a normal named JavaScript class, with no native
base class or Cloudflare import required. The following is an illustrative
candidate, **not an executable conformance fixture or a selected ABI**:

```javascript
export class Counter {
  constructor(context, env) {
    this.context = context;
    this.env = env;
  }

  async start() {
    await this.context.storage.execute(
      "CREATE TABLE IF NOT EXISTS counter (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)",
    );
  }

  async fetch(request) {
    const result = await this.context.storage.execute(
      "INSERT INTO counter VALUES (1, 1) ON CONFLICT(id) DO UPDATE SET value = value + 1 RETURNING value",
    );
    return Response.json(result.rows[0]);
  }
}
```

The candidate keeps a synchronous constructor for capturing context/environment,
with an awaited optional `start()` for state initialization before any delivered
event, including after eviction. Handlers use the instance as their receiver.
Storage would expose `execute`, `query`, and `transaction` with the existing
EdgeSqlValue/results model, while permitting actor-owned schema initialization.
It would not expose a second native KV or SQL API. Alarm and socket lifecycle
would be separate small context surfaces; neither exposes Host-native state.

The final signatures, transaction/isolation rules, initialization failure,
event deadlines and socket representation still need to be specified together.
In particular, native input-gate behavior at an `await` is not proof of the
published whole-invocation serialization guarantee. Do not implement or claim
support based on the sketch alone.

### Lifecycle: an implementation gap, not a weaker-guarantee choice

The current `worker.actor` wording promises at most one live execution context
per ID and one complete invocation at a time, including across process death.
An in-instance queue alone does not prove exclusion across runtime generations.
The HTTP operation already specifies completion at the response head and
streaming bodies in both directions. Do not relabel that completion point as
undefined. The remaining lifetime questions concern response-body producers,
cancellation, background work and callbacks sharing a live execution context.

Cloudflare's [shutdown and code-update contract](https://developers.cloudflare.com/durable-objects/concepts/durable-object-lifecycle/)
allows old in-flight requests without storage access to finish while new
requests reach the replacement. Code propagation is eventual. Thus A can await
external HTTP, B can begin on the replacement, and A can finish a storage-free
tail afterward. Native storage fencing therefore does not establish exclusion
for an invocation which is still awaiting its response head. This is a
source-backed objection to ordinary native code replacement, not a reproduced
managed integration run and not a proof that every backend design fails.

Cloudflare's [Durable Object Facets](https://developers.cloudflare.com/dynamic-workers/usage/durable-object-facets/)
provide a different candidate: a stable supervisor DO owns a named child facet,
whose class can come from a Dynamic Worker. `ctx.facets.abort(name)` shuts down
that facet and invalidates its stubs while preserving its storage. A subsequent
`get` can start it with a different class; the documentation explicitly presents
this as a code-update mechanism. Unlike replacing the supervisor's own script,
this may allow the supervisor to control a per-ID execution handoff.

The candidate is to serialize admission in the stable owner, pin the selected
version/environment, and retire the old child before admitting another version
for the same actor ID. IDs can transition independently. It still needs real
evidence for child/context termination, alarm delivery, streaming/cancellation,
hibernating sockets, failure during handoff and Host carrier upgrades. Facets
documentation is not an implemented adapter or proof of the complete contract.

No weaker lifecycle or independent ActorDeployment is selected or requested by
this draft. Preserve the current guarantee while testing this candidate. Keep
genuine missing ABI/lifetime decisions separate from backend feasibility.

## Decisions required together before implementation

1. **Executable class ABI.** Define named export validation, constructor
   arguments and exact types, method receiver/signatures, initialization
   failures, and the private actor context. Choose a concrete portable storage
   API consistent with its declared SQL/value/transaction semantics. State which
   initialization/concurrency helpers exist rather than inheriting native ones.
2. **Invocation and lifetime.** Preserve the declared response-head completion
   for HTTP and the current per-ID exclusion. Specify the remaining context
   lifetime of streaming producers, cancellation and background work, and the
   executable alarm handler/retry boundary. State when contexts can be evicted
   and what survives restart; do not weaken the current guarantee to fit a
   particular native runtime.
3. **WebSocket scope.** Decide the upgrade/response representation, message,
   close and error callbacks, connection identity and recoverable metadata,
   hibernation/reconstruction, limits and backpressure. Define how events share
   serialization with HTTP/alarm events. A connected socket must not silently
   become an invocation that prevents all later actor events forever.
4. **Stable state identity.** Keep the namespace resource incarnation and opaque
   actor ID independent of executable revisions and backend script names.
   Deployment updates/rollback must not fork storage or alarms. Define namespace
   replacement/deletion and retention separately from context eviction.
5. **Code selection and environment.** Preserve the active weighted
   WorkerDeployment rule. Resolve the genuinely unspecified context
   reuse/replacement, open-socket and alarm code-selection behavior together.
   Use only the selected version's declared environment in its context, and
   validate every active weighted version's class. Backend convenience alone
   does not justify a new activation resource or a changed existing identity.
6. **Exact identity closure.** Identify which new Form/Interface/Binding
   definitions are needed. Changed callee semantics or exact target references
   may require a forward actor interface/binding and WorkerVersion binding
   closure, not only a new ActorNamespace definition. Do not allocate versions
   or change Core/API v1 to compensate for an unfinished runtime definition.

These decisions belong in the owning published contracts if accepted, not in a
Takoserver README or an app's installation helper. The interface sketch and
backend model above are design candidates, not accepted signatures, method
names, storage representations, or release numbers.

## Backend feasibility constraint

Portable identity cannot depend on a managed runtime's physical Worker script.
A managed backend using a different script per immutable WorkerVersion must not
attach an independent actor namespace to each revision and call that one stable
namespace. A stable execution carrier or another implementation needs its own
backend design, including fencing against concurrent live contexts and version
transition behavior. Those are Host implementation details, not public cloud
resource fields added to the Form.

The managed ModuleWorker remains a genuine WfP user Worker: its fetch, queue
and scheduled execution must not be silently moved out of the dispatch
namespace. An ActorNamespace is a separate resource whose native carrier may
be owned by the private Host. A Host-owned supervisor using Dynamic Worker
facets is a candidate internal implementation of that resource, not a reason
to add cloud-specific fields to its Form. It must receive only the exact
selected module graph and declared environment, never the supervisor's
operator capabilities. WfP DO bindings and Dynamic Worker Facets each being
documented does not prove the combined integration. In particular, the
inspected WfP upload binding schema does not establish a Worker Loader binding
inside a user Worker; do not assume that topology works.

The self-host implementation likewise needs a stable namespace storage key
and the selected graph under the owner-pinned workerd runtime. Bounded local
probes on 2026-09-07 now establish native Facet handoff on that artifact, not
the complete portable Actor lifecycle. The binary digest is
`sha256:c00638f195e4a9fda4bafb07bb7b1674e4d8324d0072efbf0ea57beb0ff08e52`.
The first probe demonstrated native SQL/state identity
across a process restart, a delivered alarm, and WebSocket messages through
the native `acceptWebSocket` API. A follow-up on the same open socket, after
12 seconds idle, observed a new constructor generation and the same serialized
attachment. The socket remained usable; no process restart or reconnect occurred
during that interval. This proves one native hibernation/reconstruction path,
not code-update continuity. A later extension proves native KV/SQL separation
and a pending alarm surviving forced process termination: the old child exits
before the scheduled time, then the restarted same-binary runtime delivers the
distinct alarm. The fixture has no inherited operator environment or outbound
network. This still does not prove machine power-loss durability, storage-format
upgrades, distributed fencing, this proposed class ABI or Takoserver Actor
support.

Two later probes, still outside any published Form closure, narrow the handoff
feasibility gap:

- A native supervisor holds a second invocation for the same ID until the first
  response head, while another ID proceeds. A same-version invocation can run
  while the preceding response body remains open. Tail-free A→B→A replacement
  retains ID/SQL state, changes code/environment markers and invalidates old
  stubs. This is a native scheduling mechanism, not an accepted class ABI; the
  Dynamic Worker loader does not expose the candidate's closed-module-graph
  provenance roles.
- A static-class Facet probe uses the existing application/Host-private module
  roles instead of that dynamic loader. Declared imports work while builtin,
  Host-private and missing imports fail, including generated dynamic imports.
  Tail-free A→B→A again retains ID/SQL state and invalidates old stubs. It does
  not prove concurrent handoff with live response producers, alarm/socket
  continuity, production environment isolation or a managed WfP adapter.

The corresponding local test source digests are
`sha256:d8efb6de8227376409f85e5720c15fa2bcef3b4de8659679acef35544f11edb8`
and `sha256:717259fd4f2d6cf24466a788ae2a56a9c6e75fddb0031e20c7736cb22cc5af90`.
These evidence references neither allocate a runtime contract version nor make
the proposed signatures normative. The class/lifetime decisions above remain
required before implementing or advertising portable Actor support.

The [pinned upstream runtime schema](https://github.com/cloudflare/workerd/blob/0129b1e7aaf9afbc21cba79d215723d9839eb7a0/src/workerd/server/workerd.capnp)
marks local-disk DO storage experimental and local to one runtime instance.
The self-host design must therefore own a single active runtime for each
namespace's storage and preserve its stable `uniqueKey`. This probe proves
neither multi-instance coordination nor storage compatibility across a workerd
upgrade; those cannot be inferred from same-binary restart persistence.

Primary-source feasibility checks on 2026-09-07:

- [WfP bindings](https://developers.cloudflare.com/cloudflare-for-platforms/workers-for-platforms/configuration/bindings/)
  and the [WfP API](https://developers.cloudflare.com/api/resources/workers_for_platforms/subresources/dispatch/)
  provide DO bindings to user Workers, including external script bindings.
- [DO migrations](https://developers.cloudflare.com/durable-objects/reference/durable-objects-migrations/)
  distinguishes initial namespace provisioning from later code updates on the
  same script/export. Namespace deletion is not code rollback.
- [Dynamic Worker Facets](https://developers.cloudflare.com/dynamic-workers/usage/durable-object-facets/)
  document named child execution, separate SQLite storage, and abort/restart
  with a replacement class. The supervisor's own code lifecycle is separate.
- [WebSocket lifecycle](https://developers.cloudflare.com/durable-objects/best-practices/websockets/)
  says code deployment restarts DOs and disconnects their sockets. Ordinary
  hibernation and deployment continuity must not be conflated.

This is API/source feasibility evidence, not proof from an actual WfP deployment.
Mutable code replacement still needs a single Host writer, predecessor-aware
settlement and actual script/settings/bindings readback after lost acknowledgments.
Do not assume an upstream compare-and-swap upload API exists. Preserve these
controls in the existing provider execution/recovery owner, not a second ledger.

The self-host and managed implementations must demonstrate the same accepted
class/lifetime semantics before support is advertised. Success on one backend
does not authorize a weaker interpretation on the other. In particular, exact
upload readback and tail-free native handoff do not establish the complete
per-ID lifecycle across live streams, alarms and sockets, or resolve the missing
portable executable contract.

## Small executable conformance slice

After the decisions above, author a generic executable actor bundle and expected
behavior as a separate Host conformance artifact, outside the data-only Form
Package closure. Core verification does not execute tenant JavaScript and this
proposal does not change the Form Package format.
Use the same bundle on independent backends to cover:

- deterministic addressing, namespace separation and invalid class admission;
- concurrent increment, atomic rollback, restart persistence and alarm retry;
- streaming invocation, cancellation and callee errors;
- WebSocket messages, hibernation/reconstruction, alarms while sockets are open;
- code update/rollback preserving actor state, with exact environment selection;
- replacement/delete and the agreed retention behavior.

Develop with the focused failing case, then run the owner's complete gate once
the changed slice is stable. Do not duplicate app-wide E2E for each runtime edit.
Only after exact contracts and backend behavior agree should Takos/Yurucommu
adapt their declarations and run full interoperability tests. Disabled features
or a native-only remainder do not satisfy that result.

## Non-actions

This proposal does not modify generated catalogs, fixtures, release artifacts,
signatures, trust sets, tags, Provider support, Host admission, or deployed
resources. It introduces no Specification release stream and changes no
published identity. Publication and actual Host support remain separate work.
