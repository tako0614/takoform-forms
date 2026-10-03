# Actor and Workflow joint source candidate

Status: **unpublished, unselected source-only candidate**. This document does
not allocate a release version, change the current 17-Form catalog, or claim
Host support. The exact public Actor and Workflow contracts remain the
published bytes in the verified `e7f8a39311dd011b8467e97e7f300cabb9a6b06c`
source. The current Host API remains `forms.takoform.com/v1` and Core remains
`github.com/tako0614/takoform@v1.1.0`.

`go run ./cmd/actor-workflow-candidate` prints one deterministic JSON object to
stdout. It composes existing Actor, Workflow and Worker runtime candidate
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

The numeric prerelease/Interface/Binding values in this source candidate are
Core-valid development identities only. Before any release, the publisher
must choose non-conflicting final identities and rerender the whole exact
closure. Workflow and Actor must not independently publish different
WorkerVersion/WorkerDeployment successors. Immutable earlier Form Packages,
trust sets, release roots and tags remain readable and byte-identical; a
future publisher-set successor is a separate reviewed publication decision.

The complete candidate's Form definitions, Interface and Binding definitions,
fixtures and exact references are checked by focused Go tests. Those tests
also stage each new Form as a real Core v1.1.0 Package and compile a Snapshot
with all current Forms, the nine forward Forms, and their old/new exact
Interface/Binding artifacts. This proves data-contract closure, **not**
executable tenant JavaScript, Sigstore evidence, Host admission, or support.

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
semantic proposals. Their choices require explicit publisher adoption before
these generated development identities can become current candidates.

For the new Workflow successor only, queued/running/sleeping/waiting instances
are dependent execution identities under their Workflow UID. Generic DELETE
refuses without mutation as `dependency_in_use` (409) until all are terminal
and every owner/continuation is physically stopped and fenced. This follows
the existing Host API v1 rule in `takoform/spec/host-api/v1.md` §Relations and
bindings: deletion of a live target with dependents is `dependency_in_use`.
It does not reinterpret the frozen API or alter old Workflow bytes. In the same
frozen API, `resource_busy` is automatically retryable, so it must not label
this potentially long-lived lifecycle refusal.
