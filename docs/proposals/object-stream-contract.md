# Unknown-length ObjectBucket streams

This source candidate adds three Host API v2 Form definitions:

| Form | New source | Preserved predecessor |
| --- | --- | --- |
| ObjectBucket | [0.3.0](../../spec/forms/ObjectBucket/0.3.0/index.md) | 0.2.0 |
| WorkerVersion | [0.7.0](../../spec/forms/WorkerVersion/0.7.0/index.md) | 0.5.0 and 0.6.0 |
| WorkerDeployment | [0.6.0](../../spec/forms/WorkerDeployment/0.6.0/index.md) | 0.4.0 and 0.5.0 |

The normative bodies are Japanese. These new source identities are unpublished;
freezing or merging them does not establish public URL availability, Host support
or runtime conformance. The [data corpus](../../conformance/host-v2/object-streams/README.md)
states observations a future runtime suite must measure. It is not an executed
suite.

## Why new source is needed

ObjectBucket 0.2.0 requires a supplied exact contentLength for a ReadableStream
and sends an unknown-length producer to multipart. Its multipart parts still
require known lengths. WorkerVersion 0.5.0 and 0.6.0 select that exact bucket
Form. Omitting length in an implementation cannot change those contracts.

The retained v1 edge.objects 1.0 Interface also requires an exact integer length
in its PUT and uploadPart documents. V1 package definitions and digests remain
unchanged. This candidate adds no v1 Interface, Binding, package selection or
Core schema. A v2 Form is its own contract, rather than an alias for a v1
Interface or a provider-specific transport.

## Observable change

ObjectBucket 0.3.0 permits omitted contentLength for a byte stream in put and
uploadPart. A supplied number remains an exact promise; zero is a present empty
body whose normal EOF must be observed. Explicit undefined and null are invalid
values, not another spelling of omission. Argument validation precedes body
consumption. Known and unknown streams retain the method's existing byte ceiling
and error vocabulary.

The Host validates and fixes the original destination, conditions, metadata and
supplied count before asynchronous staging. It owns snapshots of inline body
bytes, consumes each producer at most once with backpressure, and verifies actual
normal producer EOF before publishing any replacement. Private bounded durable
staging is permitted; whole-body RAM buffering merely to discover a length is
not. Private staging cannot become public Resource state or a provider handle.

Count mismatch, non-byte data, overflow, producer error and cancellation before
validated EOF preserve the previous object or completed part. Conditions apply
atomically at final object publication. Losing an acknowledgement after an
authorized commit can remain ambiguous: neither Promise rejection nor head
alone authorizes an automatic replacement. Required private custody survives
that uncertainty until the original operation can safely be settled. This adds
no public retry key, management Operation or status endpoint.

The 979-byte key, 300-MiB put, 5-GiB object and 10,000-part ceilings remain.
Every nonfinal multipart part is still at least 5 MiB, and all nonfinal parts
have the same byte length. Private native re-chunking does not relax these
portable completion constraints. Metadata, strong etags, read-after-write,
range/list semantics and the JavaScript result types remain unchanged.

## Exact graph and compatibility

This candidate inherits the Container source prerequisite from
`tako0614/takoform-forms` PR #17 at commit
`55811b390dbae8bd0c67a65e471d68d3ffb72006`: Container 0.1.0,
WorkerVersion 0.6.0 and WorkerDeployment 0.5.0. It preserves those files and
their containerBindings rules. WorkerVersion 0.7.0 retains ModuleWorker 0.3.0's
module, default handler, Actor/Workflow class and asset ABI. No new export,
worker.runtime Interface, Host API protocol or aggregate family version is
introduced.

WorkerVersion 0.7.0 admits exact ObjectBucket 0.2.0 and 0.3.0 targets. The Host
seals each target UID, authorization and actual Form URL, then chooses that
Binding's semantics from the accepted target. A mixed Version can have a legacy
and a forward bucket under different names. A caller flag, kind or mutable
default cannot choose this behavior. Old buckets and old Versions keep their
existing exact-length contract.

WorkerDeployment 0.6.0 enumerates WorkerVersion 0.5.0, 0.6.0 and 0.7.0 as its
accepted Version Forms. Each selected execution keeps its sealed snapshot;
changing the distribution does not reinterpret an in-flight execution.

Runtime adoption must retain the original facade/module bytes, implementation
identity and closure digest for reading and recovering accepted old Versions.
New facade bytes produce a new closure for the forward selection only. A new
compiler default cannot reconstruct an old accepted closure with changed bytes.
This source change contains no Host, SDK, compiler or native implementation.

## Source integration and publication

The owning publisher can review, check and merge these exact source definitions
before they are published. An integration implementation can pin that source
explicitly and qualify its real runtime separately. Source-only checks do not
prove that such an implementation exists or passes native or staging E2E.

The three raw normative sources append to spec/forms.freeze.json using the
existing publisher-local freeze procedure. This source candidate in
[PR #20](https://github.com/tako0614/takoform-forms/pull/20) also owns its complete
freeze closure: it explicitly appends the unchanged Container 0.1.0,
WorkerVersion 0.6.0 and WorkerDeployment 0.5.0 source inherited from exact
PR #17 commit `55811b390dbae8bd0c67a65e471d68d3ffb72006`. Their source bytes remain
unchanged; this adds eligibility anchors without changing PR #17 or merging it
into main. All 24 preceding freeze entries and all normative source bytes remain
unchanged when these three prerequisite anchors are appended. There are then
27 authored and 27 frozen Form documents, without pending source dependencies.
Existing first-add anchors, packages and tags retain their original identities.

Actual publication remains the publisher's separate owning deployment surface
and readback procedure in [site maintenance](../site.md). This proposal adds no
deploy entrypoint or publication action, and does not advertise any new URL as
publicly available.
