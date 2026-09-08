# Actor execution contract — concrete proposal

Status: **unaccepted, non-normative proposal**, 2026-09-08. No version is
allocated. This is a proposed forward contract, not an interpretation of an
existing identity, an executable Host implementation, or a release artifact.
The [gap analysis](actor-runtime.md) fixes the existing published evidence.
API v1 and all published Form, Interface and Binding bytes remain unchanged.

## Decision proposed

Use an ordinary named JavaScript class, private SQL, one alarm slot and
Host-owned socket transport. Keep ordinary streaming HTTP responses. Do not
add a native base class, a second KV API, an ActorDeployment, arbitrary public
RPC methods or a new stream factory.

HTTP call completion and permission to start the next event are separate:
the caller receives the response head immediately; later events for the same
actor wait until actor-owned response production and execution have finished
safely, and request consumption has ended or been cancelled.
This is a new lifetime/liveness rule to adopt explicitly. It does not redefine
the existing HTTP completion point as body EOF.

## Proposed JavaScript surface

These declarations illustrate the exact proposed shape; they are not an
allocated Interface identity or a published TypeScript package. `EdgeSqlValue`
and SQL result semantics retain the existing actor SQL domain.

```typescript
type EdgeSqlValue =
  | null
  | number
  | string
  | { encoding: "base64"; data: string };

interface SqlStatement {
  readonly sql: string;
  readonly params?: readonly EdgeSqlValue[];
}

interface SqlResult {
  readonly rows: readonly Readonly<Record<string, EdgeSqlValue>>[];
  readonly rowsWritten: number;
}

interface ActorStorage {
  execute(sql: string, params?: readonly EdgeSqlValue[]): Promise<SqlResult>;
  query(sql: string, params?: readonly EdgeSqlValue[]): Promise<SqlResult>;
  transaction(statements: readonly SqlStatement[]): Promise<{
    readonly results: readonly SqlResult[];
  }>;
}

interface ActorAlarm {
  set(atMillis: number): Promise<void>;
  get(): Promise<number | null>;
  clear(): Promise<void>;
}

interface ActorSocket {
  readonly id: string;
  send(data: string | Uint8Array): void;
  close(code?: number, reason?: string): void;
  getAttachment(): Promise<Uint8Array | null>;
  setAttachment(value: Uint8Array | null): Promise<void>;
}

// Created only by the Host. Matching properties cannot forge an upgrade.
interface ActorUpgrade {
  readonly status: 101;
  readonly headers: Headers;
}

interface ActorSockets {
  accept(
    request: Request,
    options?: { protocol?: string; attachment?: Uint8Array },
  ): Promise<{ response: ActorUpgrade; socket: ActorSocket }>;
  get(id: string): Promise<ActorSocket | null>;
  list(): Promise<readonly ActorSocket[]>;
}

interface ActorContext {
  readonly id: string;
  readonly storage: ActorStorage;
  readonly alarm: ActorAlarm;
  readonly sockets: ActorSockets;
}

interface ActorTurn {
  readonly signal: AbortSignal;
}

interface ActorInstance {
  start?(turn: ActorTurn): void | Promise<void>;
  fetch(
    request: Request,
    turn: ActorTurn,
  ): Response | ActorUpgrade | Promise<Response | ActorUpgrade>;
  alarm(turn: ActorTurn): void | Promise<void>;
  socketMessage(
    socket: ActorSocket,
    data: string | Uint8Array,
    turn: ActorTurn,
  ): void | Promise<void>;
  socketClose(
    socket: ActorSocket,
    event: { code: number; reason: string; wasClean: boolean },
    turn: ActorTurn,
  ): void | Promise<void>;
}

type ActorConstructor = new (
  context: ActorContext,
  env: Readonly<Record<string, unknown>>,
) => ActorInstance;
```

### Class construction and initialization

The exact named export must be constructable. The four required event handlers
and optional `start` are callable prototype methods, including inherited application
methods; accessors and per-instance handler replacement are not the proposed
portable shape. Extra application helpers do not become public RPC endpoints.
Every weighted Version must supply the declared class. Class inspection runs
without real actor state, sensitive environment or an operator capability;
it must not instantiate a live actor as a readiness test.

Requiring the complete event-handler surface is a deliberate first-contract
choice. An application that does not use alarms or sockets can supply no-op
handlers; the Host does not invent them. Every weighted Version and every later
replacement must retain this surface. This avoids dispatching an existing alarm
or live socket to a Version missing its handler, without a per-ID capability
inventory, selective weight renormalization or an implicit fallback Version.

The constructor captures context and environment synchronously. It must not
start asynchronous work. The Host then calls and awaits optional `start`, with
the instance as receiver, before delivering the event that created the
context. All handlers use that same receiver. `start` runs again after eviction
and must be idempotent; it is not a once-per-durable-ID migration callback.
The Host may evict between every event, so heap fields are never durable state.

`env` contains only the selected Version's declared variables, sensitive slots
and satisfied bindings under their declared names. It does not acquire another
Version's bindings, native state, namespace handles or operator credentials.
Context and facade properties are Host-created, closed and non-replaceable.

For HTTP, user constructor/start/fetch exceptions after execution starts produce
a complete 500 response. No deployment, an unavailable class or inability to
begin execution rejects as `backend_unavailable`. Neither outcome causes an
automatic retry against a different weighted Version. Alarm initialization
failure retains the delivery obligation; socket initialization failure follows
the failed-callback connection-close rule. Those events have no HTTP response.

### Private SQL

The proposed method arguments/results follow the existing JavaScript SQL
projection, except that actor-owned schema statements remain allowed. Number
values must be finite and within the safe-integer magnitude; boolean, bigint,
ArrayBuffer and typed-array SQL values are not silently converted. Blobs keep
the canonical encoded-bytes object. There is no last-insert metadata.

`query` materializes in an always-rolled-back transaction and returns
`rowsWritten: 0`. `transaction` accepts 1–100 statements, materializes all
results before one commit and rolls back the whole call on failure. It does
not accept an async callback. Existing statement, parameter, row, value and
storage bounds remain; a callback is not implicitly one SQL transaction.
Transaction-control SQL and access to Host storage remain prohibited.

The private store is keyed by namespace incarnation and opaque actor ID, never
by Version, class instance, native script spelling or selected environment.
Context eviction and code rollback retain it. Namespace deletion/replacement
is a different resource lifecycle and must not reuse the old incarnation.

### Alarm settlement

Proposed explicit rule: there is one pending time slot and at most one admitted
delivery obligation for the actor. `get` reads the pending time, not the
currently running attempt. At admission, the due slot becomes that obligation.
Only normal handler completion settles it; throw, deadline or process loss
retains it for retry. No native provider retry-count cap is adopted silently.

While the obligation runs or awaits retry, `set` replaces its one pending
successor and `clear` removes that successor. Neither operation acknowledges
the running obligation. Thus `set(newTime); throw` retains the new time and
retries the unfinished delivery; `clear(); throw` still retries it. A successful
retry never erases a successor it did not own. Pending successors wait while
the old delivery remains unsettled; retries admit the currently selected
Version, not indefinitely retained old code. Handlers must be idempotent.

This distinction between a pending slot and an unsettled delivery is a proposed
new executable rule. The older phrase "one alarm" is not evidence that this
settlement rule was already published. Missing `alarm`, like any missing event
handler, refuses class admission before execution or traffic activation; it is
not a reason to erase an existing delivery obligation.

## Event lifetime and code selection

1. A single Host-owned admission gate orders events per namespace incarnation
   and actor ID. Different IDs proceed independently. No namespace-wide cutover
   barrier or global inventory of actor IDs is required.
2. Immediately before admission, choose one Version from the current proven
   active weighted WorkerDeployment. Pin its exact code and environment for
   this event and its producer tail. Validate every active class before traffic;
   never substitute the largest weight, arrival-time code or an alternate on
   failure.
3. Await `start` and the selected handler. For HTTP, forward the response head
   before reading response bytes. Request and response streams retain the
   existing absent/empty/unknown-length, count, backpressure and cancellation
   behavior. They are not buffered to release the admission gate.
4. After the head, keep later same-ID fetch, alarm and socket callbacks queued
   while actor-owned response production or actor request consumption can still
   run. When the response body ends, errors or is cancelled (including an absent
   body), cancel any remaining request body before retiring the actor. The
   caller owns the request producer: propagate cancellation, but do not claim
   that actor termination terminates arbitrary caller code. An open WebSocket
   transport alone is not a producer lease and does not hold the gate indefinitely.
5. Before the next event, establish that old execution cannot resume. Body EOF,
   a settled `cancel`, a storage fence or a shared boolean is not sufficient
   proof: arbitrary promises/timers may still capture and mutate the instance.
   The conservative implementation terminates the child context after every
   event/drain and confirms termination before admitting the next. Reuse is an
   optimization that needs actual native quiescence evidence.

Alarm and socket handler settlement proceeds directly to retirement, without
an HTTP producer phase. For an upgrade, transfer to the broker completes the
actor handler result. Confirm actor retirement before admitting another same-ID
event; do not wait for the outer Worker or client handshake. Broker commitment
or abandonment is independent of that retirement.

There is no Actor `waitUntil` or detached-background-work API. Actor code must
await its work; forbidding detached work is not the Host's termination proof.
Ordinary ModuleWorker `ctx.waitUntil` remains its separate existing contract.

Proposed portable deadline values, requiring explicit adoption, are 30 seconds
for initialization/handler settlement and 300 seconds for producers after the
head. These are not values derived from the frozen contracts or Cloudflare
limits. Abort the turn signal and terminate the child on expiry. Before a head,
an admitted HTTP deadline yields a complete 504; after a head, the status remains
unchanged and an unfinished response body errors as `response_aborted`. An
interrupted request body is `request_aborted` on its receiving side; a completed
response is not retroactively turned into a body failure. Do not describe already
completed effects as rolled back or automatically replay an HTTP call.

At process/owner replacement, a new owner must prove the old execution is
terminated or fenced from continuing, not merely unable to write SQL. If this
cannot be proved, hold new admission. Both actual backends must demonstrate
this boundary before the new contract is advertised.

## Socket lifetime and transport

`accept` is valid only for the current fetch invocation's incoming WebSocket
upgrade request. It verifies the standard handshake and any selected protocol
against that request. Application authentication/origin policy remains the
application's responsibility. A copied plain object or manually constructed
status 101 response cannot mint the Host-owned upgrade capability.

Returning the exact `ActorUpgrade` transfers the provisional connection to the
broker before actor retirement. It does not yet send a handshake to the client.
Returning another response or throwing abandons it and discards provisional
sends. Actor retirement must not invalidate the transferred capability.

The forward `module-worker.actor` stub has the explicit return type
`Promise<Response | ActorUpgrade>` for `fetch`. The forward Worker runtime's
ordinary `fetch` handler may return that exact Host-branded object unchanged;
the public endpoint commits the connection only when it emits the upgrade head
to the original client. A copied object, `Response` clone or JSON serialization
does not preserve the capability. This first proposal permits pass-through, not
arbitrary transfer to another request, reuse, service-binding forwarding or a
programmatic client-side socket API. Nested Actor-to-Actor upgrade forwarding is
also outside this first shape; ordinary HTTP forwarding remains unchanged.

The broker abandons a provisional connection if the original request is aborted,
the outer handler returns a different result/throws, or the head cannot be sent.
It also expires any still-uncommitted reservation 30 seconds after the actor
returns the upgrade, including one already forwarded by the outer Worker.
Commitment and expiry are mutually exclusive; expiry prevents a later client
upgrade head. This reservation timeout is a
candidate value, not an existing runtime limit. Provisional sends use a bounded
broker queue and are discarded on abandonment. Socket callback delivery starts
only after the client upgrade head. An HTTP status enum containing 101 does not
establish any of these duplex ownership or transport semantics.

The broker retains a connection's opaque ID and attachment bytes while child
contexts come and go. Each message/close callback is a separate serialized
event, selected from the then-active weighted deployment. Version selection
alone does not disconnect sockets. The Host preserves attachment bytes without
translation; mixed application Versions own their codec compatibility.

`send` returns after local acceptance, not peer receipt, persistence or drain.
It is deliberately not a backpressure Promise. No end-to-end native socket
buffer bound is claimed without an implementation that can observe/enforce it.
`close` requests the standard closing handshake and does not guarantee delivery
of a close frame. Methods on a closed handle reject/throw a defined closed-socket
error; `get` returns null for an ID not owned by this live actor connection set.
Attachment reads/writes copy bytes; successful writes survive ordinary context
eviction of that same live connection.

The initial 64 KiB message-capacity candidate is rejected. Existing consumers
accept event data approaching 1 MiB before envelope overhead and signaling
descriptions of up to 100,000 characters. A 64-connection floor also does not
prove behavior for consumers whose configured connection caps are 1,000 or
10,000. Those application caps are not an upstream transport guarantee, but a
smaller portable limit cannot silently substitute for the full consumer goal.
Frame/connection limits and the 2 KiB attachment candidate remain unresolved
until complete encoded frames and both backend implementations are checked.
Message overflow, invalid values and transport overload need explicit
error/close behavior; unbounded application mailbox accumulation is not an
implementation. No new limit is adopted from a native provider by implication.

Normal hibernation/reconstruction preserves the live connection and attachment.
Process/broker loss may disconnect it; reconnect creates a new connection ID.
Graceful restart may send 1012, but a delivered close frame is not guaranteed.
Callbacks are not automatically replayed after failure: close the affected
connection on handler failure/deadline, and let the application use durable
message IDs and acknowledgments if it needs replay. A close callback is not a
guaranteed cleanup hook across process loss.

## Alternative and remaining acceptance work

A branded `stream({ pull, cancel })` API could queue each body callback as a
separate actor event, allowing other events between chunks. It adds a new
producer/response ABI and still needs to terminate detached work. Native-looking
streams cannot be exempted by guesswork. The proposed first contract keeps
ordinary streams and accepts the explicit cost of holding same-ID admission
while they run. It must not claim long-body concurrency that it does not offer.

Before adoption, select all new error codes and capacity/deadline values,
including the provisional socket queue bound. Preserve the old identity byte
closures and finish the forward authoring prerequisites below. Then author
the new Definitions and one executable conformance bundle outside data-only
Form Packages, followed by independent Host implementations and consumer E2E.
This proposal alone does not permit Host support advertising.

## Forward identity closure

The application needs a client WebSocket path through its ordinary Worker
endpoint, not an extra Host-to-Actor public route. Therefore this proposal
includes a forward `worker.runtime` identity; it cannot keep runtime 1.1.0 and
infer upgrades from its HTTP status field. No version numbers are allocated here.

| Contract | Why a forward identity is required |
| --- | --- |
| `worker.actor` Interface | Class ABI, event lifetime, alarm settlement and socket execution. |
| `module-worker.actor` Binding | Exact new actor Interface reference and opaque upgrade return type. |
| `worker.runtime` Interface | Ordinary Worker fetch can pass through the broker-owned upgrade. |
| `ModuleWorker` Form | Provides the new exact runtime Interface. |
| `WorkerVersion` Form | Requires that runtime and the new actor Interface/Binding. |
| `ActorNamespace` Form | Requires that runtime and provides the new actor Interface. |
| `DurableWorkflow`, `QueueConsumer`, `WorkerCronTrigger`, `WorkerCustomDomain`, `WorkerEndpoint` Forms | Their definitions embed the exact runtime reference, even where event behavior does not change. |
| `WorkerDeployment` Form | Embeds the exact ModuleWorker and WorkerVersion references. |

These are nine Form identities, two Interface identities and one Binding
identity. SQL, objects, KV, queue and service Interfaces/Bindings are not
implicitly expanded. In particular, `worker.service` remains ordinary HTTP;
this proposal does not promise upgrade tunneling through an arbitrary service
chain. The Takoform API, Core and Provider do not acquire a new version merely
because these publisher-owned contracts advance. Any Provider mapping needed
to select a new Form is a separate software change.

The authoring sources are `internal/edgeformcatalog/interfaces.go`,
`bindings.go` and `catalog.go`. `render.go` resolves exact references, and
`scripts/current-form-families.mjs` builds candidate sets and their aggregate
index. Keep one explicitly selected Interface/Binding per name. A second local
multi-version archive is unnecessary: the old exact Definitions already exist
at immutable publisher source `e7f8a39311dd011b8467e97e7f300cabb9a6b06c`,
whose set tag resolves to `3231633605b737ce5279d7fc020b4780568e7091`.
Old references must resolve against that exact source, not today's named
candidate. This source/set relationship was checked locally; this proposal
does not itself claim a fresh remote readback or authorize a publication.

The original `integrity/source-baseline.json` stays byte-identical and is now
verified against that historical Git snapshot. Same-version reidentification
is refused; exact current generation is a separate check. The fixed legacy
corpus and retained Form inventory still need forward authoring/append-only
verification before new candidates can be selected and published. Preserve all
old package roots and reference closures, including the nine current Forms
that would leave current selection. The next published source must also be
protected before subsequent editing. Updating only the actor definition or
only a version label does not close this graph.

## Evidence versus proposed behavior

The [Streams Standard](https://streams.spec.whatwg.org/#underlying-source-api)
defines source callbacks and pull scheduling; wrapping only an Actor's fetch
entrypoint does not make those callbacks part of its mutex. That is the reason
for explicitly addressing producer lifetime, not a new standard stream API.

[Facet abort](https://developers.cloudflare.com/dynamic-workers/usage/durable-object-facets/#abort)
provides child shutdown and storage-preserving replacement. The existing local
tail-free native checks do not yet prove cancellation of live producer tails or
socket transport across child handoff. Ordinary native DO shutdown can allow
storage-free old calls to finish, as documented in the
[DO lifecycle](https://developers.cloudflare.com/durable-objects/concepts/durable-object-lifecycle/#shutdown-behavior).
Neither upstream documentation nor a draft signature establishes end-to-end
self-host/WfP conformance.
