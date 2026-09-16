# Aggregate Actor + Workflow + Vector runtime candidate

Status: **unaccepted, non-normative development candidate**. This renderer
does not allocate a release version, register a catalog member, create a Form
Package, advertise Host support, or change any published bytes. It exists to
exercise one exact forward closure after the Actor, Workflow and Vector
proposals are reviewed together.

## One closure

`go run ./cmd/runtime-candidate` prints one JSON document containing:

- the forward `worker.runtime@2.0.0` candidate and its `ModuleWorker` Form;
- the aligned Actor Form/Interface/Binding pair;
- the reviewed Workflow Form/Interface/Binding pair;
- the reviewed Vector Form/Interface/Binding pair;
- the four existing inward-activation Forms whose worker relation genuinely
  requires `worker.runtime`;
- exactly one aggregate `WorkerVersion` and one aggregate
  `WorkerDeployment`.

The numeric Interface and Binding values are only Core-valid development
identities. The enclosing Forms use prerelease development identities. Final
release allocation is a later qualification decision; no value printed here
is a registry or publication authority.

The aggregate WorkerVersion keeps the seven existing Binding names, replacing
the `module-worker.actor` and `module-worker.workflow` references by the exact
candidate Binding digests and appending `module-worker.edge-vector` once. Its
`worker` relation and every true dependant's worker relation require the exact
forward `worker.runtime` Interface digest. The WorkerDeployment pins the exact
aggregate WorkerVersion and forward ModuleWorker Definition refs. No stale
runtime/interface digest is tolerated, and no independent Workflow or Vector
WorkerVersion/Deployment is included.

Actor's forward Interface carries the selected class/context, retirement,
streaming, alarm and socket rules. In particular, upgrade reservations are
bound to the original HTTP invocation; only the branded `Response` initializer
alias preserves one reservation, clone throws `TypeError`, and an unbranded
status-101 response throws `RangeError`. At outer commitment absent handshake
fields may be filled, while conflicting reserved handshake headers abandon the
reservation and provisional sends and return HTTP 502 before any 101. The
candidate uses the resolved limits (10,000 connections, 33,554,432 encoded
frame/queue bytes, 16,384-byte attachments and a 123-byte UTF-8 close reason),
including the omitted-code/reason close defaults.

The closure contains no new Container, Queue-family or AI Forms. Existing
catalog entries, API/Core schemas, publisher release streams, package bytes,
and support/adoption flags remain untouched. Rendering is in-memory and
Core-validates every emitted Interface, Binding and Form; focused tests also
prove deterministic output, exact digest propagation and current-catalog
immutability.

The current released schemas were sufficient for this declarative candidate;
the renderer did not invent runtime fields or an empty artifact to fill a
missing schema seam. Host class execution, socket transport, retirement and
provider interoperability still require independent qualification before any
adoption decision.

