import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
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
  deriveSigningRoster,
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
  test("signing roster is the complete validated current selection, never retained or abandoned roots", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    writePublication({ root: fixture, verifyPackage });
    const roster = deriveSigningRoster({ root: fixture, verifyPackage });
    const plan = derivePublicationPlan({ root: fixture, verifyPackage });
    expect(roster.packageCount).toBe(17);
    expect(roster.activeReleasePaths).toEqual(
      plan.forms.map((form) => form.locator.sourcePath),
    );
    expect(roster.activeReleasePaths).not.toContain(
      plan.retainedPackages[0].sourcePath,
    );
    expect(roster.activeReleasePaths).not.toContain(
      plan.evidenceOnlyPackages[0].sourcePath,
    );
  });

  test("a version replacement keeps the verified prior release and tag outside the new active roster", () => {
    const fixture = makeFixture();
    const initialVerifier = makeFixtureVerifier(fixture);
    writePublication({ root: fixture, verifyPackage: initialVerifier });
    const prior = derivePublicationPlan({
      root: fixture,
      verifyPackage: initialVerifier,
    });
    const former = prior.forms.find((form) => form.kind === "WorkerVersion");
    const priorBytes = snapshotTree(
      path.join(fixture, former.locator.sourcePath),
    );
    const signedSets = () => [
      {
        status: "verified",
        setId: "a".repeat(40),
        family: prior.family,
        packageCount: prior.formCount,
        checkpointHistory: [{ setId: "a".repeat(40) }],
        packages: prior.forms.map((form) => ({
          formRef: form.formRef,
          packageDigest: form.packageDigest,
          locator: form.locator,
        })),
      },
    ];
    const candidateSetPath = path.join(
      fixture,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    );
    const candidateSet = JSON.parse(readFileSync(candidateSetPath, "utf8"));
    const replacement = candidateSet.forms.find(
      (form) => form.kind === "WorkerVersion",
    );
    replacement.formRef.definitionVersion = "0.4.0";
    replacement.packageDigest = `sha256:${"6".repeat(64)}`;
    const candidateIndexPath = path.join(
      fixture,
      replacement.path,
      "package-index.json",
    );
    const packageIndex = JSON.parse(readFileSync(candidateIndexPath, "utf8"));
    packageIndex.formRef.definitionVersion = "0.4.0";
    writeFileSync(
      candidateIndexPath,
      `${JSON.stringify(packageIndex, null, 2)}\n`,
    );
    const candidateSetBytes = `${JSON.stringify(candidateSet, null, 2)}\n`;
    writeFileSync(candidateSetPath, candidateSetBytes);
    const familyIndexPath = path.join(
      fixture,
      "forms/candidates/current-family-index.json",
    );
    const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
    familyIndex.families[0].sha256 = createHash("sha256")
      .update(candidateSetBytes)
      .digest("hex");
    writeFileSync(familyIndexPath, `${JSON.stringify(familyIndex, null, 2)}\n`);
    const verifyPackage = makeFixtureVerifier(fixture);
    const options = {
      root: fixture,
      verifyPackage,
      readSignedSets: signedSets,
    };
    writePublication(options);
    const next = derivePublicationPlan(options);
    expect(next.formCount).toBe(17);
    expect(
      next.retainedPackages.some((entry) => entry.tag === former.locator.tag),
    ).toBe(true);
    expect(
      next.forms.some((entry) => entry.locator.tag === former.locator.tag),
    ).toBe(false);
    expect(snapshotTree(path.join(fixture, former.locator.sourcePath))).toEqual(
      priorBytes,
    );
    expect(deriveSigningRoster(options).activeReleasePaths).not.toContain(
      former.locator.sourcePath,
    );
  });

  test("rejects changing a signed Kind and definitionVersion without advancing the Form version", () => {
    const fixture = makeFixture();
    const initialVerifier = makeFixtureVerifier(fixture);
    writePublication({ root: fixture, verifyPackage: initialVerifier });
    const prior = derivePublicationPlan({
      root: fixture,
      verifyPackage: initialVerifier,
    });
    const candidateSetPath = path.join(
      fixture,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    );
    const candidateSet = JSON.parse(readFileSync(candidateSetPath, "utf8"));
    const changed = candidateSet.forms.find(
      (form) => form.kind === "WorkerVersion",
    );
    changed.formRef.schemaDigest = `sha256:${"8".repeat(64)}`;
    changed.packageDigest = `sha256:${"9".repeat(64)}`;
    const packageIndexPath = path.join(
      fixture,
      changed.path,
      "package-index.json",
    );
    const packageIndex = JSON.parse(readFileSync(packageIndexPath, "utf8"));
    packageIndex.formRef.schemaDigest = changed.formRef.schemaDigest;
    writeFileSync(
      packageIndexPath,
      `${JSON.stringify(packageIndex, null, 2)}\n`,
    );
    const raw = `${JSON.stringify(candidateSet, null, 2)}\n`;
    writeFileSync(candidateSetPath, raw);
    const familyIndexPath = path.join(
      fixture,
      "forms/candidates/current-family-index.json",
    );
    const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
    familyIndex.families[0].sha256 = createHash("sha256")
      .update(raw)
      .digest("hex");
    writeFileSync(familyIndexPath, `${JSON.stringify(familyIndex, null, 2)}\n`);
    const options = {
      root: fixture,
      verifyPackage: makeFixtureVerifier(fixture),
      readSignedSets: () => [
        {
          status: "verified",
          setId: "a".repeat(40),
          family: prior.family,
          packageCount: prior.formCount,
          checkpointHistory: [{ setId: "a".repeat(40) }],
          packages: prior.forms.map((form) => ({
            formRef: form.formRef,
            packageDigest: form.packageDigest,
            locator: form.locator,
          })),
        },
      ],
    };
    expect(() => derivePublicationPlan(options)).toThrow(/historical FormRef/);
  });

  test("a separately promoted nineteen-item source selection drives the complete signing roster", () => {
    const fixture = makeFixture();
    const candidateSetPath = path.join(
      fixture,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    );
    const candidateSet = JSON.parse(readFileSync(candidateSetPath, "utf8"));
    for (const [index, kind] of [
      "SyntheticContainer",
      "SyntheticContainerDeployment",
    ].entries()) {
      const original = candidateSet.forms[index];
      const copy = structuredClone(original);
      copy.kind = kind;
      copy.path = `forms/candidates/edge.forms.takoform.com/synthetic-${index}`;
      copy.formRef.kind = kind;
      copy.packageDigest = `sha256:${String(index + 7).repeat(64)}`;
      cpSync(path.join(fixture, original.path), path.join(fixture, copy.path), {
        recursive: true,
      });
      const packageIndexPath = path.join(
        fixture,
        copy.path,
        "package-index.json",
      );
      const packageIndex = JSON.parse(readFileSync(packageIndexPath, "utf8"));
      packageIndex.formRef.kind = kind;
      writeFileSync(
        packageIndexPath,
        `${JSON.stringify(packageIndex, null, 2)}\n`,
      );
      candidateSet.forms.push(copy);
    }
    const candidateSetBytes = `${JSON.stringify(candidateSet, null, 2)}\n`;
    writeFileSync(candidateSetPath, candidateSetBytes);
    const familyIndexPath = path.join(
      fixture,
      "forms/candidates/current-family-index.json",
    );
    const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
    familyIndex.families[0].formCount = 19;
    familyIndex.families[0].sha256 = createHash("sha256")
      .update(candidateSetBytes)
      .digest("hex");
    writeFileSync(familyIndexPath, `${JSON.stringify(familyIndex, null, 2)}\n`);
    const verifyPackage = makeFixtureVerifier(fixture);
    writePublication({ root: fixture, verifyPackage });
    const roster = deriveSigningRoster({ root: fixture, verifyPackage });
    expect(roster.packageCount).toBe(19);
    expect(roster.activeReleasePaths).toHaveLength(19);
    expect(roster.activeReleasePaths).toEqual(
      [...roster.activeReleasePaths].sort(),
    );
  });

  test("rejects a forked signed-set history as historical-root authority", () => {
    const fixture = makeFixture();
    const verifyPackage = makeFixtureVerifier(fixture);
    const report = (id) => ({
      status: "verified",
      setId: id,
      family: "edge.forms.takoform.com",
      packageCount: 17,
      checkpointHistory: [{ setId: id }],
      packages: [],
    });
    expect(() =>
      derivePublicationPlan({
        root: fixture,
        verifyPackage,
        readSignedSets: () => [report("a".repeat(40)), report("b".repeat(40))],
      }),
    ).toThrow(/one complete successor history/);
  });

  test("rejects a candidate FormRef different from its selected package index", () => {
    const fixture = makeFixture();
    const candidateSetPath = path.join(
      fixture,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    );
    const candidateSet = JSON.parse(readFileSync(candidateSetPath, "utf8"));
    candidateSet.forms[0].formRef.definitionVersion = "9.9.9";
    const raw = `${JSON.stringify(candidateSet, null, 2)}\n`;
    writeFileSync(candidateSetPath, raw);
    const familyIndexPath = path.join(
      fixture,
      "forms/candidates/current-family-index.json",
    );
    const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
    familyIndex.families[0].sha256 = createHash("sha256")
      .update(raw)
      .digest("hex");
    writeFileSync(familyIndexPath, `${JSON.stringify(familyIndex, null, 2)}\n`);
    expect(() =>
      derivePublicationPlan({
        root: fixture,
        verifyPackage: makeFixtureVerifier(fixture),
      }),
    ).toThrow(/candidate FormRef differs/);
  });

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
    expect(listPackageRoots(fixture)).toHaveLength(22);
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
    expect(listPackageRoots(fixture)).toHaveLength(4);
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

  test("requires the one exact abandoned prepublication manifest", () => {
    const fixture = makeFixture();
    const manifestPath = path.join(
      fixture,
      "forms",
      "trust",
      "abandoned-prepublication.json",
    );
    const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
    manifest.evidenceOnlyPackages.pop();
    writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
    expect(() => derivePublicationPlan({ root: fixture })).toThrow(
      /exactly 3 evidence-only/,
    );

    const expanded = JSON.parse(
      readFileSync(
        path.join(
          repositoryRoot,
          "forms",
          "trust",
          "abandoned-prepublication.json",
        ),
        "utf8",
      ),
    );
    expanded.extra = true;
    writeFileSync(manifestPath, `${JSON.stringify(expanded, null, 2)}\n`);
    expect(() => derivePublicationPlan({ root: fixture })).toThrow(
      /exactly 3 evidence-only/,
    );
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
  mkdirSync(path.join(fixture, "forms", "trust"), { recursive: true });
  cpSync(
    path.join(
      repositoryRoot,
      "forms",
      "trust",
      "abandoned-prepublication.json",
    ),
    path.join(fixture, "forms", "trust", "abandoned-prepublication.json"),
  );
  for (const entry of JSON.parse(
    readFileSync(
      path.join(fixture, "forms", "trust", "abandoned-prepublication.json"),
      "utf8",
    ),
  ).evidenceOnlyPackages) {
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
      const releaseId = `k-${index.toString(36)}`;
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
