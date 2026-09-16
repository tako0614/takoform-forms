# Forward Workflow execution candidate

Status: **local authoring candidate, not published or supported**. The source is
[`WorkflowCandidateInterface`](../../internal/edgeformcatalog/workflow_candidate.go).
The renderer produces a separate exact closure; it does not register a current
Form, mint a package, activate a Host implementation, or publish a version.

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

## Exact forward closure

| Artifact | Local candidate version | Reference changed |
| --- | --- | --- |
| `worker.workflow` Interface | `2.0.0` | New class and replay semantics |
| `module-worker.workflow` Binding | `2.0.0` | Exact new Interface digest; caller projection only |
| `DurableWorkflow` Form | `0.2.0-workflow.1` | Provides the exact new Interface |
| `WorkerVersion` Form | `0.4.0-workflow.1` | Selects exactly the new workflow Binding and Interface |
| `WorkerDeployment` Form | `0.3.0-workflow.1` | Pins the exact new WorkerVersion Definition |

Numeric Interface/Binding versions are required by the released Core schema;
these unregistered development values do not reserve or publish those numbers.
The enclosing Form versions are prereleases. Other current bindings are
unchanged. `ModuleWorker` and `worker.runtime@1.1.0` remain exact existing
contracts: additional named exports do not alter the default handler ABI or
environment. This candidate does not add handler-free WorkerVersion authoring.
It has no dependency on the separate Vector or Container candidate graphs.

The local renderer is `go run ./cmd/workflow-candidate`; it writes JSON to
stdout only. It is intentionally absent from the current catalog generator,
publication paths, Provider mappings and Host support discovery.

## Implementation acceptance still required

Core validation and exact-reference tests prove artifact construction, not
execution. Host acceptance must exercise this exact candidate through actual
class construction, binding calls, persisted retries and wake, changed-code
replay, retained event races, invalid results, and stop-before-state visibility.
Physical stop must cover controller loss, restart, stale-owner fencing,
stop-before-start and CPU-bound code, as well as held I/O. Self-host and managed
backends need their own execution proof; publication alone qualifies neither.
