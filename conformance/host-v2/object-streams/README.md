# Object stream contract vectors

This data-only corpus describes the new source contracts
[ObjectBucket 0.3.0](../../../spec/forms/ObjectBucket/0.3.0/index.md),
[WorkerVersion 0.7.0](../../../spec/forms/WorkerVersion/0.7.0/index.md) and
[WorkerDeployment 0.6.0](../../../spec/forms/WorkerDeployment/0.6.0/index.md).
It includes negative compatibility observations for ObjectBucket 0.2.0 and
WorkerVersion 0.5.0/0.6.0. All target identities denote authored source; none
asserts public availability, implementation or an executed test result.

## Reading cases.json

Each case has an id, operation, exact source selection, harness input and expected
observations. Operation names put, uploadPart and completeMultipartUpload refer
to the JavaScript Binding. selectBindings and selectDeployment describe Host
reference selection, not additional public methods. The expected object describes
observations for a future conformance runner; it is never a fabricated native
receipt or a new API response.

Inputs are JSON descriptions of fixture construction:

The default destination is `original-key`, initially containing bytes
`[111,108,100]` with `original-etag`; a fixture upload likewise has an old part
at number 1. The create-if-absent race starts with no destination. The two
condition-race cases then publish their explicitly described concurrentWinner
bytes, ETag and content type after staging and before the attempted final write.
Those winner values replace the initial sentinel or initial absence as the
post-refusal oracle. All values are dummy fixtures. Each case starts from fresh
isolated state. put inputs
omit the key when they use that default. Multipart and reference-selection cases
use their explicitly described metadata instead of materializing a whole body.

- `body.kind: stream` constructs a fresh ReadableStream. Each `chunks` element
  contains literal `bytes`, a `repeatByte` with `count`, or `nonByte`. Repeated
  bytes must be generated incrementally in bounded chunks, never allocated as
  one object. `end` is normal `eof` or `producer-error`. `string` and
  `array-buffer` describe the other two accepted body types.
- `options` contains actual JSON option values. Absence of contentLength means
  the field is omitted. `specialOptions` describes values JSON cannot encode:
  undefined, NaN, positive-infinity or an unsafe integer. The harness constructs
  the actual JavaScript value; it must not pass these spelling strings to a
  Binding.
- `mutationAfterCall` changes the original options object or ArrayBuffer after
  the synchronous invocation, before the first staging await resolves. Expected
  observations refer to the original fixed input.
- `control` names a fixture-only pause, transport termination, cancellation,
  lost final acknowledgement or destination precondition race. These controls
  are injected at the named phase; they are not public options, endpoints or
  provider mutations supplied by an application. For either condition race,
  concurrentWinner describes the controlled competing writer's exact object;
  the attempted operation must preserve that winner, including its metadata.
- `parts` gives numbered, already stored part metadata. `size` is fixture data
  retained by the Host, not an additional field in a JavaScript completion
  member. The actual completion call passes only partNumber and etag.

`expected.promise` is resolve, reject or unobservable. error is the observable
Error.name or TypeError; publication is none, committed or unknown for the
attempted operation only. It does not describe the competing writer's publication.
The two condition-race cases require concurrentWinnerPreserved and destinationAfter
to match the winner's explicit bytes, ETag and content type, with no publication
by the attempted operation. Restoring the initial sentinel or initial absence is
an incorrect result. For other rejected pre-publication cases, oldObjectPreserved
or oldPartPreserved means exact original bytes and metadata, not merely a matching
object count. For ambiguous outcomes,
originalCustodyHeld and automaticReplay count refer to private Host observations;
they add no public status API. A successful size is the actual committed byte
count and requires byte-exact readback with matching strong metadata.

## Required observations

A future runtime suite must control producer completion independently of HTTP
EOF. Pause before EOF and prove the destination still has its old bytes; then
release normal EOF and observe the successful write. Exercise known and unknown
empty bodies, count mismatch, non-byte chunks, producer errors, truncation,
backpressure, cancellation, bounded cleanup, conditions at final publication,
metadata/input snapshots and lost acknowledgements. Abort or failure before a
commit cannot publish partial bytes or replace a previously completed part.

The boundary vectors include exact put/object ceilings and the retained
multipart equal-size rule. They are instructions for bounded future execution,
not permission to materialize GiB fixtures in a source-only check. A native
runner must preserve exact byte counts and readback; reduced limits are not proof
of the declared portable limit.

The backpressure case compares buffered byte growth against the runtime's
declared finite buffering window while its stage sink is paused. It permits
bounded stream prefetch; it does not assert an implementation-specific exact
pull count. The runner must observe the producer stopping within that window
and resuming when the sink is released.

Reference vectors require old0.2 and new0.3 bucket behavior to coexist in one
Version0.7 under their actual accepted target Forms. Existing Version0.5/0.6 and
Deployment0.4/0.5 definitions stay unchanged. A complete runtime suite must also
read and recover recorded old closures byte-for-byte when new compiler defaults
exist.

## Validation boundary

Parsing this JSON, validating its fixture descriptions, checking relative source
links and verifying the publisher-local source freeze are static source checks.
They do not execute a Host, SDK, compiler, SQLite owner, provider or native data
plane. There is no runner or new CLI in this candidate. Runtime assertions remain
unmeasured until an owning implementation consumes the corpus with genuine
controlled streams and saved readback evidence.
