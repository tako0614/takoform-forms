import { BODY_LIMIT } from "./contract.mjs";

// Plain-JavaScript assertions let this suite run under the closed Worker ABI.
// Failure text deliberately does not serialize backend/caller payloads.
function ensure(condition, message = "Container conformance assertion failed") {
  if (!condition) throw new Error(message);
}

const assert = {
  equal: (actual, expected) => ensure(Object.is(actual, expected)),
  notEqual: (actual, expected) => ensure(!Object.is(actual, expected)),
  ok: ensure,
  async rejects(promise, matches) {
    try {
      await promise;
    } catch (error) {
      ensure(matches(error));
      return;
    }
    throw new Error("Expected Container error was not raised");
  },
};

export const BINDING_CASES = [
  "logical-route",
  "application-error",
  "redirect",
  "response-head",
  "duplex-stream",
  "response-cancel",
  "request-cancel",
  "unavailable",
  "invalid-method",
  "invalid-framing",
  "request-limit",
  "response-limit",
  "response-reset",
  "uncertain-no-retry",
  "header-authority",
  "request-length-mismatch",
  "request-overflow",
  "response-length-mismatch",
  "response-overflow",
];

async function errorCode(promise, code) {
  await assert.rejects(
    promise,
    (error) => error.name === "ContainerError" && error.code === code,
  );
}

function bounded(promise, timeoutMs) {
  let timer;
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      timer = setTimeout(
        () => reject(new Error("conformance fixture timed out")),
        timeoutMs,
      );
    }),
  ]).finally(() => clearTimeout(timer));
}

// open(name) prepares a disposable fixture and returns {binding,targetUid,
// probe(),waitFor(event),releaseBody(),close()}. probe is backend instrumentation,
// not an application response. Native adapters must run inside the real Worker.
export async function runContainerBindingSuite(
  adapter,
  { timeoutMs = 10000 } = {},
) {
  const passed = [];
  for (const name of BINDING_CASES) {
    const fixture = await bounded(adapter.open(name), timeoutMs);
    try {
      await bounded(exercise(name, fixture), timeoutMs);
      passed.push(name);
    } finally {
      await bounded(fixture.close(), timeoutMs);
    }
  }
  return { scope: "container-worker-binding-fixtures", passed };
}

async function exercise(name, fixture) {
  const call = (request) =>
    Promise.resolve().then(() => fixture.binding.fetch(request));
  const request = (init = {}, path = "/echo?key=a%2Fb") =>
    new Request(`https://unrelated.invalid${path}`, init);
  if (name === "logical-route") {
    const response = await call(request({ method: "POST", body: "hello" }));
    assert.equal(await response.text(), "hello");
    const probe = await fixture.probe();
    assert.equal(probe.calls, 1);
    assert.equal(probe.targetUid, fixture.targetUid);
    assert.equal(probe.path, "/echo?key=a%2Fb");
  } else if (name === "application-error") {
    const response = await call(request());
    assert.equal(response.status, 503);
    assert.equal(await response.text(), "application failure");
  } else if (name === "redirect") {
    const response = await call(request());
    assert.equal(response.status, 302);
    assert.equal(
      response.headers.get("location"),
      "https://different.invalid/next",
    );
    await response.body?.cancel();
    assert.equal((await fixture.probe()).calls, 1);
  } else if (name === "response-head") {
    const response = await call(request());
    // Fixture withholds all body bytes until releaseBody. Whole-body buffering
    // cannot resolve this fetch and fails the suite's bound.
    assert.equal(response.status, 200);
    await fixture.releaseBody();
    assert.equal(await response.text(), "later");
  } else if (name === "duplex-stream") {
    let upload,
      pulls = 0;
    const body = new ReadableStream(
      {
        start(controller) {
          upload = controller;
          controller.enqueue(new TextEncoder().encode("one"));
        },
        pull() {
          pulls++;
        },
      },
      { highWaterMark: 0 },
    );
    const response = await call(
      request({ method: "POST", body, duplex: "half" }),
    );
    const reader = response.body.getReader();
    assert.equal(new TextDecoder().decode((await reader.read()).value), "one");
    assert.ok(pulls <= 2, "upload must honor backpressure");
    upload.enqueue(new TextEncoder().encode("two"));
    upload.close();
    assert.equal(new TextDecoder().decode((await reader.read()).value), "two");
    assert.equal((await reader.read()).done, true);
  } else if (name === "response-cancel") {
    const response = await call(request());
    await response.body.cancel();
    await fixture.waitFor("responseCancelled");
  } else if (name === "request-cancel") {
    const controller = new AbortController();
    const result = call(request({ signal: controller.signal }));
    await fixture.waitFor("requestStarted");
    controller.abort();
    await errorCode(result, "request_aborted");
    await fixture.waitFor("requestCancelled");
    assert.equal((await fixture.probe()).calls, 1);
  } else if (name === "unavailable") {
    await errorCode(call(request()), "backend_unavailable");
    assert.equal((await fixture.probe()).calls, 0);
  } else if (name === "invalid-method" || name === "invalid-framing") {
    const init =
      name === "invalid-method"
        ? { method: "PROPFIND" }
        : { method: "POST", body: "x", headers: { "Content-Length": "1, 1" } };
    await errorCode(call(request(init)), "invalid_request");
    assert.equal((await fixture.probe()).calls, 0);
  } else if (name === "request-limit") {
    await errorCode(
      call(
        request({
          method: "POST",
          headers: { "Content-Length": String(BODY_LIMIT + 1) },
        }),
      ),
      "request_too_large",
    );
    assert.equal((await fixture.probe()).calls, 0);
  } else if (name === "response-limit") {
    await errorCode(call(request()), "response_too_large");
  } else if (name === "response-reset") {
    const response = await call(request());
    assert.equal(response.status, 200);
    const reader = response.body.getReader();
    assert.equal(
      new TextDecoder().decode((await reader.read()).value),
      "prefix",
    );
    await fixture.releaseBody();
    await errorCode(reader.read(), "response_aborted");
  } else if (name === "uncertain-no-retry") {
    await errorCode(
      call(request({ method: "POST", body: "effect" })),
      "backend_unavailable",
    );
    assert.equal((await fixture.probe()).calls, 1);
  } else if (name === "request-length-mismatch") {
    await errorCode(
      call(
        request({
          method: "POST",
          body: "abc",
          headers: { "Content-Length": "4" },
        }),
      ),
      "request_aborted",
    );
    assert.equal((await fixture.probe()).calls, 1);
  } else if (name === "request-overflow") {
    const chunk = new Uint8Array(1048576);
    let sent = 0;
    const body = new ReadableStream(
      {
        pull(controller) {
          controller.enqueue(chunk);
          sent += chunk.length;
          if (sent > BODY_LIMIT) controller.close();
        },
      },
      { highWaterMark: 0 },
    );
    await errorCode(
      call(request({ method: "POST", body, duplex: "half" })),
      "request_too_large",
    );
    await fixture.waitFor("requestCancelled");
    assert.equal((await fixture.probe()).calls, 1);
  } else if (name === "response-length-mismatch") {
    const response = await call(request());
    assert.equal(response.status, 200);
    await errorCode(response.text(), "response_aborted");
  } else if (name === "response-overflow") {
    const response = await call(request());
    assert.equal(response.status, 200);
    const reader = response.body.getReader();
    let received = 0;
    await errorCode(
      (async () => {
        for (;;) {
          const chunk = await reader.read();
          if (chunk.done)
            throw new Error("overflow became a successful response");
          received += chunk.value.byteLength;
          assert.ok(received <= BODY_LIMIT, "must not expose oversized bytes");
        }
      })(),
      "response_too_large",
    );
  } else if (name === "header-authority") {
    const response = await call(
      request({
        headers: {
          Host: "attacker.invalid",
          Connection: "x-remove",
          "x-remove": "private",
          "Proxy-Authorization": "proxy-only",
          Authorization: "application-only",
        },
      }),
    );
    await response.body?.cancel();
    const probe = await fixture.probe();
    assert.equal(probe.targetUid, fixture.targetUid);
    assert.notEqual(probe.headers.host, "attacker.invalid");
    assert.equal(probe.headers["x-remove"], undefined);
    assert.equal(probe.headers["proxy-authorization"], undefined);
    assert.equal(probe.headers.authorization, "application-only");
  }
}
