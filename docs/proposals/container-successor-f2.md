# Container 0.2.0 successor source decision

This source decision answers the five open questions on
[PR #17](https://github.com/tako0614/takoform-forms/pull/17) with concrete
successor texts and fixture expectations:

- `spec/forms/Container/0.2.0/index.md`
- `spec/forms/WorkerVersion/0.8.0/index.md`, which adds Container 0.2.0 as a
  Binding target
- `spec/forms/WorkerDeployment/0.7.0/index.md`, which accepts WorkerVersion
  0.8.0 under the existing single-active fence
- `conformance/host-v2/container/0.2.0/`

The three new source identities are frozen in `spec/forms.freeze.json`.
This is source adoption, not publication, Host support, or runtime
qualification. Container 0.1.0, its 422 refusal, and every predecessor
identity are unchanged.

## Common API rules this text relies on

Host API v2 already defines these rules, so the successor states only what the
Form must add:

- **Omission on PUT:** omitting `privateInputs` keeps configured values. The
  Form must define removal and rotation (`http.md`, Update).
- **Replay:** private inputs take part in request equality without exposing
  values. A different map under the same key gives `409 idempotency_conflict`.
- **Replenishment:** an expired value for a provably unsent step uses
  `waiting_input` and the entire original map.
- **Capability:** the presence of `privateInputs`, even `{}`, requires
  `privateInputs:true` in both Discovery and this URL's support response.
  Otherwise the response is `422 capability_required`. A non-object map or a
  non-string value breaks the common envelope (`400 invalid_request`).
- **Support:** support is per exact Form URL. A Host may report
  `privateInputs:false` when the Form has valid secret-free uses.

## Adopted source decisions

**A. Identity.** The successor is `https://edge.forms.takoform.com/forms/Container/0.2.0/`. It keeps:

- `Container`, `container.http` and `module-worker.container`
- `containerBindings:[{name,resource:{resourceUid}}]`
- `env.NAME.fetch(Request)`

It adds no public field, alias, wrapper or Binding method. WorkerVersion
0.6.0 and 0.7.0 name Container 0.1.0 as their only Container target, so
WorkerVersion 0.8.0 copies 0.7.0 and also accepts Container 0.2.0, sealing each
reference's exact URL. WorkerDeployment 0.7.0 copies 0.6.0, also accepts
WorkerVersion 0.8.0, and extends the single-active fence to 0.4.0–0.7.0. This
answers question 5's identity part for staging. The WfP lane confirmed that
the recorded changes replay exactly onto its WorkerVersion 0.7.0 and
WorkerDeployment 0.6.0 source. F1 caller adoption remains separate.

**B. Secrets.**

- `requiredSensitiveVars` declares up to 64 names and is compared as a set.
- CREATE needs a map with exactly the declared names. Each value is 1–16384
  Unicode scalar values with no NUL, and all values together are at most
  65536 UTF-8 bytes.
- The **configured secret map** is the latest accepted CREATE or PUT
  generation's names and values, even when that generation failed. A DELETE
  generation never replaces it. PUT compares and inherits against it, never
  against the serving generation. A successful DELETE destroys it and every
  generation's values once retirement and cleanup are confirmed.
- On PUT, omitting the map keeps the values. Dropping names removes those
  values from the new generation, and adding a name requires a complete map.
- A supplied map always replaces every value (rotation), even when the
  values are equal. A partial map is invalid.
- Values belong to a generation:
  - Rotation switches routes only after the new generation passes health.
  - On failure, the old generation and its values keep serving.
  - A generation's values are kept until its physical retirement is
    confirmed, and the configured map is kept until a later accepted
    generation replaces it.
- `waiting_input` applies only to values sent in the Operation's own request
  for a provably unsent step. A PUT without a map never waits for input.
- If the Host loses retained values, it never starts with missing or empty
  values. Restart reports `ready:false`. An Operation that cannot restore the
  values it needs before the route switch, when `waiting_input` does not
  apply, fails with the new code `secret_unavailable` (effect `none` or
  `partial`). A loss after the switch never fails the Operation on its own:
  a completed Operation keeps its result, and an unfinished one still waits
  for the old generation's drain and physical retirement, because the switch
  alone is not cleanup. The loss shows as `ready:false` once a restart is
  needed. An unknown effect stays `reconciling`. A PUT with the full map
  repairs the Resource.
- Secret-free use is complete use, so a Host may support the Form with
  `privateInputs:false`.

**C. Images.**

- A pin may name a single manifest (OCI or Docker schema 2) or an index
  (OCI or Docker manifest list) containing exactly one linux/amd64
  descriptor. `platform.variant` is not used: an index with several amd64
  variants is refused, and the client pins the manifest it wants. Nested
  indexes and a config whose `os`/`architecture` disagree are refused.
- Every descriptor in the index, including unselected and attestation
  entries, and the executed manifest's config and layers must use canonical
  lowercase `sha256:`. Index and manifest documents are at most 4194304
  bytes, and blob lengths must equal their descriptor sizes.
- `spec` keeps the source digest. `observed.imageDigest` reports the
  executable digest.
- The Host records the selection per generation before effects and never
  reselects it.
- Selected image bytes count the executed manifest's config and layers and
  are checked before layers are transferred. The Host publishes the limit,
  and exceeding it gives the new code `image_too_large`.
- These are Operation failures after acceptance, not pre-acceptance 422s,
  because they are known only after fetching. The text adds a table that
  gives the condition for every Form error code.
- The source digest of an older serving generation is not readable from the
  current spec; clients keep their own record.
- Registry credentials are Host-owned.

**D. Egress.** `outboundInternet:true` is permission, not reachability.

- Under `false`, the execution starts no outbound connection, including DNS
  queries to the Host resolver, which could otherwise carry data out.
- Under `true`, the Host or node loopback, private addresses, link-local and
  metadata destinations (including IPv4-mapped IPv6), private paths to
  management planes, and other tenants' executions and their private paths
  are refused on every protocol and port. HTTP(S)-only filtering does not
  satisfy `true`.
- Loopback traffic that ends inside the execution's own network namespace
  is not egress. Forwarding through an in-namespace address to the Host
  resolver or services (embedded DNS, DNAT) is not exempt.
- Public endpoints, including other tenants' public services, stay ordinary
  destinations. Under `true`, only DNS queries to the Host's designated
  resolver are exempt, not other traffic to that address.
- Under both values, the refusal applies to each connection's effective
  destination, so IP literals, other notations such as IPv4-mapped IPv6,
  changed or re-resolved DNS answers, redirects, proxies and route
  translation cannot reach a refused destination.
- A Host may set abuse controls in its published policy: refusing specific
  ports such as TCP 25, connection and byte limits, metering and a kill
  switch. These stay in Offering or Host interfaces.
- Abuse controls do not replace general public reachability with a few
  protocols or ports. An HTTP(S)-only relay that refuses everything else
  does not satisfy `true`. This follows the full-scope request on PR #17,
  which says a disabled-Internet HTTP(S) broker is a narrower supply path
  that does not qualify the full contract.
- The Host may not require images to change their entrypoint, or to trust a
  Host certificate authority, to pass policy. This follows the same request.
- A Host that cannot enforce this does not return `supported:true` for this
  URL. There is no per-value egress support or capability, and an
  implementation that satisfies only `false` is not support. If the
  publisher wants an HTTP-only profile, it must define it as a separate
  capability first.

**E. HTTP.** A response head does not end the upload; `duplex-stream` is
unchanged.

- When the Host observes the response body's end, or delivers a bodyless
  response, it drops the unsent bytes and cancels the caller's request
  stream. While the caller has not read the response, the Host may not have
  observed its end yet and may keep uploading.
- Cancelling the response body and aborting the caller's signal also cancel
  an unfinished upload.
- The completion-triggered cancel is not an error and keeps the delivered
  response, but it never hides an upload failure the Host observed earlier.
- Cancelling the request stream is not evidence that the native upload
  ended. The Host does not count the call complete for drain or retirement
  until it confirms that end. A transport signal that the
backend stopped reading is not used: HTTP/1.1 has none, HTTP/2 offers it only
after a complete response, and a Workers `fetch` cannot see it.
`binding-cases.json` adds `early-complete-response`,
`early-bodyless-response`, `early-response-nonsettling-upload` and
`upload-error-before-completion`. The caller's own ReadableStream `cancel`
callback is the only accepted evidence of the source cancel. Positive cases
also require the backend's record that its native upload read ended, and a
following ordinary call.

Container 0.1.0's own text (its lines 123–125) still conflicts with its
`duplex-stream` case. Qualifying 0.1.0 needs the publisher to say whether its
"早期response" means a completed response, as this proposal reads it.

## How the caller successors are checked

WorkerVersion 0.8.0 and WorkerDeployment 0.7.0 are copies of their frozen
predecessors with recorded changes only. `successor-deltas.json` lists those
changes, and the node test applies them in both directions against the frozen
bytes. `callers.mjs` and `caller-cases.json` state which Container URL each
WorkerVersion may bind (WorkerVersion 0.6.0 and 0.7.0 cannot bind 0.2.0) and
what WorkerDeployment 0.7.0 accepts and fences, and the test reads the same
lists back from the texts.

## Source review record and remaining qualification

The publisher adopted the three exact URLs and the source values below. The
0.1.0 interpretation remains a separate qualification issue; Host behavior
and public availability require their own evidence.

- the Container 0.2.0, WorkerVersion 0.8.0 and WorkerDeployment 0.7.0 URLs
- the limits 64 names, 16384 scalar values, 65536 bytes and 4194304-byte
  image documents
- the new error codes `secret_unavailable` and `image_too_large`
- index pins are eligible; multi-variant amd64 indexes are refused
- the reading of 0.1.0's early-response rule for 0.1.0 qualification
- that removal is expressed only through `requiredSensitiveVars`
- that port policy is an abuse control, so an HTTP(S)-only relay does not
  satisfy `true` and images are not asked to trust a Host certificate
  authority; an HTTP-only profile would need its own capability
- that `false` also refuses DNS queries to the Host resolver
- that a new WorkerDeployment 0.4.0, 0.5.0 or 0.6.0 is refused while a
  WorkerDeployment 0.7.0 is active for the same Worker. The frozen 0.6.0 text
  names only 0.4.0 and 0.5.0 in its fence sentence, so this direction rests on
  its general rule that a Worker has at most one active Deployment and that a
  different Form is no reason to activate two

## Not in this change

- runner support for `binding-cases.json` in `binding-suite.mjs`
- Host implementation, publication, support and staging qualification
