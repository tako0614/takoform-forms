# Actor and Workflow executable input — source only

This is the first **unpublished input slice**, not an executed conformance
result. `manifest.json` pins the nine selected FormRefs, three Interface
Definitions and two Binding Definitions from `cmd/current-form-source`, plus
the exact bytes of `bundle.mjs` and `cases.json`. These local fixture format
labels do not create a Takoform Specification or release version.

The bundle exports ordinary named Actor and Workflow classes and a plain-object
default Worker handler. `OrdinaryActor` supplies all five required prototype
handlers. `DeepInheritedActor` inherits all five through a finite 129-link
ordinary chain and has no `start`. The three Proxy module factories supply
cyclic, changing and non-returning prototype traps. A consumer must invoke
the non-returning case only inside a killable inspection child; importing this
module never invokes that trap. `SignalWorkflow` uses one durable step and one
typed event wait. These inputs require no vendor base class or application name.

`cases.json` records **expectations**, not results. In particular, a finite
deep chain is not disallowed by a public 128-prototype rule: the selected
contract states no such limit. A cyclic/changing chain must fail closed, and a
non-returning trap must not hold Host admission indefinitely. A Host-specific
adapter must supply the exact selected candidate package closure, class
inspection, a closed declared environment, resource lifecycle, Actor/Workflow
execution and failure injection. The same input and expectations must then run
against independent self-host and Workers for Platforms adapters. Adapter
absence is **NOT QUALIFIED**, never a skipped pass or synthetic green result.
Local managed-runtime simulation cannot qualify the actual WfP service.

The one focused publisher test below checks fixture integrity against the
owning selected Definition renderers and hashes. It does not execute tenant
classes or prove Host support:

```console
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/edgeformcatalog -run '^TestActorWorkflowExecutableInputPinsSelectedDefinitionsAndBundle$' -count=1
```

Still absent: executable cross-backend SQL/transaction and alarm settlement;
same-ID admission across streams, restart and rollback; one-shot WebSocket
broker and capacity/failure behavior; Workflow retry/replay, signal races,
termination and deletion fencing; and consumer end-to-end evidence. The old
published package bytes, Host API v1, trust evidence, and current signed set
are untouched by this fixture.
