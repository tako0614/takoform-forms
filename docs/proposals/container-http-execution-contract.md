# Local ContainerService Form candidate (unpublished)

This proposal records the bounded local Host CRUD candidate rendered by
`internal/edgeformcatalog/edge_container_http_candidate.go`. Its only emitted
portable artifact is `edge.forms.takoform.com/ContainerService@0.1.0` and its
complete Core-verifiable Form Package closure. The command
`go run ./cmd/edge-container-candidate` prints those exact bytes and package
metadata to stdout. The wrapper labels the artifact `UNPUBLISHED` and
`form-only`.

This is not an addition to the registered Edge catalog or its 17 current
published Forms. It does not change current package bytes, current family
manifests, publisher trust, or release history. No Host support, installation,
activation, public endpoint, or availability is claimed. The Form has no
provided Interface in this first slice. A private native HTTP health probe is
only local implementation evidence; it is not a portable invocation contract
or a public route.

Its lifecycle capability list is explicitly limited to create, read, update,
delete, and observe. It does not advertise import: Container adoption/import
identity and its fencing semantics are not defined by this candidate.

## Desired state and ownership

The candidate declares:

| Wire field | Meaning |
| --- | --- |
| `image` | OCI reference pinned to a lowercase `sha256` manifest digest; mutable tag-only references and out-of-range registry ports are rejected. |
| `httpPort` | Private in-container HTTP listener port, 1–65535. |
| `healthPath` | Bounded absolute path with nonempty path segments; query/fragment forms and repeated empty segments are not admitted. |
| `environment` | Optional map with at most 64 ordinary desired-state string values, each at most 4096 characters, persisted/read back as part of `ResourceSpec`. This field is classified as nonsecret configuration: callers **must not** put secrets here. The schema does not automatically detect or reject secret values. Names use Core's exact portable map-key grammar `^[A-Za-z][A-Za-z0-9._-]{0,63}$`; names beginning with `_` are intentionally outside this Form's accepted set. |
| `workloadRevision` | Required bounded caller revision token used as desired-operation input, not as a Host UID or generation. |
| `outboundInternet` | Optional explicit egress choice; omission defaults to `false`. |
| `requiredSensitiveVars` | Optional bounded set of variable names only. No values are represented in portable state. |

Host Offering owns capacity and placement. The resource's desired schema has no
capacity, location, provider, account, vendor-native ID, or public endpoint
field. A separate Host runtime profile owns its backend and egress enforcement.

The local execution slice must refuse any nonempty `requiredSensitiveVars`
before mutation: the current Host API does not provide secret material. This
is a machine-enforced refusal for declared sensitive-variable names only. It
does not inspect or classify `environment` values; callers remain responsible
for keeping secrets out of that ordinary persisted desired state. A sealed,
generation-bound secret-delivery transport is a separate gap and is not
resolved in this candidate.

## Local lifecycle contract

An update starts the exact desired `workloadRevision` as a new execution
revision of the same Resource incarnation. It checks the configured health
path before moving serving traffic. If startup or health fails, the new
revision never serves and the previous serving execution revision remains
selected. A successful health check is the cutover precondition, not a claim
that any current Host has implemented the transition.

The local Host's durable operation record fences the exact resource UID,
desired generation, and operation identity. If an acknowledgement is lost,
read back that same identity before adopting completion or retrying. A
mismatched or absent identity does not authorize adoption or a blind retry;
the prior serving execution revision remains selected while the outcome is
reported unresolved. Deletion is fenced by the exact observed incarnation UID
so a stale request cannot delete a replacement resource.

Restart recovery resumes by reconciling persisted desired and observed
generations plus operation identity. The API request is not a process-control
channel, and process-local handles are not restart authority. Host implementation
must preserve this distinction across ordinary CRUD, error recovery, and
process restart.

## Deliberately unresolved portable invocation

The intended caller-facing shape is a private streaming HTTP fetch through a
Worker Binding, not a public address or a Worker endpoint. This candidate does
not emit `container.http`, `module-worker.container-http`, an Interface digest,
a Binding digest, or a WorkerVersion/WorkerDeployment successor.

Released Core v1.1.0 requires every InterfaceDefinition to declare a consistency
enum whose accepted values are `eventual`, `read_after_write`,
`per_key_linearizable`, and `serializable`. Arbitrary Container HTTP execution
does not by itself promise convergence or cross-request state visibility, so
choosing `eventual` or `read_after_write` would assert behavior not established
by this candidate. The schema is not reinterpreted here. The exact streaming
framing, body/header/path ceilings, pre/post-response-head failures, and
no-transparent-retry rule remain an explicit design gap until they can be
represented without a false consistency claim.

The Workflow owner has reserved WorkerVersion 0.4 / WorkerDeployment 0.3 for
its own predecessor transition. This Form-only candidate does not generate or
alter that pair, nor a later Container successor. Container successors require
their own approved ordering after the exact Workflow predecessor is published.

## Export verification

The small owner checks are:

```bash
go test ./internal/edgeformcatalog -run 'Test(RenderEdgeContainerServiceFormCandidate|ContainerSensitiveRequirements|EdgeContainerCandidate)'
go run ./cmd/edge-container-candidate
```

The renderer calls released Core v1.1.0 Definition/index validation. The
focused test materializes the exported package closure in a temporary directory
and calls Core `formpackage.VerifyDirectory`; its positive fixture and each
schema-negative fixture therefore use the same exact bytes sent to the local
Host test. The command writes only stdout; redirecting it to a temporary
integration artifact does not modify tracked current catalog or release files.
