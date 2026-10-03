# Forward Workflow execution candidate

Status: **selected for source authoring inside one unpublished Actor+Workflow
candidate closure; not published or supported**. The Workflow semantic source
is [`WorkflowCandidateInterface`](../../internal/edgeformcatalog/workflow_candidate.go).
The joint renderer wraps those semantics as `worker.workflow@3.0.0` against
`worker.runtime@2.0.0` and emits one shared WorkerVersion/WorkerDeployment with
Actor; see [the joint Actor proposal](actor-execution-contract.md). The
standalone `worker.workflow@2.0.0` renderer remains a local draft, not a
separately selected or publishable closure. Nothing here registers a current
Form, publishes a package, activates a Host implementation, or allocates a
release identity.

This specifies the exact JavaScript application ABI that remains underspecified
by the high-level class/`run(event, step)` description in existing
`worker.workflow@1.0.0`. It does not reinterpret that definition or change
Takoform API v1. Existing immutable definitions and packages keep their bytes.

## Application surface

A workflow is a named class export in the Worker Bundle's main ES module. No
vendor base class or native runtime context is required. The normal plain-object
default export remains required by the selected `worker.runtime@2.0.0`; its existing handlers
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

Each execution context selects and pins one exact WorkerVersion from the
deployment's then-current weights, constructs one fresh class using ordinary
`new Export(env)`, then resolves and calls `instance.run(event, step)` once with
`this=instance`. Missing/non-callable execution-time `run`, lookup failures and
constructor failures are `run_threw`. Retry and wake create a new context and
select the deployment's then-current weights again. Class fields, closures and
module state are not durable. Only the current fenced execution owner may
commit status, step history, or event consumption. Code outside steps must be
side-effect-free and deterministic against recorded history; step effects must
be idempotent because a process can die after an effect but before its journal
commit. There is no history migration or rewrite: every promoted weighted
version must replay existing histories compatibly. The DurableWorkflow
identity itself has no update capability; WorkerDeployment promotion is the
code/weight update path.

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
`terminate()` fences the current execution owner, cancels future continuations,
and succeeds only after the executing context is physically stopped; a Host
unable to prove that must not publish `terminated`. Only the current fenced
owner can commit, and deletion must wait until no owner or continuation may
still commit.

Instance and step records are keyed by the DurableWorkflow resource's Host UID,
not its reusable name. Deleting and recreating the resource receives a new UID
and starts with empty history; stale owners from the prior UID cannot commit to
it. Terminal records remain readable and their IDs remain taken for exactly
30 days while that DurableWorkflow identity exists. Generic resource DELETE is refused before
mutation with `dependency_in_use` (409) while a live Workflow Binding remains;
failure detail identifies that Binding. After bindings are removed, DELETE is
still refused with `dependency_in_use` (409) while a queued, running, sleeping,
or waiting execution identity owned by this Workflow UID remains, or any owner
or continuation may still commit. Failure detail identifies the active
execution identity, distinguishing it from an external live Binding. This is a
long-lived dependent execution, not a transient `resource_busy` condition;
`resource_busy` remains reserved for bounded transient concurrent mutation or
index maintenance. Termination must physically stop and fence each owner before
the execution identity is terminal. Once all instances are terminal, all
bindings are absent, and all owners are stopped and fenced, successful DELETE
purges the retained terminal history and queued events. This successful
identity deletion is the explicit early-purge exception to the 30-day
retention window.

The Interface declares `ordering: per_key`, keyed by Workflow instance ID.
Signals are ordered by successful per-instance acceptance. A wait atomically
consumes the oldest retained event of its exact type; events of other types
remain queued, and one event can satisfy only one wait. An event accepted at or
before its wait deadline wins, including equal timestamps. Each instance may
retain at most 1,024 unmatched events and 1,048,576 canonical UTF-8 bytes over
the stored `{type,payload?}` documents. A signal committed directly to an
already-registered parked wait does not use queue capacity. A `sendEvent` that
would exceed either limit fails with `event_queue_full` and is not durably
accepted. Terminal transition purges unmatched queued events.

## Exact forward closure

| Workflow artifact in the single joint closure | Local candidate version        | Reference changed                                                             |
| --------------------------------------------- | ------------------------------ | ----------------------------------------------------------------------------- |
| `worker.workflow` Interface                   | `3.0.0`                        | Exact workflow class/replay/event contract under the joint runtime-2 identity |
| `module-worker.workflow` Binding              | `3.0.0`                        | Exact new Interface digest; caller projection only                            |
| `DurableWorkflow` Form                        | `0.2.0` selected source target | Provides the exact joint Interface and lifecycle semantics above              |
| `worker.runtime` Interface                    | `2.0.0`                        | Shared Actor+Workflow runtime ABI; owned in the joint proposal                |
| `ModuleWorker` Form                           | `0.2.0` selected source target | Provides exact runtime-2 Interface                                            |
| `WorkerVersion` Form                          | `0.4.0` selected source target | One shared Actor+Workflow successor; exact runtime, Actor, and Workflow refs  |
| `WorkerDeployment` Form                       | `0.3.0` selected source target | One shared exact WorkerVersion target                                         |

Numeric Interface/Binding versions are required by the released Core schema;
these selected source targets do not publish or admit those identities. The
standalone and broader development renderers retain their prerelease Form
versions. This selected closure is composed once with
Actor and does not publish a Workflow-only WorkerVersion/WorkerDeployment first.
The shared WorkerVersion keeps the existing default handler surface while
selecting the exact runtime-2, Actor, and Workflow contracts. Other current
published definitions remain unchanged. The closure does not add handler-free
WorkerVersion authoring and does not include the separate Vector or Container
candidate graphs.

The standalone local renderer is `go run ./cmd/workflow-candidate`; it writes
JSON to stdout only and emits the unselected Workflow 2.0 draft. The separate
`RenderActorWorkflowCandidate` command retains its prerelease development
bytes. `cmd/current-form-source` selects `RenderActorWorkflowSelectedSource`,
which emits Workflow 3.0 with Actor and a single shared
WorkerVersion/Deployment. These selected source candidates are present in the
local candidate generator, but not in the signed current publisher set,
Provider mappings or Host support discovery.

## Implementation acceptance still required

Core validation and exact-reference tests prove artifact construction, not
execution. Host acceptance must exercise this exact candidate through actual
class construction, binding calls, persisted retries and wake, changed-code
replay without history migration, FIFO same-type signals with unrelated queued
types, event-count and byte overflow with no acceptance, equal-deadline races,
terminal queue purge, delete refusal while bound or active, deletion after
termination/fencing/unbind, early terminal-history purge, and empty history
after same-name recreation under a new UID. Termination must prove physical
stop before success/terminal visibility and stale-owner write rejection.
Physical stop must cover controller loss, restart, stale-owner fencing,
stop-before-start and CPU-bound code, as well as held I/O. Self-host and managed
backends need their own execution proof; publication alone qualifies neither.
