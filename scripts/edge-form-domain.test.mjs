import { describe, expect, test } from "bun:test";
import { bindEdgeDomain, parseDomainInvocation } from "./edge-form-domain.mjs";

const account = "a".repeat(32),
  commit = "b".repeat(40);
const options = {
  environment: "production",
  "account-id": account,
  commit,
  execute: true,
};
function fixture({
  occupied = false,
  conflict = false,
  putError = false,
  stale = false,
  alreadyBound = false,
} = {}) {
  const calls = [];
  let written = false,
    sourceChecks = 0;
  const record = {
    id: "new-binding",
    hostname: "edge.forms.takoform.com",
    service: occupied ? "other-worker" : "takoform-edge-form-pages",
    environment: "production",
    zone_id: "zone-id",
  };
  const api = async (method, route, body) => {
    calls.push({ method, route, body });
    if (route.startsWith("/zones?"))
      return [
        {
          id: "zone-id",
          name: "takoform.com",
          status: "active",
          account: { id: account },
        },
      ];
    if (method === "GET" && route.includes("/domains/records?"))
      return occupied || written || alreadyBound ? [record] : [];
    if (route.endsWith("/settings")) return {};
    if (route.endsWith("/deployments"))
      return {
        deployments: [
          { versions: [{ version_id: "version-id", percentage: 100 }] },
        ],
      };
    if (route.endsWith("/versions/version-id"))
      return {
        metadata: {
          annotations: { "workers/tag": stale ? "c".repeat(40) : commit },
        },
      };
    if (method === "POST")
      return {
        added: [{ hostname: record.hostname }],
        updated: [],
        removed: [],
        conflicting: conflict ? [{}] : [],
      };
    if (method === "PUT") {
      written = true;
      if (putError) throw new Error("lost acknowledgement");
      return {};
    }
    throw new Error(`unexpected ${method} ${route}`);
  };
  return {
    calls,
    deps: {
      api,
      assertSource: (source) => {
        expect(source).toBe(commit);
        sourceChecks++;
      },
    },
    get sourceChecks() {
      return sourceChecks;
    },
  };
}
describe("isolated Edge hostname authority", () => {
  test("requires exact environment, account and source", () => {
    expect(() => parseDomainInvocation([])).toThrow();
    expect(() =>
      parseDomainInvocation([
        "--environment",
        "integration",
        "--account-id",
        account,
        "--commit",
        commit,
      ]),
    ).toThrow();
    expect(
      parseDomainInvocation([
        "--environment",
        "production",
        "--account-id",
        account,
        "--commit",
        commit,
      ]),
    ).toEqual({ ...options, execute: false });
  });
  test("dry-run reads a changeset without creating a binding", async () => {
    const f = fixture();
    expect(
      (await bindEdgeDomain({ ...options, execute: false }, f.deps)).status,
    ).toBe("READY");
    expect(f.calls.some((call) => call.method === "PUT")).toBe(false);
  });
  test("refuses occupied names, DNS conflicts and stale code", async () => {
    for (const config of [
      { occupied: true },
      { conflict: true },
      { stale: true },
    ]) {
      const f = fixture(config);
      await expect(bindEdgeDomain(options, f.deps)).rejects.toThrow();
      expect(f.calls.some((call) => call.method === "PUT")).toBe(false);
    }
  });
  test("performs one conflict-refusing PUT and reads exact ownership back", async () => {
    const f = fixture();
    const result = await bindEdgeDomain(options, f.deps);
    expect(result.status).toBe("DOMAIN_BOUND");
    expect(f.sourceChecks).toBe(2);
    const puts = f.calls.filter((call) => call.method === "PUT");
    expect(puts).toHaveLength(1);
    expect(puts[0].body).toEqual({
      origins: [{ hostname: "edge.forms.takoform.com", zone_id: "zone-id" }],
      override_scope: false,
      override_existing_origin: false,
      override_existing_dns_record: false,
    });
  });
  test("existing bindings still require the selected Worker source", async () => {
    const stale = fixture({ alreadyBound: true, stale: true });
    await expect(bindEdgeDomain(options, stale.deps)).rejects.toThrow(
      "selected source commit",
    );
    const exact = fixture({ alreadyBound: true });
    expect((await bindEdgeDomain(options, exact.deps)).status).toBe(
      "ALREADY_BOUND",
    );
    expect(exact.calls.some((call) => call.method !== "GET")).toBe(false);
  });
  test("settles a lost acknowledgement through readback, never another write", async () => {
    const f = fixture({ putError: true });
    const result = await bindEdgeDomain(options, f.deps);
    expect(result.status).toBe("DOMAIN_BOUND");
    expect(result.settledAfterError).toBe(true);
    expect(f.calls.filter((call) => call.method === "PUT")).toHaveLength(1);
  });
});
