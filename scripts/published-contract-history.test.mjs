import { describe, expect, test } from "bun:test";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  assertPublishedDefinitionsUnchanged,
  verifyPublishedBaseline,
  verifyPublishedSetHistory,
} from "./published-contract-history.mjs";

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);

function definition(kind, version, description = "published meaning") {
  const contract = ["InterfaceDefinition", "BindingDefinition"].includes(kind);
  return Buffer.from(
    JSON.stringify({
      apiVersion: contract
        ? `${kind.toLowerCase()}.example/v1`
        : "example.forms.test",
      kind,
      ...(contract
        ? { name: "example.actor", version }
        : { definitionVersion: version }),
      description,
    }),
  );
}

describe("published contract identity preservation", () => {
  test("preserves each published kind and allows distinct forward versions", () => {
    for (const kind of [
      "ActorNamespace",
      "InterfaceDefinition",
      "BindingDefinition",
    ]) {
      const previous = new Map([
        ["old/definition.json", definition(kind, "1.0.0")],
      ]);
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([["current/definition.json", definition(kind, "1.0.0")]]),
        ),
      ).not.toThrow();
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([
            [
              "current/definition.json",
              definition(kind, "1.1.0", "new meaning"),
            ],
          ]),
        ),
      ).not.toThrow();
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([
            [
              "renamed/definition.json",
              definition(kind, "1.0.0", "new meaning"),
            ],
          ]),
        ),
      ).toThrow(/published identity has different bytes/);
    }
  });

  test("a retained Form version cannot be reused with another meaning", () => {
    const published = new Map([
      ["current/definition.json", definition("WorkerVersion", "0.3.0")],
      [
        "retained/definition.json",
        definition("WorkerVersion", "0.2.0", "retained meaning"),
      ],
    ]);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        published,
        new Map([
          [
            "candidate/definition.json",
            definition("WorkerVersion", "0.2.0", "replacement meaning"),
          ],
        ]),
      ),
    ).toThrow(/published identity has different bytes/);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        published,
        new Map([
          [
            "candidate/definition.json",
            definition("WorkerVersion", "0.2.0", "retained meaning"),
          ],
        ]),
      ),
    ).not.toThrow();
  });

  test("rejects contradictory published identities and malformed current identities", () => {
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        new Map([
          ["first", definition("InterfaceDefinition", "1.0.0")],
          [
            "second",
            definition("InterfaceDefinition", "1.0.0", "other meaning"),
          ],
        ]),
        new Map(),
      ),
    ).toThrow(/snapshot reuses an exact identity/);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        new Map(),
        new Map([["candidate", definition("ActorNamespace", "latest")]]),
      ),
    ).toThrow(/invalid contract identity/);
  });

  test("fails when the exact local Git history is unavailable", () => {
    const root = mkdtempSync(path.join(tmpdir(), "missing-contract-history-"));
    try {
      expect(() => verifyPublishedBaseline(root)).toThrow(
        /unavailable locally/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("published set tags freeze displaced Workflow roots independently of the mutable inventory", () => {
    const root = makeTaggedHistoryFixture();
    try {
      expect(() => verifyPublishedSetHistory(root)).not.toThrow();
      const manifestPath = path.join(root, "forms/retained-packages.json");
      const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
      manifest.packages = manifest.packages.filter(
        (entry) =>
          !(
            entry.formRef.kind === "WorkerDeployment" &&
            entry.formRef.definitionVersion === "0.2.0"
          ),
      );
      writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
      expect(() => verifyPublishedSetHistory(root)).toThrow(
        /disappeared from current or retained inventory/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("removing a newly retained exact release root fails even when its inventory entry remains", () => {
    const root = makeTaggedHistoryFixture();
    try {
      const manifest = JSON.parse(
        readFileSync(path.join(root, "forms/retained-packages.json"), "utf8"),
      );
      const workflow = manifest.packages.find(
        (entry) => entry.formRef.kind === "DurableWorkflow",
      );
      rmSync(path.join(root, workflow.sourcePath), { recursive: true });
      expect(() => verifyPublishedSetHistory(root)).toThrow(
        /published tagged release root is missing/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("removing governing signed-set evidence fails independent of package inventory", () => {
    const root = makeTaggedHistoryFixture();
    try {
      rmSync(
        path.join(
          root,
          "forms/trust/sets/e7f8a39311dd011b8467e97e7f300cabb9a6b06c/publisher-policy.json",
        ),
      );
      expect(() => verifyPublishedSetHistory(root)).toThrow(
        /published tagged release root is missing/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("a fetched public set anchor rejects missing newer set and package refs", () => {
    const root = makeTaggedHistoryFixture(true);
    try {
      const fakeSet = "1111111111111111111111111111111111111111";
      const checkpoint = `forms/trust/sets/${fakeSet}/revocations/checkpoint.sigstore.json`;
      mkdirSync(path.dirname(path.join(root, checkpoint)), { recursive: true });
      writeFileSync(path.join(root, checkpoint), "{}\n");
      git(root, "read-tree", "HEAD");
      git(root, "add", checkpoint);
      const tree = git(root, "write-tree").trim();
      const commit = git(
        root,
        "-c",
        "user.name=Fixture",
        "-c",
        "user.email=fixture@example.test",
        "commit-tree",
        tree,
        "-p",
        git(root, "rev-parse", "HEAD").trim(),
        "-m",
        "fixture public set",
      ).trim();
      git(root, "update-ref", "refs/remotes/origin/main", commit);
      // No set or package tags for this public-main set remain locally.
      expect(() => verifyPublishedSetHistory(root)).toThrow(
        /previously published set tag is unavailable locally/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});

function git(root, ...args) {
  const result = spawnSync("git", args, { cwd: root, encoding: "utf8" });
  if (result.status !== 0)
    throw new Error(result.stderr || `git ${args.join(" ")} failed`);
  return result.stdout;
}

function makeTaggedHistoryFixture(independentGit = false) {
  const root = mkdtempSync(path.join(tmpdir(), "tagged-form-history-"));
  if (independentGit) {
    const clone = spawnSync(
      "git",
      ["clone", "--shared", "--no-checkout", "--quiet", repositoryRoot, root],
      { encoding: "utf8" },
    );
    if (clone.status !== 0)
      throw new Error(clone.stderr || "local Git clone failed");
    git(
      root,
      "update-ref",
      "refs/remotes/origin/main",
      git(repositoryRoot, "rev-parse", "refs/remotes/origin/main").trim(),
    );
  } else {
    symlinkSync(
      git(repositoryRoot, "rev-parse", "--absolute-git-dir").trim(),
      path.join(root, ".git"),
      "dir",
    );
  }
  mkdirSync(path.join(root, "forms/candidates/edge.forms.takoform.com"), {
    recursive: true,
  });
  cpSync(
    path.join(repositoryRoot, "forms/releases"),
    path.join(root, "forms/releases"),
    { recursive: true },
  );
  cpSync(
    path.join(repositoryRoot, "forms/trust/sets"),
    path.join(root, "forms/trust/sets"),
    { recursive: true },
  );
  cpSync(
    path.join(repositoryRoot, "forms/retained-packages.json"),
    path.join(root, "forms/retained-packages.json"),
  );
  cpSync(
    path.join(
      repositoryRoot,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    ),
    path.join(
      root,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    ),
  );
  return root;
}
