import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import path from "node:path";
import {
  BINDING_NAME,
  CONTAINER_URL,
  WORKER_VERSION_URL,
  normalizeContainerSpec,
  validateContainerBindings,
  validateContainerObserved,
  validateContainerRequest,
} from "../conformance/host-v2/container/contract.mjs";
import {
  BINDING_CASES,
  runContainerBindingSuite,
} from "../conformance/host-v2/container/binding-suite.mjs";
import { loadEdgeV2Docs } from "./edge-v2-docs.mjs";
import { verifyEdgeFormFreeze } from "./edge-form-freeze.mjs";

const root = path.resolve(import.meta.dirname, "..");
const corpus = path.join(root, "conformance/host-v2/container");
const fixtures = JSON.parse(
  readFileSync(path.join(corpus, "spec-cases.json"), "utf8"),
);
const minimal = fixtures[0].spec;

describe("Container v2 desired-state corpus", () => {
  for (const fixture of fixtures) {
    test(fixture.name, () => {
      const check = () =>
        normalizeContainerSpec(fixture.spec, fixture.privateInputs);
      if (fixture.valid) expect(check).not.toThrow();
      else expect(check).toThrow("invalid_spec");
    });
  }
  test("omitted defaults normalize without changing original input", () => {
    expect(normalizeContainerSpec(minimal)).toEqual(
      normalizeContainerSpec(fixtures[1].spec),
    );
    expect(minimal).not.toHaveProperty("outboundInternet");
  });
  test("workload revision remains part of exact desired intent", () => {
    expect(normalizeContainerSpec(minimal)).not.toEqual(
      normalizeContainerSpec({ ...minimal, workloadRevision: "revision-2" }),
    );
  });
  test("rejects ill-formed Unicode and counts astral characters as scalars", () => {
    expect(() =>
      normalizeContainerSpec({ ...minimal, environment: { APP: "\ud800" } }),
    ).toThrow();
    expect(() =>
      normalizeContainerSpec({
        ...minimal,
        environment: { APP: "😀".repeat(4096) },
      }),
    ).not.toThrow();
    expect(() =>
      normalizeContainerSpec({
        ...minimal,
        environment: { APP: "😀".repeat(4097) },
      }),
    ).toThrow();
  });
});

describe("Container UID and observation boundaries", () => {
  const source = { form: WORKER_VERSION_URL, host: "host", space: "space" };
  const target = {
    uid: "container-uid",
    form: CONTAINER_URL,
    host: "host",
    space: "space",
    authorized: true,
  };
  const bindings = [{ name: "APP", resource: { resourceUid: target.uid } }];
  const check = (entries = bindings, overrides = {}, options = {}) =>
    validateContainerBindings(entries, {
      source,
      resolve: () => ({ ...target, ...overrides }),
      ...options,
    });
  test("accepts only the exact UID/Form/Host/Space with explicit authorization", () => {
    expect(() => check()).not.toThrow();
    for (const change of [
      { uid: "recreated-uid" },
      { form: "https://edge.forms.takoform.com/forms/ContainerService/0.1.0/" },
      { form: CONTAINER_URL + "?" },
      { host: "foreign" },
      { space: "foreign" },
      { authorized: false },
      { authorized: undefined },
      { deleting: true },
    ])
      expect(() => check(bindings, change)).toThrow("dependency_conflict");
    expect(() => check(bindings, {}, { resolve: () => undefined })).toThrow();
    expect(() =>
      check(
        bindings,
        {},
        {
          source: {
            ...source,
            form: WORKER_VERSION_URL.replace("0.6.0", "0.5.0"),
          },
        },
      ),
    ).toThrow();
  });
  test("closed binding shape and one env namespace", () => {
    expect(() => check([...bindings, ...bindings])).toThrow();
    expect(() => check(bindings, {}, { occupiedNames: ["APP"] })).toThrow();
    for (const entry of [
      {
        name: "APP",
        resource: { resourceUid: "container-uid", name: "ignored" },
      },
      { name: "APP", resource: { resourceUid: "" } },
      { name: "bad-name", resource: { resourceUid: "container-uid" } },
      {
        name: "APP",
        resource: { resourceUid: "container-uid" },
        url: "http://native",
      },
    ])
      expect(() => check([entry])).toThrow();
  });
  test("older serving generation may remain healthy after failed update", () => {
    expect(() =>
      validateContainerObserved(
        {
          ready: true,
          servingGeneration: 2,
          imageDigest: "sha256:" + "a".repeat(64),
        },
        3,
      ),
    ).not.toThrow();
    expect(() => validateContainerObserved({}, 1)).not.toThrow();
    expect(() => validateContainerObserved({ ready: false }, 1)).not.toThrow();
    for (const observed of [
      { ready: true },
      { ready: false, servingGeneration: 1 },
      { ready: false, imageDigest: "sha256:" + "a".repeat(64) },
      {
        ready: true,
        servingGeneration: "1",
        imageDigest: "sha256:" + "a".repeat(64),
      },
      {
        ready: true,
        servingGeneration: 4,
        imageDigest: "sha256:" + "a".repeat(64),
      },
      { ready: false, nativeIp: "127.0.0.1" },
    ])
      expect(() => validateContainerObserved(observed, 3)).toThrow();
  });
});

describe("Container HTTP head checks", () => {
  test("validates unknown-length stream without reading it", () => {
    let pulls = 0;
    const body = new ReadableStream(
      {
        pull() {
          pulls++;
        },
      },
      { highWaterMark: 0 },
    );
    const request = new Request("https://ignored.invalid/echo?x=a%2Fb", {
      method: "POST",
      body,
      duplex: "half",
    });
    expect(validateContainerRequest(request)).toEqual({
      method: "POST",
      path: "/echo?x=a%2Fb",
      contentLength: null,
    });
    expect(pulls).toBe(0);
  });
  test("canonical framing, limits, upgrade and cancellation refuse before dispatch", () => {
    for (const length of ["00", "-1", "1, 1", "9007199254740992"]) {
      expect(() =>
        validateContainerRequest(
          new Request("https://ignored.invalid/", {
            headers: { "Content-Length": length },
          }),
        ),
      ).toThrow("invalid_request");
    }
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/", {
          headers: { Upgrade: "websocket" },
        }),
      ),
    ).toThrow("invalid_request");
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/", {
          headers: { Connection: "Content-Length", "Content-Length": "0" },
        }),
      ),
    ).toThrow("invalid_request");
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/", {
          headers: Object.fromEntries(
            Array.from({ length: 65 }, (_, i) => ["x-" + i, "value"]),
          ),
        }),
      ),
    ).toThrow("invalid_request");
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/", {
          headers: { "x-long": "x".repeat(16384) },
        }),
      ),
    ).toThrow("invalid_request");
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/#fragment"),
      ),
    ).toThrow("invalid_request");
    expect(() =>
      validateContainerRequest(
        new Request("https://ignored.invalid/", {
          signal: AbortSignal.abort(),
        }),
      ),
    ).toThrow("request_aborted");
  });
});

describe("reusable Binding suite acceptance and refusal", () => {
  test("exercises all named scenarios with instrumented test fixtures", async () => {
    const report = await runContainerBindingSuite(fixtureAdapter());
    expect(report.passed).toEqual(BINDING_CASES);
    expect(report.scope).toBe("container-worker-binding-fixtures");
  });
  for (const violation of [
    "redirect-follow",
    "retry",
    "native-authority",
    "empty-reset",
    "buffer-body",
  ]) {
    test(`rejects ${violation}`, async () => {
      await expect(
        runContainerBindingSuite(fixtureAdapter(violation), { timeoutMs: 100 }),
      ).rejects.toThrow();
    });
  }
});

describe("candidate source and native qualification boundaries", () => {
  test("candidate routes appear in preview and remain outside frozen publication", () => {
    const docs = loadEdgeV2Docs(root);
    for (const route of [
      "/forms/Container/0.1.0/",
      "/forms/WorkerVersion/0.6.0/",
      "/forms/WorkerDeployment/0.5.0/",
    ])
      expect(docs.some((entry) => entry.route === route)).toBe(true);
    const freeze = verifyEdgeFormFreeze(root, {
      mode: "check",
      requireFrozen: true,
    });
    expect(freeze.status).toBe("FROZEN");
    expect(freeze.frozen).toHaveLength(17);
    expect(freeze.pending.map((entry) => entry.url)).toContain(CONTAINER_URL);
  });
  test("native matrix requires all three execution profiles and real process restart", () => {
    const matrix = JSON.parse(
      readFileSync(path.join(corpus, "host-cases.json"), "utf8"),
    );
    expect(matrix.form).toBe(CONTAINER_URL);
    expect(matrix.binding).toBe(BINDING_NAME);
    expect(new Set(matrix.cases.map((entry) => entry.id)).size).toBe(
      matrix.cases.length,
    );
    const restart = matrix.cases.find(
      (entry) => entry.id === "process-restart-unknown",
    );
    expect(restart.requiredProfiles).toEqual(matrix.profiles);
    expect(restart.assertions.join(" ")).toContain("real Host OS process");
    expect(
      matrix.cases.find((entry) => entry.id === "cloudflare-image-capacity")
        .requiredProfiles,
    ).toEqual(["cloudflare-containers"]);
  });
});

// A self-test fixture for the runner, intentionally not a Host/backend adapter.
function fixtureAdapter(violation) {
  return {
    async open(scenario) {
      const targetUid = "fixture-container-uid";
      let calls = 0,
        captured,
        controller;
      const events = new Map();
      const signal = (name) => {
        const event = events.get(name) ?? {};
        event.done = true;
        event.resolve?.();
        events.set(name, event);
      };
      const waitFor = (name) => {
        const event = events.get(name);
        if (event?.done) return Promise.resolve();
        return new Promise((resolve) => events.set(name, { resolve }));
      };
      const fault = (code) =>
        Object.assign(new Error(code), { name: "ContainerError", code });
      return {
        targetUid,
        waitFor,
        probe: async () => ({
          calls,
          targetUid,
          path: captured?.path,
          headers: captured?.headers,
        }),
        close: async () => {},
        releaseBody: async () => {
          if (scenario === "response-reset")
            controller.error(fault("response_aborted"));
          else {
            controller.enqueue(new TextEncoder().encode("later"));
            controller.close();
          }
        },
        binding: {
          async fetch(request) {
            let head;
            try {
              head = validateContainerRequest(request);
            } catch (error) {
              throw fault(error.code);
            }
            if (scenario === "unavailable") throw fault("backend_unavailable");
            calls++;
            const headers = Object.fromEntries(request.headers);
            if (violation !== "native-authority") {
              for (const name of [
                "host",
                "connection",
                "x-remove",
                "proxy-authorization",
              ])
                delete headers[name];
            }
            captured = { path: head.path, headers };
            if (scenario === "application-error")
              return new Response("application failure", { status: 503 });
            if (scenario === "redirect")
              return violation === "redirect-follow"
                ? new Response("followed")
                : new Response(null, {
                    status: 302,
                    headers: { location: "https://different.invalid/next" },
                  });
            if (scenario === "duplex-stream") return new Response(request.body);
            if (scenario === "request-cancel") {
              return new Promise((_, reject) => {
                request.signal.addEventListener(
                  "abort",
                  () => {
                    signal("requestCancelled");
                    reject(fault("request_aborted"));
                  },
                  { once: true },
                );
                signal("requestStarted");
              });
            }
            if (scenario === "response-limit")
              throw fault("response_too_large");
            if (scenario === "request-length-mismatch") {
              await request.text();
              throw fault("request_aborted");
            }
            if (scenario === "request-overflow") {
              const reader = request.body.getReader();
              let received = 0;
              for (;;) {
                const item = await reader.read();
                if (item.done) break;
                received += item.value.byteLength;
                if (received > 104857600) {
                  await reader.cancel();
                  signal("requestCancelled");
                  throw fault("request_too_large");
                }
              }
              throw new Error("fixture failed to exceed request limit");
            }
            if (
              scenario === "response-length-mismatch" ||
              scenario === "response-overflow"
            ) {
              let received = 0;
              const chunk = new Uint8Array(
                scenario === "response-overflow" ? 1048576 : 3,
              );
              return new Response(
                new ReadableStream({
                  pull(stream) {
                    if (
                      (scenario === "response-overflow" &&
                        received >= 104857600) ||
                      (scenario === "response-length-mismatch" && received > 0)
                    ) {
                      stream.error(
                        fault(
                          scenario === "response-overflow"
                            ? "response_too_large"
                            : "response_aborted",
                        ),
                      );
                    } else {
                      stream.enqueue(chunk);
                      received += chunk.length;
                    }
                  },
                }),
                {
                  headers:
                    scenario === "response-length-mismatch"
                      ? { "Content-Length": "4" }
                      : {},
                },
              );
            }
            if (scenario === "uncertain-no-retry") {
              if (violation === "retry") calls++;
              throw fault("backend_unavailable");
            }
            if (
              ["response-head", "response-reset", "response-cancel"].includes(
                scenario,
              )
            ) {
              const body = new ReadableStream({
                start(stream) {
                  controller = stream;
                  if (scenario === "response-reset") {
                    if (violation === "empty-reset") stream.close();
                    else stream.enqueue(new TextEncoder().encode("prefix"));
                  }
                },
                cancel() {
                  signal("responseCancelled");
                },
              });
              if (violation === "buffer-body" && scenario === "response-head")
                return new Response(await new Response(body).text());
              return new Response(body);
            }
            return new Response(request.body ? await request.text() : "ok");
          },
        },
      };
    },
  };
}
