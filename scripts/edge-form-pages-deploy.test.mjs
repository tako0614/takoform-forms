import { describe, expect, test } from "bun:test";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import {
  DEPLOY_CONTRACT,
  assertEdgePageHeaders,
  readEdgeFormPages,
  parseDeployInvocation as parseOwnerInvocation,
  runDeploy as runOwnerDeploy,
} from "./deploy.mjs";
import { EDGE_FORM_PAGES_SURFACE } from "./edge-form-pages.mjs";

const SET_ID = "e7f8a39311dd011b8467e97e7f300cabb9a6b06c";
const COMMIT = "0123456789abcdef0123456789abcdef01234567";
const target = (args) => [
  ...args,
  "--environment",
  "production",
  "--commit",
  COMMIT,
];
const parseDeployInvocation = (args) => parseOwnerInvocation(target(args));
const runDeploy = (args, dependencies) =>
  runOwnerDeploy(target(args), dependencies);

describe("Edge Form human page deploy surface", () => {
  test("root readback rejects matching HTML with missing CSP after an upload", () => {
    const temporary = mkdtempSync(path.join(tmpdir(), "edge-header-readback-"));
    const assetsDirectory = path.join(temporary, "assets");
    mkdirSync(assetsDirectory);
    writeFileSync(path.join(assetsDirectory, "index.html"), "matching HTML");
    writeFileSync(
      path.join(assetsDirectory, "_headers"),
      "/*\n  Content-Security-Policy: default-src 'none'\n",
    );
    let reads = 0;
    try {
      let failure;
      try {
        readEdgeFormPages(
          {
            stderr() {},
            runReadOnly(command, args) {
              expect(command).toBe("curl");
              expect(args).toContain("--dump-header");
              writeFileSync(
                args[args.indexOf("--output") + 1],
                "matching HTML",
              );
              writeFileSync(
                args[args.indexOf("--dump-header") + 1],
                "HTTP/2 200\r\ncache-control: no-transform\r\nx-content-type-options: nosniff\r\n\r\n",
              );
              reads++;
              return { exitCode: 0, stdout: "200", stderr: "" };
            },
          },
          { routes: ["/"], files: [] },
          assetsDirectory,
          true,
        );
      } catch (error) {
        failure = error;
      }
      expect(failure?.message).toContain("deployed response headers differ");
      expect(failure?.mutationStarted).toBe(true);
      expect(reads).toBe(1);
    } finally {
      rmSync(temporary, { recursive: true, force: true });
    }
  });
  test("requires deployed script policy and response protections, not just matching HTML", () => {
    const expected =
      "/*\n  Content-Security-Policy: default-src 'none'; script-src 'self' 'sha256-test'; style-src 'self' 'unsafe-inline'\n";
    const valid =
      "HTTP/1.1 200 Connection established\r\n\r\nHTTP/2 200\r\ncontent-security-policy: style-src 'unsafe-inline' 'self'; script-src 'sha256-test' 'self'; default-src 'none'\r\nx-content-type-options: nosniff\r\ncache-control: public, no-transform\r\n\r\n";
    expect(() => assertEdgePageHeaders(valid, expected)).not.toThrow();
    for (const invalid of [
      valid.replace(/content-security-policy:[^\r]+\r\n/u, ""),
      valid.replace("'sha256-test'", "'unsafe-inline'"),
      valid.replace("nosniff", "none"),
      valid.replace("no-transform", "max-age=0"),
    ])
      expect(() => assertEdgePageHeaders(invalid, expected)).toThrow();
    const pages = DEPLOY_CONTRACT.surfaces.filter((surface) =>
      ["edge-form-pages-bootstrap", "edge-form-pages"].includes(
        surface.surface,
      ),
    );
    expect(pages[0].covers).toEqual(pages[1].covers);
    expect(new Set(pages[0].covers).size).toBe(pages[0].covers.length);
    expect(pages[0].covers).toContain("scripts/edge-form-pages-vitepress.mjs");
  });
  test("requires an explicit production environment and exact source", () => {
    expect(() =>
      parseOwnerInvocation([EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID]),
    ).toThrow();
    expect(() =>
      parseOwnerInvocation([
        EDGE_FORM_PAGES_SURFACE,
        "--trust-set",
        SET_ID,
        "--environment",
        "integration",
        "--commit",
        COMMIT,
      ]),
    ).toThrow();
  });
  test("stops a source change after the gate before upload", () => {
    const f = pageDependencies();
    let calls = 0;
    f.dependencies.requireEdgeFormPageSourceIdentity = () =>
      ++calls === 1 ? COMMIT : "f".repeat(40);
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID],
        f.dependencies,
      ),
    ).toBe(1);
    expect(f.calls.some((call) => call.command.endsWith("wrangler"))).toBe(
      false,
    );
  });
  test("bootstrap only creates an absent Worker and does not claim a public site", () => {
    const f = pageDependencies();
    let reads = 0;
    f.dependencies.readEdgeHistory = () =>
      ++reads <= 2
        ? { absent: true, deployments: [] }
        : {
            absent: false,
            deployments: [
              {
                versions: [
                  {
                    version_id: "11111111-1111-1111-1111-111111111111",
                    percentage: 100,
                  },
                ],
              },
            ],
          };
    expect(
      runDeploy(
        ["edge-form-pages-bootstrap", "--trust-set", SET_ID],
        f.dependencies,
      ),
    ).toBe(0);
    expect(JSON.parse(f.stdout).status).toBe("UPLOADED_AWAITING_DOMAIN");
    expect(f.readbacks).toBe(0);
    const existing = pageDependencies();
    expect(
      runDeploy(
        ["edge-form-pages-bootstrap", "--trust-set", SET_ID],
        existing.dependencies,
      ),
    ).toBe(1);
    expect(
      existing.calls.some((call) => call.command.endsWith("wrangler")),
    ).toBe(false);
  });
  test("refuses routine upload without a provider rollback identity", () => {
    const f = pageDependencies();
    f.dependencies.readEdgeHistory = () => ({ absent: false, deployments: [] });
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID],
        f.dependencies,
      ),
    ).toBe(1);
    expect(f.stderr).toContain("predecessor identity");
    expect(
      f.calls.filter((call) => call.command.endsWith("wrangler")),
    ).toHaveLength(0);
  });
  test("requires provider proof of the actual uploaded version", () => {
    const f = pageDependencies();
    const read = f.dependencies.readEdgeHistory;
    let reads = 0;
    f.dependencies.readEdgeHistory = () =>
      ++reads <= 2 ? read() : { absent: false, deployments: [] };
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID],
        f.dependencies,
      ),
    ).toBe(1);
    expect(f.stderr).toContain("indeterminate");
    expect(
      f.calls.filter((call) => call.command.endsWith("wrangler")),
    ).toHaveLength(1);
  });
  test("refuses changed provider state immediately before any upload", () => {
    for (const bootstrap of [true, false]) {
      const f = pageDependencies();
      let reads = 0;
      f.dependencies.readEdgeHistory = () =>
        ++reads === 1
          ? {
              absent: bootstrap,
              deployments: bootstrap
                ? []
                : [
                    {
                      id: "before",
                      versions: [{ version_id: "previous", percentage: 100 }],
                    },
                  ],
            }
          : { absent: false, deployments: [{ id: "concurrent-deploy" }] };
      expect(
        runDeploy(
          [
            bootstrap ? "edge-form-pages-bootstrap" : EDGE_FORM_PAGES_SURFACE,
            "--trust-set",
            SET_ID,
          ],
          f.dependencies,
        ),
      ).toBe(1);
      expect(f.stderr).toContain("provider state changed");
      expect(f.calls.some((call) => call.command.endsWith("wrangler"))).toBe(
        false,
      );
    }
  });
  test("reads history after an uncertain upload without a second write or success claim", () => {
    const f = pageDependencies();
    const run = f.dependencies.run;
    const read = f.dependencies.readEdgeHistory;
    let reads = 0;
    f.dependencies.readEdgeHistory = () => {
      reads++;
      return read();
    };
    f.dependencies.run = (command, args) => {
      const result = run(command, args);
      return command.endsWith("wrangler")
        ? { ...result, exitCode: 1, stderr: "acknowledgement lost" }
        : result;
    };
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID],
        f.dependencies,
      ),
    ).toBe(1);
    expect(reads).toBe(3);
    expect(f.stderr).toContain("Provider history after uncertain upload");
    expect(f.stderr).toContain("indeterminate");
    expect(
      f.calls.filter((call) => call.command.endsWith("wrangler")),
    ).toHaveLength(1);
  });
  test("has an independent contract and exact CLI", () => {
    const surface = DEPLOY_CONTRACT.surfaces.find(
      (entry) => entry.surface === EDGE_FORM_PAGES_SURFACE,
    );
    expect(surface.target).toContain("takoform-edge-form-pages");
    expect(surface.target).toContain("edge.forms.takoform.com");
    expect(surface.covers).toContain("site/wrangler.jsonc");
    expect(surface.requiresScripts).toEqual([
      "check:edge-form-pages",
      "deploy",
    ]);
    expect(surface.triggers).toEqual([]);

    expect(
      parseDeployInvocation([EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID]),
    ).toEqual({
      surface: EDGE_FORM_PAGES_SURFACE,
      mode: "publish",
      trustSet: SET_ID,
      environment: "production",
      commit: COMMIT,
    });
    expect(
      parseDeployInvocation([
        EDGE_FORM_PAGES_SURFACE,
        "--trust-set",
        SET_ID,
        "--dry-run",
      ]),
    ).toEqual({
      surface: EDGE_FORM_PAGES_SURFACE,
      mode: "dry-run",
      trustSet: SET_ID,
      environment: "production",
      commit: COMMIT,
    });
    expect(
      parseDeployInvocation([
        EDGE_FORM_PAGES_SURFACE,
        "--trust-set",
        SET_ID,
        "--verify",
      ]),
    ).toEqual({
      surface: EDGE_FORM_PAGES_SURFACE,
      mode: "verify",
      trustSet: SET_ID,
      environment: "production",
      commit: COMMIT,
    });
  });

  test("dry-run verifies public packages, gates, builds, and only runs Wrangler dry-run", () => {
    const fixture = pageDependencies();
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID, "--dry-run"],
        fixture.dependencies,
      ),
    ).toBe(0);
    expect(fixture.publicChecks).toBe(2);
    expect(fixture.calls).toContainEqual({
      command: "bun",
      args: ["run", "check:edge-form-pages", "--trust-set", SET_ID],
    });
    const wrangler = fixture.calls.find((call) =>
      call.command.endsWith("wrangler"),
    );
    expect(wrangler.args).toContain("--dry-run");
    expect(wrangler.args).toContain("--outdir");
    expect(fixture.readbacks).toBe(0);
    expect(JSON.parse(fixture.stdout).status).toBe("DRY_RUN_VERIFIED");
    expect(
      fixture.calls.some(
        (call) => call.command === "git" && call.args.includes("push"),
      ),
    ).toBe(false);
  });

  test("verify reads every expected route without uploading", () => {
    const fixture = pageDependencies();
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID, "--verify"],
        fixture.dependencies,
      ),
    ).toBe(0);
    expect(fixture.publicChecks).toBe(1);
    expect(fixture.readbacks).toBe(1);
    expect(
      fixture.calls.some((call) => call.command.endsWith("wrangler")),
    ).toBe(false);
    expect(JSON.parse(fixture.stdout).status).toBe("VERIFIED");
  });

  test("never uploads pages when the exact signed package set is not public", () => {
    const fixture = pageDependencies({
      publicFailure: "signed package tag is missing",
    });
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID, "--dry-run"],
        fixture.dependencies,
      ),
    ).toBe(1);
    expect(fixture.stderr).toContain("signed package tag is missing");
    expect(
      fixture.calls.some((call) => call.command.endsWith("wrangler")),
    ).toBe(false);
    expect(
      fixture.calls.some(
        (call) => call.command === "git" && call.args.includes("push"),
      ),
    ).toBe(false);
  });

  test("reports Cloudflare state, not package refs, when page readback fails after upload", () => {
    const fixture = pageDependencies({ readbackFailure: "page bytes differ" });
    expect(
      runDeploy(
        [EDGE_FORM_PAGES_SURFACE, "--trust-set", SET_ID],
        fixture.dependencies,
      ),
    ).toBe(1);
    expect(
      fixture.calls.some((call) => call.command.endsWith("wrangler")),
    ).toBe(true);
    expect(fixture.stderr).toContain("deployment is indeterminate");
    expect(fixture.stderr).toContain("Cloudflare deployment history");
    expect(fixture.stderr).not.toContain("origin main, every expected tag");
  });
});

function pageDependencies({ publicFailure, readbackFailure } = {}) {
  const state = {
    calls: [],
    stdout: "",
    stderr: "",
    publicChecks: 0,
    readbacks: 0,
  };
  const plan = { family: "edge.forms.takoform.com", formCount: 1, forms: [] };
  const trust = { setId: SET_ID };
  const publicReadback = {
    kind: "takoform.edge-form-package-readback@v1",
    status: "VERIFIED",
    setId: SET_ID,
    tags: [],
  };
  state.dependencies = {
    readEdgeHistory() {
      return {
        absent: false,
        deployments: [
          {
            id: "deployment-fixture",
            versions: [
              {
                version_id: "11111111-1111-1111-1111-111111111111",
                percentage: 100,
              },
            ],
          },
        ],
      };
    },
    readEdgeFormPageInputs() {
      return { plan, trust };
    },
    verifyEdgeFormPackages() {
      state.publicChecks += 1;
      if (publicFailure) throw new Error(publicFailure);
      return publicReadback;
    },
    requireEdgeFormPageSourceIdentity() {
      return "0123456789abcdef0123456789abcdef01234567";
    },
    buildEdgeFormPages({ publicReadback: evidence }) {
      expect(evidence).toEqual(publicReadback);
      return {
        kind: "takoform.edge-form-pages-build@v1",
        signedSet: SET_ID,
        formCount: 1,
        routes: ["/", "/forms/ObjectBucket/0.1.0/"],
      };
    },
    readEdgeFormPages() {
      state.readbacks += 1;
      if (readbackFailure) throw new Error(readbackFailure);
    },
    run(command, args) {
      state.calls.push({ command, args });
      return {
        exitCode: 0,
        stdout: command.endsWith("wrangler")
          ? "Current Version ID: 11111111-1111-1111-1111-111111111111"
          : "",
        stderr: "",
      };
    },
    runReadOnly(command, args) {
      state.calls.push({ command, args });
      return { exitCode: 0, stdout: "", stderr: "" };
    },
    stdout(value) {
      state.stdout += value;
    },
    stderr(value) {
      state.stderr += value;
    },
  };
  return state;
}
