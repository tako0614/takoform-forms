# Container v2 conformance candidate

Normative source: Container/0.1.0 (including container.http and
module-worker.container), WorkerVersion/0.6.0, WorkerDeployment/0.5.0 under
spec/forms. These are unpublished candidates; no native Host is qualified here.

`contract.mjs` checks parsed spec/observation values, request heads and resolved
UID references. It is an optional authoring aid, not a common API parser,
authorization service, runtime implementation or overriding specification.
The Host still validates duplicate JSON keys and raw Unicode, replay intent,
generation fences and durable acceptance under Host API v2.

`binding-suite.mjs` exports runContainerBindingSuite(adapter). The adapter's
async open(caseName) creates a disposable backend fixture and returns:

- binding: the actual Worker env.APP capability to test.
- targetUid: the bound Container UID.
- probe(): independent backend instrumentation with calls, targetUid, path and
  headers. Count application attempts, excluding health probes.
- waitFor(event): resolves on requestStarted/requestCancelled/responseCancelled.
- releaseBody(): releases withheld response bytes (or injects a reset).
- close(): cancels remaining work and confirms disposable fixture cleanup.

Scenario names and exact application response shapes are in BINDING_CASES and
exercise. `duplex-stream` must echo the first upload chunk before waiting for
the remainder; `response-head` withholds bytes; `response-reset` sends a prefix
then resets; `uncertain-no-retry` loses its head after accepting the request.
Invalid input fixtures record zero dispatches. Invoke the suite inside the
actual Worker execution context, across the real broker/native transport.
Instrumented test doubles self-test this runner locally and establish only
the runner's acceptance/refusal behavior.

`host-cases.json` defines the additional management/native qualifications.
Run the same cases on hosted Cloudflare Containers, self-host Docker, and
Linux/systemd with Docker Engine. Capture exact commit, Form URLs, adapter,
Offering, image digest, Resource/Operation IDs, generations, timestamps and
backend-owned readback. Never put private credentials into a report. Physical
Host process termination and restart is required for restart cases; recreating
handles is insufficient. Catalog quota is Cloudflare-specific, while admission
must preserve the common contract everywhere. An unsupported required case
is a failed qualification, not a skip or an assumed supported Form.

```console
bun test scripts/container-v2.test.mjs
bun run check
```

This corpus does not publish, activate or declare support. The initial
profile refuses secret injection. Existing Host adapters also refuse enabled
Internet egress and lack the Worker Binding path, so real backend qualification
and Takoserver agreement remain pending.
