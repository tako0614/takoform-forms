# Portable Workflow execution successor

Status: **selected in local current source; not signed, published, or Host-supported**. The source is
[`WorkflowCandidateInterface`](../../internal/edgeformcatalog/workflow_candidate.go).
The current generator selects exactly its five-artifact closure and the
create-only writer materializes three new local Form Package roots. Neither
action activates a Host implementation or publishes a version.

This fills the application execution contract missing from the existing
`worker.workflow@1.0.0`. It does not reinterpret that definition or change
Takoform API v1. Existing immutable definitions and packages keep their bytes.

## Application surface

A workflow is a named class export in the Worker Bundle's main ES module. No
vendor base class or native runtime context is required. The normal plain-object
default export remains required by `worker.runtime@1.1.0`; its existing handlers
are unchanged.

```js
export class OrderFulfilment {
  constructor(env) {
    this.env = env;
  }

  async run(event, step) {
    const order = await step.do("load-order", async () => {
      return { id: event.params.orderId };
    });
    await step.sleep("delay", 10);
    const approval = await step.waitForEvent("approval", {
      type: "approved",
      timeoutSeconds: 3600,
    });
    return { orderId: order.id, approved: approval?.approved === true };
  }
}

export default {
  async fetch(request, env) {
    const instance = await env.ORDERS.create({ params: { orderId: "order-1" } });
    return Response.json({ id: instance.id });
  },
};
```

The instance caller remains `env.ORDERS.create/get`, followed by
`instance.status/sendEvent/terminate`. It does not receive the class's `step`
object. `ORDERS` must be an explicitly declared workflow binding.

`create({id, params})` commits the queued instance before replying. An omitted
`id` is not a retry key: callers that might repeat a call after a lost reply
choose a stable ID and use `get(id)` to inspect the result. IDs are scoped to
one `DurableWorkflow` resource and cannot be reused while its instance is
retained. `status()` reads committed state; a workflow failure is an `errored`
status, not a failed status request. `terminate()` preserves an already-terminal
outcome, otherwise fences and retires running code before publishing
`terminated`.

`sendEvent({type, payload?, eventId?})` commits an event before replying. The
optional `eventId` is an instance-scoped retry key: a second send with the same
key, type and canonical payload succeeds without another admission, even after
the event was consumed or the instance terminated. The key remains reserved
until that same retained instance expires; reusing it with different content
fails `event_conflict`. The Host keeps the key, type, and canonical payload
digest after consumption, not an extra full copy of the payload; absent
payload is distinct from `{}`. Deduplication precedes terminal and capacity
rejection. Without a key,
each successful send is a new event and a lost reply can result in duplicates.
Matching events are consumed in acceptance order. An instance retains at most
1,024 unconsumed events; a new one fails `event_queue_full` at that bound.
Admission and terminal transition serialize, so a new event after terminal
fails `instance_terminal`. A transport interruption alone does not prove
whether create or send committed.

One atomic admission decision checks the retained key first, then terminal
state, then pending-event capacity, then appends the event. A same-key retry
therefore succeeds even when the inbox is full or terminal; conflict and
capacity refusals make no state change. A wait consumes the earliest eligible
matching event atomically with its step completion. An event already committed
before the timeout decision and accepted no later than the saved deadline wins,
including at the same timestamp. Once timeout commits, a subsequently admitted
event cannot change that step even if its timestamp is identical; it may remain
for a future wait. Linearization order, not clock equality alone, breaks the
tie. Termination discards
unconsumed events while retaining their keyed digests for retry settlement.
The 1,024-step lifetime bound permits at most 1,024 consumed events; combined
with the 1,024 pending-event bound, at most 2,048 distinct keyed accepted
events need fingerprints per instance. Pending payloads are bounded by the
existing 1 MiB-per-document and 1,024-event limits. A Host that cannot meet
those bounds must not advertise this Interface.

After retention expiry, the ID may be reused for a new instance generation.
Because the caller ABI has no generation argument, neither `create` nor
`sendEvent` promises that an old retry is safe across expiry and ID reuse.
Callers must settle uncertain keyed sends while the original instance remains
retained; the Host does not extend retention for them.

The observable status transitions are:

| From | Committed cause | To |
| --- | --- | --- |
| absent | accepted `create` | `queued` |
| `queued`, `sleeping`, `waiting` | fenced execution starts or resumes | `running` |
| `running` | future sleep/retry or event wait after old context retires | `sleeping` or `waiting` |
| `running` | valid final output or failure after old context retires | `complete` or `errored` |
| any live status | explicit `terminate` after old context retires | `terminated` |
| any live status | absolute lifetime or step bound after old context retires | `errored` |
| terminal status | repeated `terminate` | unchanged |

`status()` never exposes a half-committed transition. The absolute lifetime is
measured from the committed creation time, not extended by retries, sleeps,
events, or deployment changes. Terminal records remain readable for 30 days;
after expiry `get`/`status` report `unknown_instance`, and ID reuse follows
the normal creation rule. Removing a `DurableWorkflow` resource with live
instances requires the Host to settle or refuse them under its lifecycle
contract; this source does not create a second instance Resource or bypass the
Host's Resource deletion authority.
The Interface's `per_key` ordering key is `(DurableWorkflow resource, instance
id)`. It covers admission, consumption, state transitions and status reads;
different instances need not be globally ordered. Events of one matching
type use their per-instance acceptance order.

Each execution context selects the deployment's then-current weighted version,
constructs one fresh class using ordinary `new Export(env)`, then resolves and
calls `instance.run(event, step)` once with `this=instance`. Missing/non-callable
execution-time `run`, lookup failures and constructor failures are `run_threw`.
Retry and wake create a new context and replay the code. Class fields,
closures and module state are not durable. Code outside steps must be
side-effect-free and deterministic against recorded history; step effects must
be idempotent because a process can die after an effect but before its journal
commit. A deployment change must remain compatible with in-flight histories.

## Decisions encoded in the candidate

- `run` and `step.do` return a plain data-only JSON object or `undefined`, not
  result wrappers. Documents are at most 1 MiB canonical UTF-8 and 1,024 root
  properties. Getters and `toJSON` are never invoked during serialization.
- `step.do(name, effect, retryPolicy?)` durably saves the normalized policy on
  first use. Omission means one attempt. A supplied policy requires
  `maxAttempts` (1–100, including the first); delay defaults to zero, backoff to
  `constant`, and the delay ceiling to 43,200 seconds. Exponential delay after
  failed attempt `k` is `min(ceiling, initialDelay * 2 ** (k - 1))`.
- Invalid or oversized callback results count as failed attempts. Exhaustion
  is durably journaled before `step_failed` is returned. Success is committed
  before resolving and returns a decoded clone, not the callback's object.
  Invalid final `run` output becomes `run_threw`. This deliberately changes the
  old separate `stepDo.document_too_large` outcome only in the new contract;
  caller-side create/event admission retains that error.
- Future sleep, retry and event wait stop the context before publishing the
  parked status. Their Promises never resolve in that stopped context. A zero
  sleep completes durably in the current context; even a zero-delay retry uses
  a new context. A retained eligible event beats timeout and is consumed once.
- Finished names replay success or failure without re-running a callback,
  including when code changes the step kind. Pending names keep their first
  configuration. An incomplete cross-kind reuse, overlapping step calls, or
  any `run` settlement with an unsettled step terminates with
  `step_definition_mismatch`, ahead of `run_threw`. This is an uncatchable Host
  stop, not a Promise rejection delivered to application catch/finally.
- Invalid JS step arguments reject with `TypeError` before new effects or
  journal writes. A finished name is looked up before unused argument
  validation. Backend interruptions and instance bounds are Host control, not
  catchable app sentinels.
- Host errors have an immutable, non-enumerable own `name`, with private
  object-identity provenance. Rethrowing the exact error preserves its origin;
  a copied name or wrapper does not. Other fields remain annotatable. Only a
  genuine uncaught exhausted-step error becomes terminal `step_failed`;
  uncaught timeout or ordinary application failure becomes `run_threw`.

The existing status vocabulary, 1,024-step bound, one-year absolute execution
lifetime, 30-day terminal retention, and runtime-data ownership are retained.
Nothing may keep executing after termination or the absolute lifetime cutoff.
The Host owns the journal, event inbox, wake scheduling and per-instance
execution fence. Replayed wakes may be delivered more than once but only one
fenced context owns an instance at a time. A stale context cannot commit,
perform a new Host-mediated effect, or publish a state. External effects remain
at-least-once and require application idempotency. The Host retires old code before a
new owner runs or a parked/terminal state becomes visible. These are portable
outcomes; the storage, scheduler and isolation mechanisms belong to each Host.

Cloudflare's native Workflows are not a universal implementation of this
contract: as of the cited documentation they cannot deploy into Workers for
Platforms namespaces. A WfP Host therefore needs its own durable coordinator
and fresh, request-scoped invocation of the user Worker; request-scoped RPC
stubs cannot be stored across wakes. Self-host implementations have the same
observable obligations but may use different execution machinery. See
[Cloudflare Workflows limits](https://developers.cloudflare.com/workflows/reference/limits/)
and [Workers for Platforms dynamic dispatch](https://developers.cloudflare.com/cloudflare-for-platforms/workers-for-platforms/configuration/dynamic-dispatch/).

## Exact forward closure

| Artifact | Selected local successor identity | Reference changed |
| --- | --- | --- |
| `worker.workflow` Interface | `2.0.0` | New class and replay semantics |
| `module-worker.workflow` Binding | `2.0.0` | Exact new Interface digest; caller projection only |
| `DurableWorkflow` Form | `0.2.0` | Provides the exact new Interface |
| `WorkerVersion` Form | `0.4.0` | Selects exactly the new workflow Binding and Interface |
| `WorkerDeployment` Form | `0.3.0` | Pins the exact new WorkerVersion Definition |

Numeric Interface/Binding versions are required by the released Core schema.
Local selection does not publish or reserve these identities. Other current bindings are
unchanged. `ModuleWorker` and `worker.runtime@1.1.0` remain exact existing
contracts: additional named exports do not alter the default handler ABI or
environment. This candidate does not add handler-free WorkerVersion authoring.
It has no dependency on the separate Vector or Container candidate graphs.
The three stable Form targets are independent next versions after published
`DurableWorkflow@0.1.0`, `WorkerVersion@0.3.0` and
`WorkerDeployment@0.2.0`. Unpublished Actor, Vector and aggregate Runtime
proposals do not reserve them. Selecting these versions does not promote those
graphs or claim Host support. The publisher must recheck remote identity
absence and retain old package roots before any publication.

The standalone renderer is `go run ./cmd/workflow-candidate`; it writes JSON to
stdout only. `cmd/current-form-source` selects this closure for local package
generation. It remains absent from Provider mappings and Host support discovery.

## Implementation acceptance still required

Core validation and exact-reference tests prove artifact construction, not
execution. Host acceptance must exercise this exact candidate through actual
class construction, binding calls, persisted retries and wake, changed-code
replay, retained event races, invalid results, and stop-before-state visibility.
Physical stop must cover controller loss, restart, stale-owner fencing,
stop-before-start and CPU-bound code, as well as held I/O. Self-host and managed
backends need their own execution proof; publication alone qualifies neither.
