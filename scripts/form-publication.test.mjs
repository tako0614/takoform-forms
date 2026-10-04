import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import {
  cpSync,
  existsSync,
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
  verifyWithCore,
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
  // The real Core verifier checks 31 current and historical release roots;
  // Bun's default 5 s can cancel an in-flight verifier on a cold CI runner.
  test("selected Actor and Workflow source retains nine old signed roots without treating new roots as signed", () => {
    // The default reader re-verifies installed historical sets through Core;
    // it does not fabricate a signed successor for this source-only change.
    const plan = derivePublicationPlan();
    expect(plan.formCount).toBe(17);
    expect(plan.retainedPackageCount).toBe(11);
    expect(plan.evidenceOnlyPackageCount).toBe(3);
    expect(plan.releaseRootCount).toBe(40);
    expect(plan.formCount + plan.retainedPackageCount).toBe(28);
    const former = plan.retainedPackages.filter(
      (entry) =>
        !["WorkerVersion@0.2.0", "WorkerDeployment@0.1.0"].includes(
          `${entry.formRef.kind}@${entry.formRef.definitionVersion}`,
        ),
    );
    expect(former).toHaveLength(9);
    const activeKinds = new Set(plan.forms.map((entry) => entry.kind));
    for (const old of former) {
      expect(activeKinds.has(old.formRef.kind)).toBe(true);
      expect(plan.forms.some((entry) => entry.locator.tag === old.tag)).toBe(
        false,
      );
    }
    const checked = verifyPublicationTree(plan);
    expect(checked.checked).toHaveLength(17);
  }, 60_000);

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
    replacement.formRef.definitionVersion = "0.5.0";
    replacement.packageDigest = `sha256:${"6".repeat(64)}`;
    const candidateIndexPath = path.join(
      fixture,
      replacement.path,
      "package-index.json",
    );
    const packageIndex = JSON.parse(readFileSync(candidateIndexPath, "utf8"));
    packageIndex.formRef.definitionVersion = "0.5.0";
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

  test("writes current release directories while preserving archive and retained roots", () => {
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
    expect(listPackageRoots(fixture)).toHaveLength(31);
    for (const snapshot of snapshots) {
      expect(
        snapshotTree(
          path.join(fixture, "forms", "releases", snapshot.entry.relative),
        ),
      ).toEqual(snapshot.bytes);
    }
  });

  test("preserves exact prior-source roots as unsigned history while appending the selected successors", () => {
    const coreVerifier = (packageRoot) =>
      verifyWithCore(packageRoot, repositoryRoot);
    const { fixture, history, priorSnapshots } = makeSourceHistoryFixture();

    const plan = derivePublicationPlan({
      root: fixture,
      verifyPackage: coreVerifier,
    });
    const initialRoots = listPackageRoots(fixture);
    expect(plan.sourceHistoryPackageCount).toBe(9);
    expect(plan.formCount).toBe(17);
    expect(plan.retainedPackageCount).toBe(2);
    expect(plan.evidenceOnlyPackageCount).toBe(3);
    expect(plan.releaseRootCount).toBe(31);

    writePublication({ root: fixture, verifyPackage: coreVerifier });
    const checked = verifyPublicationTree(plan, {
      root: fixture,
      verifyPackage: coreVerifier,
    });
    expect(checked.checked).toHaveLength(17);
    expect(initialRoots).toHaveLength(22);
    expect(listPackageRoots(fixture)).toHaveLength(31);
    for (const prior of priorSnapshots) {
      expect(snapshotTree(prior.source)).toEqual(prior.bytes);
    }

    const roster = deriveSigningRoster({
      root: fixture,
      verifyPackage: coreVerifier,
    });
    expect(roster.activeReleasePaths).toHaveLength(17);
    for (const entry of history.packages) {
      expect(roster.activeReleasePaths).not.toContain(entry.sourcePath);
    }
  }, 60_000);

  test("rejects unknown, incomplete, changed, duplicate, cross-classified and current roots before writing", () => {
    const mutateCases = [
      ["unknown", (history) => (history.packages[0].formRef.kind = "Unknown")],
      ["missing", (history) => history.packages.pop()],
      [
        "duplicate",
        (history) => history.packages.push({ ...history.packages[0] }),
      ],
      ["source mismatch", (history) => (history.sourceCommit = "0".repeat(40))],
      [
        "missing root",
        (_history, fixture) => {
          const history = JSON.parse(
            readFileSync(
              path.join(fixture, "forms", "source-history.json"),
              "utf8",
            ),
          );
          rmSync(path.join(fixture, history.packages[0].sourcePath), {
            recursive: true,
            force: true,
          });
        },
      ],
      ["extra field", (history) => (history.packages[0].extra = true)],
      [
        "cross classified",
        (history, fixture) => {
          const retained = JSON.parse(
            readFileSync(
              path.join(fixture, "forms", "retained-packages.json"),
              "utf8",
            ),
          ).packages[0];
          history.packages[0] = retained;
        },
      ],
      [
        "current as history",
        (history, fixture) => {
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
          const current = candidateSet.forms.find(
            (candidate) => candidate.kind === history.packages[0].formRef.kind,
          );
          const prior = history.packages[0];
          prior.formRef = current.formRef;
          prior.packageDigest = current.packageDigest;
          prior.artifactId = current.packageDigest.replace(":", "-");
          prior.tag = `forms/${prior.releaseId}/${prior.artifactId}`;
          prior.sourcePath = `forms/releases/${prior.releaseId}/${prior.artifactId}`;
        },
      ],
      [
        "tampered bytes",
        (_history, fixture) => {
          const history = JSON.parse(
            readFileSync(
              path.join(fixture, "forms", "source-history.json"),
              "utf8",
            ),
          );
          const file = path.join(
            fixture,
            history.packages[0].sourcePath,
            "definition.json",
          );
          writeFileSync(file, `${readFileSync(file, "utf8")}\nchanged\n`);
        },
      ],
    ];
    for (const [label, mutate] of mutateCases) {
      const fixture = makeFixture();
      const historyPath = path.join(fixture, "forms", "source-history.json");
      const history = JSON.parse(readFileSync(historyPath, "utf8"));
      mutate(history, fixture);
      if (label !== "tampered bytes") {
        writeFileSync(
          path.join(fixture, "forms", "source-history.json"),
          `${JSON.stringify(history, null, 2)}\n`,
        );
      }
      const baselineRoots = listPackageRoots(fixture);
      expect(
        () =>
          writePublication({
            root: fixture,
            verifyPackage: makeFixtureVerifier(fixture),
          }),
        label,
      ).toThrow();
      expect(listPackageRoots(fixture), `${label} wrote release roots`).toEqual(
        baselineRoots,
      );
      rmSync(fixture, { recursive: true, force: true });
    }
  }, 60_000);

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
    expect(listPackageRoots(fixture)).toHaveLength(13);
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
  const sourceHistoryPath = path.join(
    repositoryRoot,
    "forms",
    "source-history.json",
  );
  cpSync(sourceHistoryPath, path.join(fixture, "forms", "source-history.json"));
  const sourceHistory = JSON.parse(readFileSync(sourceHistoryPath, "utf8"));
  for (const entry of sourceHistory.packages) {
    cpSync(
      path.join(repositoryRoot, entry.sourcePath),
      path.join(fixture, entry.sourcePath),
      { recursive: true },
    );
  }
  return fixture;
}

function makeSourceHistoryFixture() {
  const fixture = makeFixture();
  const coreVerifier = (packageRoot) =>
    verifyWithCore(packageRoot, repositoryRoot);
  const historyPath = path.join(fixture, "forms", "source-history.json");
  const history = JSON.parse(readFileSync(historyPath, "utf8"));
  const priorSnapshots = [];
  for (const entry of history.packages) {
    const source = path.join(repositoryRoot, entry.sourcePath);
    const target = path.join(fixture, entry.sourcePath);
    cpSync(source, target, { recursive: true });
    priorSnapshots.push({ source: target, bytes: snapshotTree(target) });
  }
  const plan = derivePublicationPlan({
    root: fixture,
    verifyPackage: coreVerifier,
  });
  const replacedKinds = new Set(
    history.packages.map((entry) => entry.formRef.kind),
  );
  for (const form of plan.forms) {
    if (replacedKinds.has(form.kind)) continue;
    const source = path.join(repositoryRoot, form.locator.sourcePath);
    cpSync(source, path.join(fixture, form.locator.sourcePath), {
      recursive: true,
    });
  }
  return { fixture, history, priorSnapshots };
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
