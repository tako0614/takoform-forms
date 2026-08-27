import { describe, expect, test } from "bun:test";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
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
    expect(checked.checked).toHaveLength(16);
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
    const locator = locators.get(packageIndex.formRef?.kind);
    if (!locator) throw new Error(`unknown fixture package ${packageRoot}`);
    return locator;
  };
}
