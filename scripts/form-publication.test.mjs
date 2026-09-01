import { describe, expect, test } from "bun:test";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  derivePublicationPlan,
  verifyPublicationTree,
  writePublication,
} from "./form-publication.mjs";

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const candidateRoot = path.join(
  repositoryRoot,
  "forms",
  "candidates",
  "edge.forms.takoform.com",
);

describe("Edge Form Package publication materialization", () => {
  test("writes every release directory when the publication root is empty", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const plan = derivePublicationPlan({ root: fixture, verifyPackage });

    writePublication({ root: fixture, verifyPackage });

    const checked = verifyPublicationTree(plan, {
      root: fixture,
      verifyPackage,
    });
    expect(checked.failures).toEqual([]);
    expect(checked.checked).toHaveLength(17);
  });

  test.each(["partial", "divergent"])(
    "refuses an existing %s release directory without changing it",
    (kind) => {
      const fixture = makeFixture();
      const verifyPackage = makeFixtureVerifier(fixture);
      const plan = derivePublicationPlan({ root: fixture, verifyPackage });
      const form = plan.forms[0];
      const target = path.join(fixture, form.locator.sourcePath);
      const source = path.join(fixture, form.candidateRelativePath);
      cpSync(source, target, { recursive: true });

      const definition = path.join(target, "definition.json");
      if (kind === "partial") {
        rmSync(path.join(target, "package-index.json"));
      } else {
        writeFileSync(
          definition,
          `${readFileSync(definition, "utf8")}\nchanged\n`,
        );
      }
      const before = readFileSync(definition);

      expect(() => writePublication({ root: fixture, verifyPackage })).toThrow(
        /refusing to rewrite an existing release tree/,
      );
      expect(readFileSync(definition)).toEqual(before);
    },
  );

  test("retains immutable historical roots while adding changed identities", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const historical = findHistoricalPackageRoots();
    const snapshots = [];
    for (const entry of historical) {
      const target = path.join(fixture, "forms", "releases", entry.relative);
      cpSync(entry.source, target, { recursive: true });
      snapshots.push({ entry, bytes: snapshotTree(target) });
    }

    const plan = derivePublicationPlan({ root: fixture, verifyPackage });
    writePublication({ root: fixture, verifyPackage });
    const checked = verifyPublicationTree(plan, {
      root: fixture,
      verifyPackage,
    });

    expect(checked.checked).toHaveLength(17);
    expect(listPackageRoots(fixture)).toHaveLength(19);
    for (const snapshot of snapshots) {
      expect(
        snapshotTree(
          path.join(fixture, "forms", "releases", snapshot.entry.relative),
        ),
      ).toEqual(snapshot.bytes);
    }
  });

  test("rejects a third Core-valid package root outside the retained inventory", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const unlisted = findCurrentPackageRoot("ObjectBucket");
    const target = path.join(
      fixture,
      "forms",
      "releases",
      "unlisted-object-bucket",
      "sha256-5277d10da8ca9531cd98ac098266bfe709757cdec444648b748defd9f4a28e45",
    );
    cpSync(unlisted, target, { recursive: true });
    const plan = derivePublicationPlan({ root: fixture, verifyPackage });
    expect(() =>
      verifyPublicationTree(plan, { root: fixture, verifyPackage }),
    ).toThrow(/unknown retained release root/);
  });

  test("rejects a missing retained root before writing current packages", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const inventory = JSON.parse(
      readFileSync(
        path.join(fixture, "forms", "retained-packages.json"),
        "utf8",
      ),
    );
    rmSync(path.join(fixture, inventory.packages[0].sourcePath), {
      recursive: true,
      force: true,
    });
    const plan = derivePublicationPlan({ root: fixture, verifyPackage });
    expect(() => writePublication({ root: fixture, verifyPackage })).toThrow(
      /retained release root is missing/,
    );
    expect(listPackageRoots(fixture)).toHaveLength(1);
    expect(plan.retainedPackageCount).toBe(2);
  });

  test("rejects divergent retained bytes and inventory tags", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const inventoryPath = path.join(fixture, "forms", "retained-packages.json");
    const inventory = JSON.parse(readFileSync(inventoryPath, "utf8"));
    const retained = inventory.packages[0];
    const plan = derivePublicationPlan({ root: fixture, verifyPackage });
    writePublication({ root: fixture, verifyPackage });
    const indexPath = path.join(
      fixture,
      retained.sourcePath,
      "package-index.json",
    );
    const packageIndex = JSON.parse(readFileSync(indexPath, "utf8"));
    packageIndex.formRef.definitionVersion = "9.9.9";
    writeFileSync(indexPath, `${JSON.stringify(packageIndex, null, 2)}\n`);
    expect(() =>
      verifyPublicationTree(plan, { root: fixture, verifyPackage }),
    ).toThrow(/FormRef differs from the exact inventory entry/);

    inventory.packages[0].tag = `${retained.tag}-divergent`;
    writeFileSync(inventoryPath, `${JSON.stringify(inventory, null, 2)}\n`);
    expect(() =>
      derivePublicationPlan({ root: fixture, verifyPackage }),
    ).toThrow(/differs from the exact published identity/);
  });
});

function makeFixture() {
  const fixture = mkdtempSync(
    path.join(tmpdir(), "takoform-publication-test-"),
  );
  const candidateDestination = path.join(
    fixture,
    "forms",
    "candidates",
    "edge.forms.takoform.com",
  );
  mkdirSync(candidateDestination, { recursive: true });
  cpSync(
    path.join(
      repositoryRoot,
      "forms",
      "candidates",
      "current-family-index.json",
    ),
    path.join(fixture, "forms", "candidates", "current-family-index.json"),
  );
  cpSync(candidateRoot, candidateDestination, { recursive: true });
  cpSync(
    path.join(repositoryRoot, "forms", "retained-packages.json"),
    path.join(fixture, "forms", "retained-packages.json"),
  );
  for (const entry of JSON.parse(
    readFileSync(path.join(fixture, "forms", "retained-packages.json"), "utf8"),
  ).packages) {
    const source = path.join(repositoryRoot, entry.sourcePath);
    const target = path.join(fixture, entry.sourcePath);
    cpSync(source, target, { recursive: true });
  }
  return fixture;
}

function makeFixtureVerifier(fixture) {
  const candidateSet = JSON.parse(
    readFileSync(
      path.join(
        fixture,
        "forms",
        "candidates",
        "edge.forms.takoform.com",
        "candidate-set.json",
      ),
      "utf8",
    ),
  );
  const locators = new Map(
    candidateSet.forms.map((candidate, index) => {
      const releaseId = `k-${"abcdefghijklmnop"[index]}`;
      const artifactId = candidate.packageDigest.replace(":", "-");
      return [
        candidate.kind,
        {
          apiVersion: "packages.forms.takoform.com/v1alpha5",
          releaseId,
          artifactId,
          tag: `forms/${releaseId}/${artifactId}`,
          sourcePath: `forms/releases/${releaseId}/${artifactId}`,
        },
      ];
    }),
  );
  return (packageRoot) => {
    const packageIndex = JSON.parse(
      readFileSync(path.join(packageRoot, "package-index.json"), "utf8"),
    );
    const relative = path
      .relative(fixture, packageRoot)
      .split(path.sep)
      .join("/");
    if (relative.startsWith("forms/releases/")) {
      const [, , releaseId, artifactId] = relative.split("/");
      return {
        apiVersion: "packages.forms.takoform.com/v1alpha5",
        releaseId,
        artifactId,
        tag: `forms/${releaseId}/${artifactId}`,
        sourcePath: `forms/releases/${releaseId}/${artifactId}`,
      };
    }
    const locator = locators.get(packageIndex.formRef?.kind);
    if (!locator) throw new Error(`unknown fixture package ${packageRoot}`);
    return locator;
  };
}

function findHistoricalPackageRoots() {
  const root = path.join(repositoryRoot, "forms", "releases");
  const wanted = new Map([
    ["WorkerVersion", "0.2.0"],
    ["WorkerDeployment", "0.1.0"],
  ]);
  const found = [];
  for (const releaseId of readdirSync(root)) {
    const releaseDirectory = path.join(root, releaseId);
    for (const artifactId of readdirSync(releaseDirectory)) {
      const source = path.join(releaseDirectory, artifactId);
      const index = path.join(source, "package-index.json");
      let formRef;
      try {
        formRef = JSON.parse(readFileSync(index, "utf8")).formRef;
      } catch {
        continue;
      }
      if (wanted.get(formRef?.kind) !== formRef?.definitionVersion) continue;
      found.push({
        kind: formRef.kind,
        source,
        relative: `${releaseId}/${artifactId}`,
      });
    }
  }
  if (found.length !== wanted.size) {
    throw new Error(`historical package roots found: ${found.length}`);
  }
  return found;
}

function findCurrentPackageRoot(kind) {
  const source = path.join(
    candidateRoot,
    kind.replaceAll(/([a-z])([A-Z])/gu, "$1-$2").toLowerCase(),
  );
  if (!readFileSync(path.join(source, "package-index.json"))) {
    throw new Error(`current package root not found for ${kind}`);
  }
  return source;
}

function snapshotTree(root) {
  const result = {};
  const walk = (directory, prefix = "") => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const child = path.join(directory, entry.name);
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
      if (entry.isDirectory()) {
        walk(child, relative);
      } else if (entry.isFile()) {
        result[relative] = readFileSync(child).toString("base64");
      } else {
        throw new Error(`unsupported fixture entry ${child}`);
      }
    }
  };
  walk(root);
  return result;
}

function listPackageRoots(fixture) {
  const root = path.join(fixture, "forms", "releases");
  const roots = [];
  for (const releaseId of readdirSync(root)) {
    for (const artifactId of readdirSync(path.join(root, releaseId))) {
      roots.push(`${releaseId}/${artifactId}`);
    }
  }
  return roots;
}
