# Actor execution contract — concrete proposal

Status: **unaccepted, non-normative proposal**, 2026-09-08. No version is
allocated. This is a proposed forward contract, not an interpretation of an
existing identity, an executable Host implementation, a catalog member or a
release artifact.
The [gap analysis](actor-runtime.md) fixes the existing published evidence.
API v1 and all published Form, Interface and Binding bytes remain unchanged.
The Takoserver actor-runtime review is input for resolving this proposal; it is
not publisher authority and does not allocate an identity or grant support.

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

interface ActorSocketErrorEvent {
  readonly code: "transport_error";
}

// A real Response with a Host-owned reservation, not a structural substitute.
interface ActorUpgradeResponse extends Response {
  readonly status: 101;
  readonly body: null;
}

interface ActorSockets {
  accept(
    request: Request,
    options?: { protocol?: string; attachment?: Uint8Array },
  ): Promise<{ response: ActorUpgradeResponse; socket: ActorSocket }>;
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
  ): Response | Promise<Response>;
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
  socketError(
    socket: ActorSocket,
    event: ActorSocketErrorEvent,
    turn: ActorTurn,
  ): void | Promise<void>;
}

type ActorConstructor = new (
  context: ActorContext,
  env: Readonly<Record<string, unknown>>,
) => ActorInstance;
```

### Class construction and initialization

The exact named export must be constructable. The five required event handlers
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
   barrier is required for ordinary admission; deletion separately maintains a
   durable actor-ID inventory for destruction.
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

The candidate deadline is by event kind, rather than one wall-clock handler
plus a separate producer timer:

| Phase | Candidate deadline |
| --- | --- |
| Synchronous construction plus `start` | 30 seconds wall time. |
| Any event and its actor-owned producer | 30 seconds active CPU. Waiting on storage, network or backpressure is not CPU. |
| Connected HTTP fetch and response producer | No hard wall deadline while the original caller remains connected. Caller abort cancels both streams and retires the child. |
| Alarm handler | 15 minutes wall time. Failure, timeout or owner loss keeps the obligation for retry. |
| Message, close or error callback | 15 minutes wall time. Timeout terminates the child and closes the affected connection with 1011. |

Thirty seconds follows the managed Durable Object active-CPU and
initialization-gate limits; 15 minutes bounds alarm and socket callbacks. These
are deliberate portable candidate values, not claims about frozen bytes or
every native default. Abort `ActorTurn.signal`, cancel owned streams and any
upgrade reservation, then terminate the child on expiry; do not wait for
application cleanup. Before a
head, an admitted HTTP deadline yields a complete 504. After a head, the status
remains unchanged and an unfinished response body errors as `response_aborted`.
An interrupted request body is `request_aborted` on its receiving side; a
completed response is not retroactively turned into a body failure. Do not
describe already completed effects as rolled back or automatically replay an
HTTP call. Retirement acknowledgement, not promise rejection alone, reopens
same-ID admission.

At process/owner replacement, a new owner must prove the old execution is
terminated or fenced from continuing, not merely unable to write SQL. If this
cannot be proved, hold new admission. Both actual backends must demonstrate
this boundary before the new contract is advertised.

## Socket lifetime and transport

`accept` is valid only for an Actor fetch reached from an incoming client
WebSocket upgrade through the ordinary Worker endpoint. The reservation is
bound to that Host-tracked invocation chain, not JavaScript `Request` object
identity: a caller can construct a sanitized internal request before calling
the Actor. The Host validates the actor's incoming handshake and selected
protocol against the original client handshake. A different invocation cannot
acquire authority by copying its URL, headers or other properties. Application
authentication/origin policy remains the application's responsibility.

Only `accept` mints the hidden reservation carried by `ActorUpgradeResponse`.
This is a real Response with status 101 and a null body, mutable ordinary
headers and no public native WebSocket handle. Returning it, or an authorized
alias described below, transfers the provisional connection to the broker
before actor retirement. It does not yet send a handshake to the client.
Returning a response without that reservation or throwing abandons it and
discards provisional sends. Actor retirement must not invalidate a transferred
reservation.

The forward `module-worker.actor` stub retains `fetch(request):
Promise<Response>`. Ordinary HTTP responses still support the usual body,
`ok`, `json` and other Response operations without union casts. The forward
Worker runtime's ordinary fetch handler also returns a Response. To preserve
framework composition, its Response implementation has these explicit rules:

- `new Response(upgrade.body, upgrade)` with the branded Response itself as
  the initialization argument, status 101 and null body creates an alias to
  the same reservation. `upgrade.clone()` throws `TypeError`; it does not
  create an alias. Neither operation creates a second connection or a second
  right to commit.
- Copying properties into a plain initialization object, spreading the
  Response, serialization or an unbranded status-101 value cannot preserve or
  mint the slot. A constructor invocation without a valid slot cannot create
  an upgrade; a non-null upgrade body is rejected.
- Ordinary response headers can be changed by middleware. The Host validates
  reserved handshake headers and the negotiated protocol when committing;
  header mutation cannot acquire or redirect transport authority.
- Every alias remains bound to the same original invocation and one-shot
  reservation. Returning an alias in another request, forwarding it through
  a service binding or trying to commit an already settled reservation fails.
  The public endpoint commits only when emitting the head to the original
  client; commitment, caller abort and abandonment settle all aliases together.

The observable constructor rules are therefore:

```typescript
actorResponse.status === 101;                    // true
actorResponse.body === null;                     // true
new Response(actorResponse.body, actorResponse);  // same reservation
actorResponse.clone();                           // throws TypeError
new Response(null, { status: 101 });              // throws RangeError
new Response(null, { ...actorResponse });         // no reservation; cannot be 101
```

Only the original response and the Response-initializer alias are portable.
Copying properties, serializing, or using an unbranded status-101 value cannot
preserve or mint the reservation. A non-null body for a 101 upgrade is rejected.
At outer commitment, the Host fills absent handshake fields and validates
reserved `Connection`, `Upgrade`, `Sec-WebSocket-Accept`,
`Sec-WebSocket-Protocol` and `Sec-WebSocket-Extensions` headers. If an alias
supplies a conflicting reserved value, commitment is rejected: the Host
abandons the reservation and provisional sends and returns HTTP 502 before
sending any 101. It does not silently repair or overwrite a conflict. Ordinary
headers remain mutable for middleware and cannot acquire transport authority.
Protocol selection must be absent or exactly one token offered by the original
client, otherwise `accept` rejects `invalid_upgrade`. It also rejects
`invalid_upgrade` before creating a provisional connection when the original
invocation is not a client WebSocket upgrade or the Host-tracked request chain
does not match.

These constructor/clone semantics are a proposed extension of the forward
Worker runtime, not behavior inferred from an HTTP status enum or the current
runtime identity. No native WebSocket, Durable Object, public reservation
token or programmatic client-side socket API enters the application ABI.
Nested Actor-to-Actor upgrade forwarding is also outside this first shape;
ordinary HTTP forwarding remains unchanged.

The broker abandons a provisional connection if the original request is aborted,
the outer handler returns a response without its reservation or throws, or the
head cannot be sent.
There is no independent reservation timer: before commitment, its lifetime is
the original connected HTTP invocation and its abort signal. Provisional sends
use a bounded broker queue and are discarded on abandonment. Socket callback
delivery starts only after the client upgrade head. An HTTP status enum
containing 101 does not establish any of these duplex ownership or transport
semantics.

The broker retains a connection's opaque ID and attachment bytes while child
contexts come and go. Each message, close or error callback is a separate
serialized event, selected from the then-active weighted deployment. Version
selection alone does not disconnect sockets. The Host preserves attachment
bytes without translation; mixed application Versions own their codec
compatibility.

For a Host-observed terminal event, exactly one of `socketClose` and
`socketError` is admitted. A received Close frame or Host-initiated close uses
`socketClose`; loss without a usable close code uses `socketError` with the
stable `transport_error` code. A broker/process loss can prevent either
callback; neither is a durable cleanup hook and callbacks are not replayed.
Once a terminal callback is admitted, `get`/`list` no longer expose the
connection, and `send`, `close` and `setAttachment` fail `socket_closed`.
`getAttachment` remains readable from that callback's handle until it settles;
failure never schedules the other terminal callback.

The candidate capacities are:

| Limit | Candidate value and unit | Required behavior |
| --- | --- | --- |
| Connections | 10,000 live or provisional connections per namespace incarnation and actor ID | A provisional connection counts from `accept`; exceeding the limit rejects `accept` as `connection_limit_exceeded`. Abandonment or terminal close releases it. |
| Message | 33,554,432 encoded bytes per complete text or binary frame, inbound and outbound | Strings are counted after UTF-8 encoding; `Uint8Array` uses `byteLength`. Oversized inbound data closes with 1009; oversized `send` throws `message_too_large` without a partial frame. |
| Attachment | 16,384 bytes per live connection | Bytes are copied. `null` clears. An oversized initial/replacement value rejects as `attachment_too_large` and leaves the old value unchanged. |
| Outbound queue | 33,554,432 encoded bytes per connection, including the frame being added | `send` returns after broker acceptance. Crossing the bound throws `transport_overloaded`; no partial frame is queued. Occupancy falls only when the broker hands a complete frame to transport. |

`ActorSocket.id` is opaque, unique within the namespace incarnation, stable
through context eviction and code selection, and never reused while live.
`list()` returns committed live/closing connections in lexical ID order; a
provisional connection is visible only through its own `accept` handle.
Version change preserves ID, attachment and transport; reconnect after
transport loss gets a new ID. `setAttachment` resolves only after atomic broker
replacement, so a later callback sees the copied bytes. Attachments survive
context eviction only while the transport survives, and are removed on
transport loss, connection close or namespace deletion.

`send` returns after local acceptance, not peer receipt, persistence or drain.
`close` requests the standard handshake and does not guarantee a close frame.
Host-generated closures use 1009 for oversized inbound data, 1011 for handler
error/deadline, 1012 for planned broker restart and 1001 for namespace deletion.
Application close codes must be 1000 or 3000–4999 and the UTF-8 reason must fit
123 bytes; otherwise `close` throws `invalid_close`. An omitted or `undefined`
code defaults to 1000, an omitted or `undefined` reason defaults to the empty
string, and a reason supplied without a code also uses 1000; the same 123-byte
UTF-8 bound applies in every case.

Normal hibernation/reconstruction preserves the live connection and attachment.
Process/broker loss may disconnect it; reconnect creates a new connection ID.
Graceful restart may send 1012, but a delivered close frame is not guaranteed.
Callbacks are not automatically replayed after failure: close the affected
connection on handler failure/deadline, and let the application use durable
message IDs and acknowledgments if it needs replay. A close callback is not a
guaranteed cleanup hook across process loss.

### Errors

Every public Actor runtime rejection is an `Error` whose `name` and readonly
`code` are the same stable snake-case value. Messages and causes are diagnostic
only:

```typescript
type ActorRuntimeErrorCode =
  | "backend_unavailable"
  | "request_too_large"
  | "invalid_upgrade"
  | "connection_limit_exceeded"
  | "message_too_large"
  | "attachment_too_large"
  | "transport_overloaded"
  | "socket_closed"
  | "invalid_close"
  | "request_aborted"
  | "response_aborted";

interface ActorRuntimeError extends Error {
  readonly name: ActorRuntimeErrorCode;
  readonly code: ActorRuntimeErrorCode;
}
```

Class/deployment/environment absence or inability to begin an event rejects
`backend_unavailable`; it never retries another weighted Version. The existing
HTTP streaming contract retains `request_too_large`, `request_aborted` and
`response_aborted`, including its 100 MiB portable request/response floors and
declared-length behavior. Storage/SQL errors retain their separate domain and
are not collapsed into transport codes.

After execution begins, constructor, `start` or `fetch` failure before a head
returns a complete generic 500. An HTTP deadline before a head returns 504;
failure or deadline after a head preserves its status and errors an unfinished
body as `response_aborted`. Caller disconnect or request-body cancellation is
`request_aborted`. Error bodies do not expose tenant exceptions. Alarm
failure/deadline retains the delivery obligation. Socket callback
failure/deadline closes the affected connection with 1011 and is not retried.

## Retirement and namespace destruction

The implementable boundary is a stable Host owner with a durable namespace
incarnation/epoch, an actor-ID inventory, the per-ID admission gate, alarm
obligations and socket broker. Replaceable tenant execution is a child facet or
equivalent killable context. None of that state is exposed to tenant code.

After every event and producer drain, or on deadline/cancellation, the owner
terminates the child and must establish all three conditions before the next
same-ID event: the old stub rejects with the retirement reason; no held external
completion can resume tenant code; and the replacement child is created under
the same namespace epoch and sees the preserved private SQL. Version selection
and rollback use this storage-preserving retirement. A native mechanism that
cannot prove these conditions keeps admission closed; a promise rejection,
storage fence or shared boolean alone is insufficient.

Namespace deletion is a separate durable state machine:

1. Refuse deletion while any WorkerVersion binding targets the namespace, as
   the current Binding already requires.
2. Persist `deleting` and advance the incarnation fence before acknowledging
   work. New/current stubs fail `backend_unavailable`; recovery resumes the
   same deletion.
3. For every inventoried actor ID, cancel queued/unstarted events, terminate
   the child, discard the alarm obligation, abandon provisional upgrades and
   remove broker connections (best-effort close 1001).
4. Permanently delete each child's private database and all Host inventory,
   alarm and broker records for that epoch. A replacement resource receives a
   new epoch and empty state.
5. Report success only after authoritative readback finds no state, alarm,
   connection, live child or admissible stub for the deleted epoch. Uncertainty
   remains retryable `deleting`, never successful deletion.

Context eviction, Version removal, deployment update and rollback never enter
this deletion state. A delivered close callback is not required for
destruction; the Host-owned broker and inventory are the authority.

## Alternative and remaining acceptance work

A branded `stream({ pull, cancel })` API could queue each body callback as a
separate actor event, allowing other events between chunks. It adds a new
producer/response ABI and still needs to terminate detached work. Native-looking
streams cannot be exempted by guesswork. The proposed first contract keeps
ordinary streams and accepts the explicit cost of holding same-ID admission
while they run. It must not claim long-body concurrency that it does not offer.

The concrete error, deadline, capacity, callback and retirement values above
are one resolved candidate closure, not a menu left for backend defaults to
fill. Backends must not silently replace them with provider defaults. The valid
alternatives are to keep the current fail-closed refusal,
publish an HTTP/alarm-only subset (which would not run socket consumers), adopt a
provider-specific Durable Object class API (which would not define an
independent self-host ABI), or finish this portable proposal as one closure.
The last is the candidate recommendation; none of the first three is selected.

Before adoption, preserve the old identity byte closures and finish the forward
authoring prerequisites below. Then author the new Definitions and one
executable conformance bundle outside data-only Form Packages, followed by
independent Host implementations and consumer E2E. This proposal alone does
not permit Host support advertising.

## Forward identity closure

The application needs a client WebSocket path through its ordinary Worker
endpoint, not an extra Host-to-Actor public route. Therefore this proposal
includes a forward `worker.runtime` identity; it cannot keep runtime 1.1.0 and
infer upgrades from its HTTP status field. No version numbers are allocated here.

| Contract | Why a forward identity is required |
| --- | --- |
| `worker.actor` Interface | Class ABI, event lifetime, alarm settlement and socket execution. |
| `module-worker.actor` Binding | Exact new actor Interface reference and Response-compatible upgrade behavior. |
| `worker.runtime` Interface | Ordinary Worker fetch and Response construction preserve the request-bound broker reservation; cloning rejects the upgrade. |
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

If the Actor candidate is coordinated with the separate Workflow and VectorIndex
proposals, their exact references must be closed in one WorkerVersion and one
matching WorkerDeployment update. They are not independent WorkerVersion or
WorkerDeployment bumps. This proposal introduces no Container, new Queue-family
or AIForms identity; the existing `QueueConsumer` row above is only a
runtime-dependent Form in that same graph.

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
is refused; exact current generation is a separate check. The unused legacy
corpus copies have been removed from the current checkout, not replaced with
another generator: no executable check consumed their scenario expectations.
Their bytes remain protected in exact Git history; active fixtures remain
beside each Form. Retained Form inventory advancement still needs append-only
verification before new candidates can be published. Preserve all
old package roots and reference closures, including the nine current Forms
that would leave current selection. The next published source must also be
protected before subsequent editing. Updating only the actor definition or
only a version label does not close this graph.

## Evidence versus proposed behavior

The actual consumer review rejects an opaque `{ status, headers }` upgrade
union. Takos routes return stub responses through Hono; its CORS middleware
first initializes `c.res`, after which Hono reconstructs the returned value
with `new Response(_res.body, _res)`. A reproduction against pinned Hono
4.12.31 converts the opaque value to a 500 because it cannot construct that
status-101 Response. Takos also mutates headers after routing. Yurucommu's
realtime/signaling hub interfaces and routes return `Promise<Response>`, and
ordinary hub calls use `ok` and `json`. The proposed Response-compatible slot
keeps those existing interfaces without a product-specific framework bypass.
This evidence rejects the opaque design; it does not prove the new constructor,
clone rejection, cross-request isolation or backend transport implementation
works.

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
