# Container Host API v2 candidate

This change restores the minimal private HTTP Container lifecycle from main's
unpublished ContainerService 0.1.0. It adds a URL-identified Container contract
and its container.http Interface / module-worker.container Binding, plus
WorkerVersion 0.6.0 and WorkerDeployment 0.5.0 for consumption. None is frozen
or selected for publication. The existing 17 frozen v2 Forms and every v1
package, release, trust set and tag keep their bytes and meaning.

The initial request names the resource Container and the Binding
module-worker.container. The Takoserver survey proposes ContainerService and
module-worker.container-http instead. These names require agreement before
merge; no aliases or implicit state conversion are introduced. Both proposals
use containerBindings[{name,resource:{resourceUid}}] and env.APP.fetch(Request).
The Binding is a typed runtime capability, not a second CRUD Resource. Its
normative identity is the Container Form URL's worker-binding chapter; no
independent v1 package/consistency enum or Binding release line is needed.

The source retains image digest pinning, httpPort, healthPath, nonsecret
environment, workloadRevision, outboundInternet:false by default, and
names-only requiredSensitiveVars. Like the source candidate, the initial
profile rejects nonempty secret declarations. Enabled egress is a required
behavior for valid outboundInternet:true inputs; existing runtime refusal is
an implementation gap, not permission to advertise full Form support.

Linux means a Bun/systemd Host using Docker Engine on Linux, as proposed by the
Takoserver owner. A daemon-free OCI backend is separate work. Every backend
uses linux/amd64 and the same HTTP contract. Cloudflare's limit is 100 named
image entries per application with the durable_object policy, not 100 Takoform
Forms or a global Resource limit. Registry ingestion, retention and capacity
admission belong to the Host. Running instances need explicit healthy cutover
after image map changes.

WorkerVersion 0.6.0 preserves the ModuleWorker 0.3.0 ABI and all prior Binding
contracts, adding containerBindings. WorkerDeployment 0.5.0 enumerates both
exact WorkerVersion URLs (0.5.0 and 0.6.0), so it can choose existing versions
without rewriting them. It must participate in the same single-active fence
as WorkerDeployment 0.4.0; mixed deployments cannot serve one Worker. Existing
ModuleWorker runtime and attachment behavior is retained. No previously
published resource is silently migrated to a successor URL.

## Historical basis

- Provider history has published pre-Beta ContainerService definitions under
  forms.takoform.com/v1alpha1 and immutable forms/k-inxw45dbnfxgk4stmvzhm2ldmu
  tags. PR82 restarted the experimental epoch; PR148 withdrew pre-Beta epochs
  with explicit retired identities. Those URLs are not reused.
- Forms commit d5e69bfc19155b2c35726e7ca1d96ef4224e2b3e (2026-08-27)
  removed the five unpublished container.forms.takoform.com candidates. Its
  message and TASK-0036 record the maintainer's Edge-first release scope.
- [PR6](https://github.com/tako0614/takoform-forms/pull/6), merged
  2026-10-02, restored source-only ContainerService/ContainerEndpoint. Main's
  container_forms.go and source-candidates remain intact. Its v1 HTTP Binding
  gap does not prohibit a v2 Form-owned runtime contract.
- TASK-0090 established the current 17 v2 exact Form URLs, preserving the
  separate Container work. TASK-0098 owns Provider v2 transport and codecs;
  this PR does not register Provider resources or change persisted state.

## Task ledger and verification boundary

Only takoform-forms changes. Takoform, Core, Provider, Takoserver and control
are read-only references. Work order: full-ref/deletion/tag/release history,
HISTORY.md, SPEC.md, isolated branch, source/conformance, complete local gate,
draft PR. The external operator-run HISTORY.md/SPEC.md hold the detailed
investigation and proposed coordination points. This ledger covers a new
public spec shape and authority-bearing capability; it grants no deployment
authority.

The publisher owns normative source and reusable conformance cases. Host owners
must qualify image retention/registry authorization, cold start, generation
cutover and physical retirement, reference grants and revocation, egress,
framing/cancellation, cross-version deployment exclusion, and real process
restart. Test doubles and local publisher CI are not native backend evidence.
Takoserver review is required before integration. Merge, publication, signing,
production deployment and force-push remain outside this request.
