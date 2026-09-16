# VectorIndex — forward contract proposal

Status: **unaccepted, non-normative proposal**, 2026-09-09. This document
does not allocate an API version, register a catalog member, create a
publishable package, or change any existing package bytes. It records one
selected candidate profile; it is not a menu of alternatives.

## Future identity allocation (not assigned)

If the profile is accepted in a later publication decision, the proposed
numeric identities are:

| subject | proposed identity |
| --- | --- |
| Form | `VectorIndex@0.1.0` |
| Interface | `edge.vector@0.1.0` |
| Binding | `module-worker.edge-vector@1.0.0` |
| WorkerVersion | `0.4.0` |
| WorkerDeployment | `0.3.0` |

These are future allocations only. The development renderer emits an
unregistered `VectorIndex@0.1.0-dev.1` and `WorkerVersion@0.4.0-dev.1`; Core
requires numeric SemVer for the unregistered Interface and Binding, so it
emits `edge.vector@0.1.0` and `module-worker.edge-vector@1.0.0` without making
either identity publishable. The renderer also emits an unregistered
`WorkerDeployment@0.3.0-dev.1` whose version relation pins the exact candidate
WorkerVersion Definition. The existing published Deployment continues to pin
the existing WorkerVersion; neither reference is widened or reinterpreted.
No candidate artifact is registered or offered by a production Host.

`go run ./cmd/vector-index-candidate` prints the Core-validated development
artifacts `{form, interface, binding, workerVersion, workerDeployment}` and performs no catalog
or release write. Existing WorkerVersion bytes and its seven accepted Binding
identities remain unchanged.

## Selected resource profile

VectorIndex is an install-owned Resource addressed by its Host Resource UID.
The provider's native index, table, endpoint, credentials, account, and native
ID are Host-private. A provider may use pgvector or another implementation,
but must provide the same observable semantics below.

The immutable configuration is:

- `dimension` is an integer from 1 through 1536. Every stored and query vector
  has exactly this length; a host rejects a mismatch rather than padding or
  truncating.
- `metric` is the literal `cosine`; no other metric is substituted.
- `filterKeys` is an optional immutable set of at most eight simple ASCII
  identifier keys (`[A-Za-z][A-Za-z0-9_]{0,63}`). The candidate's canonical
  Form fixture declares `spaceId`; its alternate immutable example is empty.
  Observed `filterKeys` are emitted in lexical order, matching canonical set
  defaults. A key not declared by the Resource is invalid in a query filter.

IDs and namespaces are opaque strings of at most 128 Unicode code points. NUL
is forbidden in both; `*` is additionally forbidden in namespaces. Omitting
`namespace` selects the empty namespace and never means “all namespaces”. IDs
are unique within `(namespace, id)`, so the same ID may occur in different
namespaces. Every vectors or IDs array contains 1 through 100 entries; an
empty upsert, get, or delete request is `invalid_spec`.

## Vector and metadata representation

Before a vector is stored or used for cosine similarity, every component is
converted to IEEE 754 binary32. The conversion itself is validated: a value
that converts to a non-finite binary32 value, or whose converted vector has a
zero cosine norm, is `invalid_spec`. Returned vector values are those canonical
binary32 values, not the caller's wider input spelling. Repeating an upsert
with the same `(namespace, id)`, canonical binary32 values, and metadata is
idempotent by value.

Metadata is a flat JSON object whose values are only string, finite number,
boolean, or null. Keys use the same simple ASCII identifier grammar and there
are at most 64 properties. The size limit is measured on the UTF-8 bytes of
the RFC 8785 canonical JSON encoding of the complete metadata object and is
8192 bytes. Nested objects, arrays, non-finite numbers, and other shapes are
`invalid_spec`; providers do not silently drop or reinterpret them.

## Operations

The Interface exposes only `upsert`, `get`, `delete`, and `query`. Every input
is one closed JSON object; unknown members, missing required members, wrong
types, and resource-dependent dimension/filter violations are `invalid_spec`.
The only operation error codes are `invalid_spec`, `quota`, and `unavailable`.

### `upsert`

`upsert` accepts `{namespace?, vectors}`. Each record has `id`, `values`, and
optional `metadata`. The complete request is validated before effects. An
`invalid_spec` therefore has zero effects. An existing `(namespace, id)` is
replaced with the complete canonical vector and metadata; there is no
create-only `insert` operation. Duplicate IDs in one upsert are
`invalid_spec`.

On success, every input record is durably accepted and the output is
`{ids, count}` where `ids` are exactly the input IDs in input order and
`count == len(vectors)` (there is at least one input). Omitted `metadata` is
materialized as `{}` and fully replaces or clears prior metadata; it is never
merged. A `quota` or `unavailable` failure may leave a partial batch.
Retrying the whole call with the same canonical values and metadata is
idempotent; this profile makes no cross-provider atomic-batch promise.

### `get`

`get` accepts `{namespace?, ids}` and returns `{vectors}`. Visibility is
eventual after mutation. Existing records that are visible are returned in
request ID order; missing IDs are omitted. An omitted namespace means only
the empty namespace and never all namespaces. A recent upsert may be absent
until it becomes visible and a recent delete may remain visible temporarily.
The operation has no `not_found` result.

### `delete`

`delete` accepts `{namespace?, ids}` and is idempotent. Unknown IDs are no-ops.
On success it returns `{ids, count}` with the exact accepted request IDs in
request order, including unknown IDs, and `count == len(ids)` (there is at
least one requested ID). Neither the IDs nor count is an existence proof. A
`quota` or `unavailable` failure may leave a partial batch; retrying the whole
call with the same IDs is safe and idempotent. A successful unbound Resource
deletion has application-visible absence; subsequent get/query results become
absent under the same eventual visibility rule.

### `query`

`query` accepts `{namespace?, values, topK, filter?, returnMetadata?,
returnValues?}`. Namespace selection and every declared filter term are
applied first. Filter terms use exact, type-sensitive equality and are joined
with AND before approximate nearest-neighbor selection. A missing or empty
`filter` imposes no metadata restriction. `topK` is 1 through 100. The
provider returns at most `topK` matches with a finite cosine score in `[-1, 1]`,
sorted by descending score; ties have unspecified order and there is no
exact-nearest-neighbor promise. After accepted mutations affecting the selected
namespace and filter have converged, and while no concurrent writes are
present, let `eligibleVisibleCount` be the number of unique visible records
that satisfy that namespace and filter. A successful query returns exactly
`min(topK, eligibleVisibleCount)` matches, with each `(namespace, id)` appearing
at most once. Before convergence, eventual visibility may return fewer matches.
The output `count` is exactly `len(matches)`.

When `returnMetadata` is true, every match carries `metadata`; when it is
false or omitted, no match carries that field. The same rule applies to
`returnValues` and `values`. No output fabricates an omitted field.

The Interface declares eventual consistency, no pagination, and no stable
ordering across ties. The Form's observed `count` is the eventual total across
all namespaces and may lag a completed mutation. Observed `filterKeys` retain
the lexical ordering used by canonical set defaults.

## Lifecycle and binding boundary

The Form claims only create, read, delete, and observe. Dimension, metric, and
filterKeys are immutable; changing any requires an explicitly reviewed
replacement or migration outside this candidate. Deleting a Resource while a
binding targets it is refused. Deleting an unbound Resource succeeds only when
application-visible absence is established. Physical media retention,
tombstones, and purge timing are operator policy, not portable semantics.

The forward WorkerVersion adds `vectorBindings` (`vector_bindings` in HCL) to
the existing environment-name namespace. Each binding targets exactly the
candidate Form and `edge.vector@0.1.0`; an omitted or empty list declares no
Vector binding. `env.NAME` exposes only `upsert(input)`, `get(input)`,
`delete(input)`, and `query(input)`. Each method accepts exactly one closed
operation object and returns a Promise for the exact operation output
document. The binding exposes no vendor methods, native IDs, endpoints,
credentials, or backend metadata.

## Fixture and acceptance boundary

The data-only Interface fixtures run against a fresh VectorIndex configured as
`{dimension: 3, metric: "cosine", filterKeys: ["spaceId"]}` and include
deterministic expected outputs where the Core fixture format can express them
(for example, successful mutation IDs/counts and an empty query in a fresh
scope). They do not claim immediate get/query visibility after a mutation and
contain no polling or sleep step. Core's `InterfaceFixtureStep` format has only
`operation`, `input`, `expected`, and `expectedError`; it cannot represent a
settled precondition or a polling sequence. This candidate therefore does not
add a positive query-after-upsert fixture or claim immediate visibility.

Host conformance must prove the positive query cardinality outside the static
fixture, using only the declared operations:

1. In a fresh configured index, choose `N` explicitly in `1..100`, then
   `upsert` `N` records with distinct IDs in one namespace, all satisfying the
   selected filter, and require the successful acceptance response
   (`count == N`).
2. Stop issuing writes to that namespace/filter so no concurrent writes remain.
3. Poll `query` with `topK: N` until all `N` eligible matches are visible; this
   is the host harness's convergence witness. The harness uses a finite timeout
   chosen by target-qualification infrastructure; that timeout is not a
   portable consistency SLA. If it expires before convergence, target
   qualification fails—never poll indefinitely or treat timeout as success—and
   no fixture field/API is invented.
4. Query with `topK` values below, equal to, and above `N` (where valid) and
   assert exactly `min(topK, eligibleVisibleCount)` matches, unique by
   `(namespace, id)`, with finite descending cosine scores and unspecified tie
   order. A host may use a different existing visibility witness, but it must
   establish the same settled, no-concurrent-writes precondition before this
   assertion.

The same host-side polling rule, including a finite target-qualification
timeout that fails the qualification when it expires, applies before asserting
eventual read/query absence after deletes; these are later Host evidence, not
static Interface fixtures.

Publication, catalog registration, Host admission, provider materialization,
replacement migration, retention implementation, and consumer interoperability
are separate decisions. None is implied by this unaccepted candidate or by a
local renderer run.
