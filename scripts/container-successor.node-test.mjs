import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, test } from "node:test";
import { fileURLToPath } from "node:url";
import {
  CONTAINER_URL,
  ContainerContractError,
  MAX_IMAGE_DOCUMENT_BYTES,
  MAX_SECRET_NAMES,
  MAX_SECRET_TOTAL_BYTES,
  MAX_SECRET_VALUE_SCALARS,
  classifyPrivateInputs,
  normalizeContainerSpec,
  selectExecutableManifest,
  selectedImageBytes,
} from "../conformance/host-v2/container/0.2.0/contract.mjs";
import {
  CONTAINER_TARGETS,
  DEPLOYMENT_0_7_0_VERSIONS,
  WORKER_DEPLOYMENT_URLS,
  deploymentAcceptsVersion,
  deploymentFenceAllows,
  sealContainerBindings,
} from "../conformance/host-v2/container/0.2.0/callers.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const corpus = path.join(root, "conformance/host-v2/container/0.2.0");
const read = (name) =>
  JSON.parse(readFileSync(path.join(corpus, name), "utf8"));
const page = (form) =>
  readFileSync(
    path.join(root, "spec/forms", form.split("/forms/")[1], "index.md"),
    "utf8",
  );
const linked = (text, kind) =>
  [
    ...text.matchAll(
      new RegExp(`\\(\\.\\./\\.\\./${kind}/([0-9.]+)/\\)`, "gu"),
    ),
  ].map(
    ([, version]) =>
      `https://edge.forms.takoform.com/forms/${kind}/${version}/`,
  );
const spec = {
  image: `ghcr.io/example/app@sha256:${"a".repeat(64)}`,
  httpPort: 8080,
  healthPath: "/health",
  workloadRevision: "revision-1",
};

function outcome(run) {
  try {
    return run();
  } catch (error) {
    if (!(error instanceof ContainerContractError)) throw error;
    return { error: error.code };
  }
}

describe("Container 0.2.0 source proposal", () => {
  test("every corpus names the successor URL", () => {
    for (const name of [
      "private-input-cases.json",
      "image-cases.json",
      "binding-cases.json",
      "host-cases.json",
      "caller-cases.json",
      "successor-deltas.json",
    ])
      assert.equal(read(name).form, CONTAINER_URL, name);
  });

  test("private-input transitions follow the case expectations", () => {
    const cases = read("private-input-cases.json").privateInputCases;
    assert.ok(cases.length > 0);
    for (const entry of cases)
      assert.deepEqual(
        outcome(() => classifyPrivateInputs(entry)),
        entry.expect,
        entry.name,
      );
  });

  test("results never carry private values", () => {
    const value = "never-returned-value";
    const result = classifyPrivateInputs({
      operation: "create",
      spec: { ...spec, requiredSensitiveVars: ["API_TOKEN"] },
      privateInputs: { API_TOKEN: value },
      hostPrivateInputs: true,
    });
    assert.ok(!JSON.stringify(result).includes(value));
  });

  test("secret name and value limits are inclusive", () => {
    const names = (count) =>
      Array.from({ length: count }, (_, i) => `S${String(i).padStart(2, "0")}`);
    assert.equal(
      normalizeContainerSpec({
        ...spec,
        requiredSensitiveVars: names(MAX_SECRET_NAMES),
      }).requiredSensitiveVars.length,
      MAX_SECRET_NAMES,
    );
    assert.deepEqual(
      outcome(() =>
        normalizeContainerSpec({
          ...spec,
          requiredSensitiveVars: names(MAX_SECRET_NAMES + 1),
        }),
      ),
      { error: "invalid_spec" },
    );
    const create = (privateInputs) =>
      outcome(() =>
        classifyPrivateInputs({
          operation: "create",
          spec: { ...spec, requiredSensitiveVars: Object.keys(privateInputs) },
          privateInputs,
          hostPrivateInputs: true,
        }),
      );
    // Scalar values, not UTF-16 code units: one astral character counts once.
    const astral = "\u{1F600}".repeat(MAX_SECRET_VALUE_SCALARS);
    assert.equal(
      create({ A: "a".repeat(MAX_SECRET_VALUE_SCALARS) }).secrets,
      "set",
    );
    assert.equal(create({ A: astral }).secrets, "set");
    assert.deepEqual(create({ A: astral.slice(0, -2) + "ab" }), {
      error: "invalid_spec",
    });
    assert.deepEqual(create({ A: "a".repeat(MAX_SECRET_VALUE_SCALARS + 1) }), {
      error: "invalid_spec",
    });
    const quarter = MAX_SECRET_TOTAL_BYTES / 4;
    const fill = (count) =>
      Object.fromEntries(
        Array.from({ length: count }, (_, i) => [`V${i}`, "a".repeat(quarter)]),
      );
    assert.equal(create(fill(4)).secrets, "set");
    assert.deepEqual(create({ ...fill(4), V4: "a" }), {
      error: "invalid_spec",
    });
  });

  test("image selection and selected bytes follow the case expectations", () => {
    const { selectionCases, sizeCases } = read("image-cases.json");
    for (const entry of selectionCases) {
      const result = outcome(() => selectExecutableManifest(entry));
      assert.deepEqual(
        result.error ? result : { executableDigest: result.executableDigest },
        entry.expect,
        entry.name,
      );
      if (!result.error) assert.equal(result.sourceDigest, entry.sourceDigest);
    }
    assert.throws(
      () => selectedImageBytes({ config: {}, layers: [] }, undefined),
      TypeError,
    );
    for (const entry of sizeCases) {
      const result = outcome(() =>
        selectedImageBytes(entry.manifest, entry.limit),
      );
      assert.deepEqual(
        typeof result === "number" ? { selectedImageBytes: result } : result,
        entry.expect,
        entry.name,
      );
    }
  });

  test("Binding and host expectations stay within the existing names", () => {
    const binding = read("binding-cases.json");
    assert.equal(binding.binding, "module-worker.container");
    const early = binding.cases.filter((entry) =>
      entry.name.startsWith("early-"),
    );
    assert.equal(early.length, 3);
    for (const entry of early) {
      // Only the caller stream's own cancel callback may report the cancel.
      assert.match(entry.steps[0], /its own cancel callback/u, entry.name);
      assert.equal(entry.expect.uploadSourceCancelled, true, entry.name);
    }
    for (const entry of early.filter((e) => e.expect.uploadPumpClosed))
      assert.equal(entry.expect.nextCallStatus, 200, entry.name);
    assert.equal(
      binding.cases.find(
        (entry) => entry.name === "early-response-nonsettling-upload",
      ).expect.hostCountsCallComplete,
      false,
    );
    const host = read("host-cases.json");
    assert.deepEqual(host.profiles, [
      "cloudflare-containers",
      "docker",
      "linux-systemd-docker",
    ]);
    for (const entry of host.cases)
      assert.ok(
        entry.requiredProfiles.every((profile) =>
          host.profiles.includes(profile),
        ),
        entry.id,
      );
  });
  test("WorkerVersion and WorkerDeployment caller rules follow the cases", () => {
    const cases = read("caller-cases.json");
    for (const entry of cases.bindingCases) {
      // `for` lets a case resolve a UID to a record with a different uid.
      const targets = new Map(
        entry.targets.map((target) => [target.for ?? target.uid, target]),
      );
      const result = outcome(() =>
        sealContainerBindings(entry.bindings, {
          source: entry.source,
          resolve: (uid) => targets.get(uid),
        }),
      );
      assert.deepEqual(
        Array.isArray(result) ? { sealed: result } : result,
        entry.expect,
        entry.name,
      );
    }
    for (const entry of cases.deploymentVersionCases)
      assert.equal(
        deploymentAcceptsVersion(entry.form),
        entry.expect.accepted,
        entry.form,
      );
    for (const entry of cases.deploymentFenceCases)
      assert.equal(
        deploymentFenceAllows(entry),
        entry.expect.allowed,
        entry.name,
      );
  });

  test("the spec texts name exactly the checker's caller rules and limits", () => {
    // Each WorkerVersion's Binding target list, the frozen ones included.
    for (const [form, targets] of Object.entries(CONTAINER_TARGETS)) {
      const list = page(form).match(/Bindingの参照先は順に([\s\S]*?)である。/u);
      assert.ok(list, form);
      assert.deepEqual(linked(list[1], "Container"), targets, form);
    }
    const deployment = page(WORKER_DEPLOYMENT_URLS.at(-1));
    const selects = deployment.match(/^このFormは([\s\S]*?)の集合を選ぶ。/mu);
    assert.ok(selects);
    assert.deepEqual(
      linked(selects[1], "WorkerVersion"),
      DEPLOYMENT_0_7_0_VERSIONS,
    );
    const fence = deployment.match(
      /このWorkerDeployment ([0-9.]+)と旧WorkerDeployment ([^を]+)を跨いで/u,
    );
    assert.ok(fence);
    assert.deepEqual(
      [...fence[2].split("、"), fence[1]].map(
        (version) =>
          `https://edge.forms.takoform.com/forms/WorkerDeployment/${version}/`,
      ),
      WORKER_DEPLOYMENT_URLS,
    );
    // Container 0.2.0 limits in the text equal the checker's constants.
    const text = page(CONTAINER_URL);
    // Each limit is stated once, in its own sentence, with the checker's value.
    for (const [pattern, value] of [
      [/名前だけの宣言欄、最大(\d+)件/gu, MAX_SECRET_NAMES],
      [/各値は1〜(\d+) Unicodeスカラー値/gu, MAX_SECRET_VALUE_SCALARS],
      [/全値のUTF-8 bytes合計は(\d+)以下/gu, MAX_SECRET_TOTAL_BYTES],
      [/manifest文書はそれぞれ\s*(\d+) bytes以下/gu, MAX_IMAGE_DOCUMENT_BYTES],
    ]) {
      const found = [...text.matchAll(pattern)].map(([, number]) =>
        Number(number),
      );
      assert.deepEqual(found, [value], pattern.source);
    }
  });

  test("each successor differs from its frozen predecessor only by the recorded changes", () => {
    const freeze = JSON.parse(
      readFileSync(path.join(root, "spec/forms.freeze.json"), "utf8"),
    );
    const deltas = read("successor-deltas.json").successors;
    assert.deepEqual(
      deltas.map((entry) => entry.successor),
      [
        "spec/forms/WorkerVersion/0.8.0/index.md",
        "spec/forms/WorkerDeployment/0.7.0/index.md",
      ],
    );
    for (const entry of deltas) {
      const predecessor = readFileSync(
        path.join(root, entry.predecessor),
        "utf8",
      );
      const successor = readFileSync(path.join(root, entry.successor), "utf8");
      const frozen = freeze.forms.find(
        (form) => form.path === entry.predecessor,
      );
      assert.ok(frozen, `${entry.predecessor} is frozen`);
      assert.equal(
        `sha256:${createHash("sha256").update(predecessor).digest("hex")}`,
        frozen.sha256,
        entry.predecessor,
      );
      let forward = predecessor;
      for (const change of entry.changes) {
        assert.equal(
          forward.split(change.predecessor).length,
          2,
          change.predecessor,
        );
        forward = forward.replace(change.predecessor, () => change.successor);
      }
      let backward = successor;
      for (const change of [...entry.changes].reverse()) {
        assert.equal(
          backward.split(change.successor).length,
          2,
          change.successor,
        );
        backward = backward.replace(change.successor, () => change.predecessor);
      }
      assert.equal(forward, successor, entry.successor);
      assert.equal(backward, predecessor, entry.predecessor);
    }
  });
});
