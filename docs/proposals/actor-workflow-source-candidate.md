# Actor and Workflow joint source candidate

Status: **unpublished, selected source-only candidate**. Phase 1 selects the
joint composition in `cmd/current-form-source`; it does not write generated
packages, change the published 17-Form release, or claim Host support. The
exact public Actor and Workflow contracts remain the published bytes in the
verified `e7f8a39311dd011b8467e97e7f300cabb9a6b06c` source. The current
Host API remains `forms.takoform.com/v1` and Core remains
`github.com/tako0614/takoform@v1.1.0`.

`go run ./cmd/actor-workflow-candidate` prints one deterministic JSON object to
stdout with its original prerelease development identities. It composes existing Actor, Workflow and Worker runtime candidate
renderers with **one** shared WorkerVersion and WorkerDeployment. The candidate
contains two changed capability pairs (`worker.actor` with
`module-worker.actor`, `worker.workflow` with `module-worker.workflow`), one
forward `worker.runtime`, and nine Form successors: ModuleWorker,
ActorNamespace, DurableWorkflow, WorkerVersion, WorkerDeployment,
WorkerCustomDomain, WorkerEndpoint, WorkerCronTrigger and QueueConsumer. The
last four change only their exact runtime requirement; they do not acquire
new domain behavior. VectorIndex, Container, AIForms and provider-specific
capabilities are not in this selection. The existing broader
`runtime-candidate` command remains a separate source-only experiment, not a
publication input.

`cmd/current-form-source` alone selects the joint composition with the final
source target Form versions below. It does not change the stdout or bytes of
either development-candidate command.

The selected source targets ModuleWorker, ActorNamespace, DurableWorkflow,
WorkerCustomDomain, WorkerEndpoint, WorkerCronTrigger and QueueConsumer at
definitionVersion `0.2.0`, WorkerVersion at `0.4.0`, and WorkerDeployment at
`0.3.0`. Its three Interfaces remain worker.runtime `2.0.0`, worker.actor
`2.0.0`, worker.workflow `3.0.0`; its two Bindings remain module-worker.actor
`2.0.0` and module-worker.workflow `3.0.0`. These are **UNRELEASED source
targets**, not an aggregate version stream or published identities. The
standalone Actor/Workflow and broader runtime+Vector development candidates
keep their separate draft identities. Selection preserves the other eight
Forms, five Interfaces and five Bindings byte-for-byte. Workflow and Actor
must not independently publish different WorkerVersion/WorkerDeployment
successors. Immutable earlier Form Packages, trust sets, release roots and
tags remain readable and byte-identical; generation, review and a future
publisher-set successor are separate publication decisions.

The complete source candidate's Form, Interface and Binding definitions,
fixtures and exact references are checked by focused Go tests. Those tests
stage each new Form as a real Core v1.1.0 Package and compile a Snapshot with
all currently released Forms, the nine selected Forms, and their old/new exact
Interface/Binding artifacts. This proves data-contract closure, **not**
executable tenant JavaScript, Sigstore evidence, Host admission, or support.

The first exact-digest-bound JavaScript input in
`conformance/edge-runtime/actor-workflow/` is also source-only. Its integrity
check pins the bundle and selected Definitions; without a consumer Host adapter,
none of its case expectations has run. It does not adopt the semantic proposals,
change the signed current publisher set, or qualify either backend.

Before Host enablement, an independent executable corpus must run the same
generic bundle on self-host and Workers for Platforms. It must cover class
inspection and closed environment, private SQL and alarm retry, one live
actor context per ID across restart/update/rollback, response-head streaming
and producer drain, one-shot middleware-transparent WebSocket upgrade and
socket failure/capacity behavior, and namespace deletion fencing/purge.
Workflow instances need their own retry/replay, event-signal, active-delete
refusal, retention and compatible-version checks. A backend implementation
cannot substitute its native class base, credential plane or application
name for these portable contracts. See
[Actor execution](actor-execution-contract.md) and
[Workflow execution](workflow-execution-contract.md) for the unaccepted
semantic proposals. Source selection alone does not adopt them as released
contracts or qualify either Host backend.

For the new Workflow successor only, queued/running/sleeping/waiting instances
are dependent execution identities under their Workflow UID. Generic DELETE
refuses without mutation as `dependency_in_use` (409) until all are terminal
and every owner/continuation is physically stopped and fenced. This follows
the existing Host API v1 rule in `takoform/spec/host-api/v1.md` §Relations and
bindings: deletion of a live target with dependents is `dependency_in_use`.
It does not reinterpret the frozen API or alter old Workflow bytes. In the same
frozen API, `resource_busy` is automatically retryable, so it must not label
this potentially long-lived lifecycle refusal.
