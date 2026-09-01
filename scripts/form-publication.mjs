#!/usr/bin/env bun

// Materialize the current Edge Form Package candidates at the exact
// content-addressed locators derived by the released Core package verifier.
// This script owns only the checked-in package copies. It does not publish a
// tag, talk to GitHub, or infer release identities itself.

import { createHash } from "node:crypto";
import {
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const ACTIVE_FAMILY = "edge.forms.takoform.com";
export const EXPECTED_FORM_COUNT = 17;
export const EXPECTED_RETAINED_PACKAGE_COUNT = 2;
export const EXPECTED_EVIDENCE_ONLY_PACKAGE_COUNT = 3;
export const RETAINED_PACKAGE_INVENTORY_RELATIVE =
  "forms/retained-packages.json";
export const ABANDONED_PREPUBLICATION_RELATIVE =
  "forms/trust/abandoned-prepublication.json";
export const ABANDONED_PREPUBLICATION_SET_ID =
  "cdd30b711e2c6857b1b4d247b1471f5676904933";
export const ABANDONED_PREPUBLICATION_SET_TAG = `forms/sets/${ABANDONED_PREPUBLICATION_SET_ID}`;
export const FORM_PACKAGE_VERIFY = [
  "go",
  "run",
  "./cmd/form-package",
  "verify",
];

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const currentFamilyIndexRelative = "forms/candidates/current-family-index.json";
const expectedCandidateSetRelative = `forms/candidates/${ACTIVE_FAMILY}/candidate-set.json`;
const publicationRootRelative = "forms/releases";
const digestPattern = /^sha256:[0-9a-f]{64}$/u;
const contentAddressedPackageFile = /^([^/]+)\/(sha256-[0-9a-f]{64})(?:\/.+)$/u;
const releaseIdPattern = /^[a-z0-9-]+$/u;
const artifactIdPattern = /^sha256-[0-9a-f]{64}$/u;

// This is a one-time, source-controlled recovery record. It is deliberately
// an exact allowlist rather than a wildcard for any historical package root.
// A second abandoned set needs a new format and explicit architecture
// decision; this manifest must not silently grow into a lifecycle registry.
const expectedEvidenceOnlyPackages = Object.freeze([
  Object.freeze({
    formRef: Object.freeze({
      apiVersion: ACTIVE_FAMILY,
      kind: "ObjectBucket",
      definitionVersion: "0.1.0",
      schemaDigest:
        "sha256:eeda7b2fe4450bdd2301a348c27d7ade81b0a94bf9708655875329d72f902c57",
    }),
    packageDigest:
      "sha256:52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
    releaseId: "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2pmjvgky3uij2wg23foq",
    artifactId:
      "sha256-52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
  }),
  Object.freeze({
    formRef: Object.freeze({
      apiVersion: ACTIVE_FAMILY,
      kind: "WorkerDeployment",
      definitionVersion: "0.2.0",
      schemaDigest:
        "sha256:247d64335cbff296efc0298aa6811f299714fe7187d29aec6f73ed734e978756",
    }),
    packageDigest:
      "sha256:f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
    releaseId:
      "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu",
    artifactId:
      "sha256-f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
  }),
  Object.freeze({
    formRef: Object.freeze({
      apiVersion: ACTIVE_FAMILY,
      kind: "WorkerVersion",
      definitionVersion: "0.3.0",
      schemaDigest:
        "sha256:e82dce714f8b623ca926379c855ee9e314c83262e5564828ccc37be2dbe05820",
    }),
    packageDigest:
      "sha256:d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
    releaseId: "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa",
    artifactId:
      "sha256-d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
  }),
]);

// These are the only two pre-current package identities that this publisher
// is authorized to retain. The inventory is an append-only locator manifest,
// not a wildcard permit for arbitrary content-addressed roots.
const expectedRetainedPackages = Object.freeze([
  Object.freeze({
    formRef: Object.freeze({
      apiVersion: ACTIVE_FAMILY,
      kind: "WorkerDeployment",
      definitionVersion: "0.1.0",
      schemaDigest:
        "sha256:0d2bca351b8ecade0a1ebbddf2463bba22910313ff916414112ec8762204e769",
    }),
    packageDigest:
      "sha256:535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
    releaseId:
      "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu",
    artifactId:
      "sha256-535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
  }),
  Object.freeze({
    formRef: Object.freeze({
      apiVersion: ACTIVE_FAMILY,
      kind: "WorkerVersion",
      definitionVersion: "0.2.0",
      schemaDigest:
        "sha256:3d4eeed966867a1ef8d7ce629a77c4b9687c6d48d3e496d22314b29aff0a42ed",
    }),
    packageDigest:
      "sha256:63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
    releaseId: "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa",
    artifactId:
      "sha256-63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
  }),
]);

if (import.meta.main) {
  try {
    const mode = process.argv[2];
    if (process.argv.length !== 3 || !["--write", "--check"].includes(mode)) {
      throw new Error(
        "usage: bun scripts/form-publication.mjs --write|--check",
      );
    }
    if (mode === "--write") {
      writePublication();
    } else {
      checkPublication();
    }
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}

/**
 * Read candidates and derive all publication identities through the public
 * Core CLI. The returned object contains absolute paths only for local use;
 * the identity fields are the Core-produced values and are safe to serialize.
 */
export function derivePublicationPlan({
  root = repositoryRoot,
  verifyPackage = verifyWithCore,
} = {}) {
  const retainedPackages = readRetainedPackageInventory(root);
  const evidenceOnlyPackages = readAbandonedPrepublication(root);
  const familyIndexPath = resolveRepositoryPath(
    root,
    currentFamilyIndexRelative,
  );
  const familyIndex = readJSON(familyIndexPath, "current-family index");
  if (familyIndex?.format !== "takoform.current-family-index@v1") {
    throw new Error("current-family index has an unexpected format");
  }
  if (!Array.isArray(familyIndex.families)) {
    throw new Error("current-family index families must be an array");
  }
  if (familyIndex.families.length !== 1) {
    throw new Error(
      `Edge publication requires exactly one active family; found ${familyIndex.families.length}`,
    );
  }
  const family = familyIndex.families[0];
  if (
    family?.group !== ACTIVE_FAMILY ||
    family.candidateSet !== expectedCandidateSetRelative ||
    family.formCount !== EXPECTED_FORM_COUNT
  ) {
    throw new Error(
      `active family must be ${ACTIVE_FAMILY} with ${EXPECTED_FORM_COUNT} Forms at ${expectedCandidateSetRelative}`,
    );
  }

  const candidateSetPath = resolveRepositoryPath(root, family.candidateSet);
  const candidateSetBytes = readRegularFile(candidateSetPath, "candidate set");
  const candidateSetDigest = sha256Hex(candidateSetBytes);
  if (candidateSetDigest !== family.sha256) {
    throw new Error(
      `candidate-set digest ${candidateSetDigest} does not match current-family index ${family.sha256}`,
    );
  }
  const candidateSet = parseJSON(candidateSetBytes, "candidate set");
  if (
    candidateSet?.format !== "takoform.form-family-candidates@v1" ||
    candidateSet.family !== ACTIVE_FAMILY ||
    !Array.isArray(candidateSet.forms) ||
    candidateSet.forms.length !== EXPECTED_FORM_COUNT
  ) {
    throw new Error(
      `candidate set must contain exactly ${EXPECTED_FORM_COUNT} Edge Forms`,
    );
  }

  const seenKinds = new Set();
  const seenTags = new Set();
  const forms = candidateSet.forms.map((candidate, index) => {
    if (
      candidate === null ||
      typeof candidate !== "object" ||
      typeof candidate.kind !== "string" ||
      candidate.kind.length === 0 ||
      seenKinds.has(candidate.kind) ||
      typeof candidate.path !== "string" ||
      typeof candidate.packageDigest !== "string" ||
      !digestPattern.test(candidate.packageDigest)
    ) {
      throw new Error(
        `candidate set Form[${index}] has an invalid or duplicate identity`,
      );
    }
    seenKinds.add(candidate.kind);
    const candidatePath = resolveRepositoryPath(root, candidate.path);
    const expectedPrefix = `forms/candidates/${ACTIVE_FAMILY}/`;
    if (!candidate.path.startsWith(expectedPrefix)) {
      throw new Error(
        `${candidate.kind}: candidate path escapes the Edge candidate root`,
      );
    }
    const locator = verifyPackage(candidatePath, root);
    if (
      locator === null ||
      typeof locator !== "object" ||
      locator.apiVersion !== "packages.forms.takoform.com/v1alpha5" ||
      typeof locator.releaseId !== "string" ||
      locator.releaseId.length === 0 ||
      typeof locator.artifactId !== "string" ||
      locator.artifactId.length === 0 ||
      typeof locator.tag !== "string" ||
      typeof locator.sourcePath !== "string"
    ) {
      throw new Error(
        `${candidate.kind}: Core returned an invalid publication locator`,
      );
    }
    const expectedArtifact = candidate.packageDigest.replace(":", "-");
    if (locator.artifactId !== expectedArtifact) {
      throw new Error(
        `${candidate.kind}: candidate packageDigest ${candidate.packageDigest} differs from Core package digest ${locator.artifactId}`,
      );
    }
    const expectedTag = `forms/${locator.releaseId}/${locator.artifactId}`;
    const expectedSourcePath = `forms/releases/${locator.releaseId}/${locator.artifactId}`;
    // Core owns release-ID encoding. We only ensure its returned source path
    // remains a safe repository-relative path before touching the filesystem.
    resolveRepositoryPath(root, expectedSourcePath);
    if (
      locator.tag !== expectedTag ||
      locator.sourcePath !== expectedSourcePath
    ) {
      throw new Error(
        `${candidate.kind}: Core locator does not use its canonical tag/source path`,
      );
    }
    if (seenTags.has(locator.tag)) {
      throw new Error(
        `${candidate.kind}: duplicate publication tag ${locator.tag}`,
      );
    }
    seenTags.add(locator.tag);
    return {
      kind: candidate.kind,
      role: candidate.role,
      candidatePath,
      candidateRelativePath: candidate.path,
      packageDigest: candidate.packageDigest,
      formRef: candidate.formRef,
      locator,
    };
  });

  forms.sort((left, right) =>
    compareStrings(left.locator.tag, right.locator.tag),
  );
  return {
    repositoryRoot: path.resolve(root),
    familyIndexPath,
    candidateSetPath,
    publicationRoot: resolveRepositoryPath(root, publicationRootRelative),
    family: ACTIVE_FAMILY,
    formCount: forms.length,
    currentPackageCount: forms.length,
    retainedPackageCount: retainedPackages.length,
    evidenceOnlyPackageCount: evidenceOnlyPackages.length,
    releaseRootCount:
      forms.length + retainedPackages.length + evidenceOnlyPackages.length,
    forms,
    retainedPackages,
    evidenceOnlyPackages,
  };
}

/**
 * Read the one-time abandoned prepublication recovery record. This is an
 * exact singleton allowlist: it is intentionally not a general historical
 * package registry, and a second abandoned set requires a new format and
 * explicit architecture decision.
 */
export function readAbandonedPrepublication(root = repositoryRoot) {
  const manifestPath = resolveRepositoryPath(
    root,
    ABANDONED_PREPUBLICATION_RELATIVE,
  );
  const manifest = readJSON(manifestPath, "abandoned prepublication manifest");
  if (
    manifest === null ||
    typeof manifest !== "object" ||
    Object.keys(manifest).sort().join(",") !==
      "disposition,evidenceOnlyPackages,family,format,setId,setTag" ||
    manifest.format !== "takoform.abandoned-prepublication@v1" ||
    manifest.family !== ACTIVE_FAMILY ||
    manifest.setId !== ABANDONED_PREPUBLICATION_SET_ID ||
    manifest.setTag !== ABANDONED_PREPUBLICATION_SET_TAG ||
    manifest.disposition !== "evidence-only" ||
    !Array.isArray(manifest.evidenceOnlyPackages) ||
    manifest.evidenceOnlyPackages.length !==
      EXPECTED_EVIDENCE_ONLY_PACKAGE_COUNT
  ) {
    throw new Error(
      `abandoned prepublication manifest must contain exactly ${EXPECTED_EVIDENCE_ONLY_PACKAGE_COUNT} evidence-only ${ACTIVE_FAMILY} roots`,
    );
  }
  const expectedByKey = new Map(
    expectedEvidenceOnlyPackages.map((entry) => [
      `${entry.formRef.kind}@${entry.formRef.definitionVersion}`,
      entry,
    ]),
  );
  const seen = new Set();
  return manifest.evidenceOnlyPackages.map((entry, index) => {
    if (entry === null || typeof entry !== "object") {
      throw new Error(
        `abandoned prepublication evidence entry[${index}] is invalid`,
      );
    }
    if (
      Object.keys(entry).sort().join(",") !==
      "artifactId,formRef,packageDigest,releaseId,sourcePath,tag"
    ) {
      throw new Error(
        `abandoned prepublication evidence entry[${index}] has unexpected fields`,
      );
    }
    const formRef = entry.formRef;
    if (
      formRef === null ||
      typeof formRef !== "object" ||
      Object.keys(formRef).sort().join(",") !==
        "apiVersion,definitionVersion,kind,schemaDigest" ||
      formRef.apiVersion !== ACTIVE_FAMILY ||
      typeof formRef.kind !== "string" ||
      typeof formRef.definitionVersion !== "string" ||
      !digestPattern.test(formRef.schemaDigest) ||
      !digestPattern.test(entry.packageDigest) ||
      typeof entry.releaseId !== "string" ||
      !releaseIdPattern.test(entry.releaseId) ||
      typeof entry.artifactId !== "string" ||
      !artifactIdPattern.test(entry.artifactId) ||
      typeof entry.tag !== "string" ||
      typeof entry.sourcePath !== "string"
    ) {
      throw new Error(
        `abandoned prepublication evidence entry[${index}] is invalid`,
      );
    }
    const key = `${formRef.kind}@${formRef.definitionVersion}`;
    const expected = expectedByKey.get(key);
    if (!expected || seen.has(key)) {
      throw new Error(
        `abandoned prepublication evidence entry[${index}] is not one of the exact abandoned identities`,
      );
    }
    seen.add(key);
    const expectedArtifact = entry.packageDigest.replace(":", "-");
    const expectedTag = `forms/${entry.releaseId}/${entry.artifactId}`;
    const expectedSourcePath = `${publicationRootRelative}/${entry.releaseId}/${entry.artifactId}`;
    if (
      entry.artifactId !== expectedArtifact ||
      entry.tag !== expectedTag ||
      entry.sourcePath !== expectedSourcePath ||
      JSON.stringify(formRef) !== JSON.stringify(expected.formRef) ||
      entry.packageDigest !== expected.packageDigest ||
      entry.releaseId !== expected.releaseId ||
      entry.artifactId !== expected.artifactId
    ) {
      throw new Error(
        `abandoned prepublication evidence entry ${key} differs from the exact abandoned identity`,
      );
    }
    const releasePath = resolveRepositoryPath(root, entry.sourcePath);
    return {
      formRef,
      packageDigest: entry.packageDigest,
      releaseId: entry.releaseId,
      artifactId: entry.artifactId,
      tag: entry.tag,
      sourcePath: entry.sourcePath,
      releasePath,
    };
  });
}

/**
 * Read the publisher-owned allowlist of immutable pre-current package roots.
 * Every field is checked against the exact retained identity; a valid Core
 * package at another path is not sufficient authority to enter this set.
 */
export function readRetainedPackageInventory(root = repositoryRoot) {
  const inventoryPath = resolveRepositoryPath(
    root,
    RETAINED_PACKAGE_INVENTORY_RELATIVE,
  );
  const inventory = readJSON(inventoryPath, "retained package inventory");
  if (
    inventory?.format !== "takoform.retained-package-inventory@v1" ||
    inventory.family !== ACTIVE_FAMILY ||
    !Array.isArray(inventory.packages) ||
    inventory.packages.length !== EXPECTED_RETAINED_PACKAGE_COUNT
  ) {
    throw new Error(
      `retained package inventory must contain exactly ${EXPECTED_RETAINED_PACKAGE_COUNT} ${ACTIVE_FAMILY} roots`,
    );
  }
  const expectedByKey = new Map(
    expectedRetainedPackages.map((entry) => [
      `${entry.formRef.kind}@${entry.formRef.definitionVersion}`,
      entry,
    ]),
  );
  const seen = new Set();
  return inventory.packages.map((entry, index) => {
    if (entry === null || typeof entry !== "object") {
      throw new Error(`retained package inventory entry[${index}] is invalid`);
    }
    const keys = Object.keys(entry).sort().join(",");
    if (keys !== "artifactId,formRef,packageDigest,releaseId,sourcePath,tag") {
      throw new Error(
        `retained package inventory entry[${index}] has unexpected fields`,
      );
    }
    const formRef = entry.formRef;
    if (
      formRef === null ||
      typeof formRef !== "object" ||
      Object.keys(formRef).sort().join(",") !==
        "apiVersion,definitionVersion,kind,schemaDigest" ||
      formRef.apiVersion !== ACTIVE_FAMILY ||
      typeof formRef.kind !== "string" ||
      typeof formRef.definitionVersion !== "string" ||
      !digestPattern.test(formRef.schemaDigest) ||
      !digestPattern.test(entry.packageDigest) ||
      typeof entry.releaseId !== "string" ||
      !releaseIdPattern.test(entry.releaseId) ||
      typeof entry.artifactId !== "string" ||
      !artifactIdPattern.test(entry.artifactId) ||
      typeof entry.tag !== "string" ||
      typeof entry.sourcePath !== "string"
    ) {
      throw new Error(`retained package inventory entry[${index}] is invalid`);
    }
    const key = `${formRef.kind}@${formRef.definitionVersion}`;
    const expected = expectedByKey.get(key);
    if (!expected || seen.has(key)) {
      throw new Error(
        `retained package inventory entry[${index}] is not one of the exact retained identities`,
      );
    }
    seen.add(key);
    const expectedArtifact = entry.packageDigest.replace(":", "-");
    const expectedTag = `forms/${entry.releaseId}/${entry.artifactId}`;
    const expectedSourcePath = `${publicationRootRelative}/${entry.releaseId}/${entry.artifactId}`;
    if (
      entry.artifactId !== expectedArtifact ||
      entry.tag !== expectedTag ||
      entry.sourcePath !== expectedSourcePath ||
      JSON.stringify(formRef) !== JSON.stringify(expected.formRef) ||
      entry.packageDigest !== expected.packageDigest ||
      entry.releaseId !== expected.releaseId ||
      entry.artifactId !== expected.artifactId
    ) {
      throw new Error(
        `retained package inventory entry[${index}] differs from the exact published identity`,
      );
    }
    const releasePath = resolveRepositoryPath(root, entry.sourcePath);
    return {
      formRef,
      packageDigest: entry.packageDigest,
      releaseId: entry.releaseId,
      artifactId: entry.artifactId,
      tag: entry.tag,
      sourcePath: entry.sourcePath,
      releasePath,
    };
  });
}

/** Verify one complete package with the checked-in public Core CLI. */
export function verifyWithCore(packageRoot, root = repositoryRoot) {
  const result = spawnSync(
    FORM_PACKAGE_VERIFY[0],
    [...FORM_PACKAGE_VERIFY.slice(1), packageRoot],
    {
      cwd: root,
      encoding: "utf8",
      maxBuffer: 16 * 1024 * 1024,
      // Core verification may acquire the released public module, but it uses
      // only the public Go proxy/checksum service and no operator credentials
      // or private Go/Git configuration.
      env: credentialFreeEnvironment(),
    },
  );
  if (result.status !== 0) {
    const detail = [result.stderr, result.stdout]
      .map((value) => String(value ?? "").trim())
      .filter(Boolean)
      .join("\n");
    throw new Error(
      `Core v1.1.0 package verification failed for ${path.relative(root, packageRoot)}${detail ? `:\n${detail}` : ""}`,
    );
  }
  let locator;
  try {
    locator = JSON.parse(result.stdout);
  } catch {
    throw new Error(
      `Core v1.1.0 package verifier returned non-JSON for ${path.relative(root, packageRoot)}`,
    );
  }
  return locator;
}

/**
 * Check the tracked release tree against the current candidate closures and
 * the exact retained-package inventory. Current roots are copied from the
 * candidate closure; retained roots are admitted only by their explicit
 * publisher-owned locator entries and are verified separately through Core.
 */
export function inspectPublicationTree(plan, { root = repositoryRoot } = {}) {
  const expected = new Map();
  const expectedPackageRoots = new Set();
  const failures = [];
  for (const form of plan.forms) {
    const releaseRoot = resolveRepositoryPath(root, form.locator.sourcePath);
    expectedPackageRoots.add(
      `${form.locator.releaseId}/${form.locator.artifactId}`,
    );
    const candidateFiles = inventoryFiles(form.candidatePath);
    for (const [relative, digest] of candidateFiles) {
      const releaseRelative = `${form.locator.sourcePath}/${relative}`;
      expected.set(releaseRelative, {
        digest,
        source: path.join(form.candidatePath, relative),
      });
    }
    // Keep the release root in the expected set even though an empty package
    // is impossible under the Core verifier; this also validates path shape.
    if (
      !releaseRoot.startsWith(
        resolveRepositoryPath(root, publicationRootRelative) + path.sep,
      )
    ) {
      throw new Error(`${form.kind}: Core source path escapes forms/releases`);
    }
  }

  const retainedPackages = plan.retainedPackages ?? [];
  for (const retained of retainedPackages) {
    const releaseRoot = resolveRepositoryPath(root, retained.sourcePath);
    const packageRoot = `${retained.releaseId}/${retained.artifactId}`;
    expectedPackageRoots.add(packageRoot);
    if (!pathExists(releaseRoot)) {
      failures.push(`${retained.sourcePath}: retained release root is missing`);
    }
    if (
      !releaseRoot.startsWith(
        resolveRepositoryPath(root, publicationRootRelative) + path.sep,
      )
    ) {
      throw new Error(
        `${retained.formRef.kind}: retained source path escapes forms/releases`,
      );
    }
  }

  const evidenceOnlyPackages = plan.evidenceOnlyPackages ?? [];
  for (const evidence of evidenceOnlyPackages) {
    const releaseRoot = resolveRepositoryPath(root, evidence.sourcePath);
    const packageRoot = `${evidence.releaseId}/${evidence.artifactId}`;
    expectedPackageRoots.add(packageRoot);
    if (!pathExists(releaseRoot)) {
      failures.push(
        `${evidence.sourcePath}: abandoned evidence-only release root is missing`,
      );
    }
    if (
      !releaseRoot.startsWith(
        resolveRepositoryPath(root, publicationRootRelative) + path.sep,
      )
    ) {
      throw new Error(
        `${evidence.formRef.kind}: abandoned evidence source path escapes forms/releases`,
      );
    }
  }

  const actual = new Map();
  const releaseRoot = resolveRepositoryPath(root, publicationRootRelative);
  if (pathExists(releaseRoot)) {
    for (const [relative, digest] of inventoryFiles(releaseRoot)) {
      actual.set(`${publicationRootRelative}/${relative}`, digest);
    }
  }
  for (const [relative, expectedEntry] of expected) {
    if (!actual.has(relative)) {
      failures.push(`${relative}: release file is missing`);
      continue;
    }
    if (actual.get(relative) !== expectedEntry.digest) {
      failures.push(
        `${relative}: release bytes diverge from candidate closure`,
      );
    }
  }
  const retainedRoots = new Set();
  const evidenceRoots = new Set();
  for (const relative of actual.keys()) {
    if (expected.has(relative)) continue;
    const packageRoot = contentAddressedPackageRoot(
      relative.startsWith(`${publicationRootRelative}/`)
        ? relative.slice(publicationRootRelative.length + 1)
        : relative,
    );
    if (packageRoot === null) {
      failures.push(`${relative}: extra release file`);
      continue;
    }
    if (expectedPackageRoots.has(packageRoot)) {
      if (
        retainedPackages.some(
          (entry) => `${entry.releaseId}/${entry.artifactId}` === packageRoot,
        )
      ) {
        retainedRoots.add(packageRoot);
      } else if (
        evidenceOnlyPackages.some(
          (entry) => `${entry.releaseId}/${entry.artifactId}` === packageRoot,
        )
      ) {
        evidenceRoots.add(packageRoot);
      } else {
        failures.push(`${relative}: extra release file`);
      }
    } else failures.push(`${relative}: unknown retained release root`);
  }
  return {
    expected,
    actual,
    failures,
    retainedRoots,
    evidenceRoots,
  };
}

export function verifyPublicationTree(
  plan,
  {
    root = repositoryRoot,
    verifyPackage = verifyWithCore,
    requireComplete = true,
  } = {},
) {
  const inspection = inspectPublicationTree(plan, { root });
  if (requireComplete && inspection.failures.length > 0) {
    throw new Error(
      `tracked Form Package release tree is not exact:\n${inspection.failures.join("\n")}`,
    );
  }
  verifyRetainedPublicationRoots(plan, { root, verifyPackage });
  verifyEvidenceOnlyPublicationRoots(plan, { root, verifyPackage });
  const checked = [];
  for (const form of plan.forms) {
    const releasePath = resolveRepositoryPath(root, form.locator.sourcePath);
    if (!pathExists(releasePath)) {
      if (requireComplete)
        throw new Error(`${form.kind}: release package is missing`);
      continue;
    }
    const locator = verifyPackage(releasePath, root);
    if (!sameLocator(locator, form.locator)) {
      throw new Error(
        `${form.kind}: release locator differs from Core-derived candidate locator`,
      );
    }
    checked.push(form.locator.tag);
  }
  return {
    ...inspection,
    checked,
  };
}

export function writePublication({
  root = repositoryRoot,
  verifyPackage = verifyWithCore,
} = {}) {
  const plan = derivePublicationPlan({ root, verifyPackage });
  const inspection = inspectPublicationTree(plan, { root });
  const existingPackageRoots = new Set(
    plan.forms
      .filter((form) =>
        pathExists(resolveRepositoryPath(root, form.locator.sourcePath)),
      )
      .map((form) => form.locator.sourcePath),
  );
  const blockingFailures = inspection.failures.filter((failure) => {
    if (!failure.endsWith(": release file is missing")) return true;
    const relative = failure.slice(0, -": release file is missing".length);
    return [...existingPackageRoots].some((packageRoot) =>
      relative.startsWith(`${packageRoot}/`),
    );
  });
  if (blockingFailures.length > 0) {
    throw new Error(
      `refusing to rewrite an existing release tree:\n${blockingFailures.join("\n")}`,
    );
  }
  // Validate retained package bytes before staging any new tree. Retained
  // identities are never created, replaced, or removed by this writer.
  verifyRetainedPublicationRoots(plan, { root, verifyPackage });
  // Abandoned evidence-only roots are also immutable and must already be
  // present; this writer never materializes or changes them.
  verifyEvidenceOnlyPublicationRoots(plan, { root, verifyPackage });

  const stagingParent = mkdtempSync(
    path.join(root, ".form-publication-build-"),
  );
  try {
    for (const form of plan.forms) {
      const target = resolveRepositoryPath(root, form.locator.sourcePath);
      if (pathExists(target)) continue;
      const staged = path.join(
        stagingParent,
        form.locator.releaseId,
        form.locator.artifactId,
      );
      copyTree(form.candidatePath, staged);
      mkdirSync(path.dirname(target), { recursive: true });
      // Never replace a directory that appeared after the inspection. A
      // concurrent writer must stop rather than silently overwrite it.
      if (pathExists(target)) {
        throw new Error(
          `${form.kind}: release target appeared during materialization`,
        );
      }
      renameSync(staged, target);
    }
  } finally {
    rmSync(stagingParent, { recursive: true, force: true });
  }
  verifyPublicationTree(plan, { root, verifyPackage });
  process.stdout.write(
    `wrote and verified ${plan.formCount} current Edge Form Package release directories (${plan.releaseRootCount ?? plan.formCount} release roots)\n`,
  );
}

export function checkPublication({
  root = repositoryRoot,
  verifyPackage = verifyWithCore,
} = {}) {
  const plan = derivePublicationPlan({ root, verifyPackage });
  verifyPublicationTree(plan, { root, verifyPackage });
  process.stdout.write(
    `verified ${plan.formCount} current Edge Form Package release directories and ${plan.releaseRootCount ?? plan.formCount} Core-derived release roots\n`,
  );
}

function resolveRepositoryPath(root, relative) {
  if (
    typeof relative !== "string" ||
    relative.length === 0 ||
    path.posix.isAbsolute(relative) ||
    relative.includes("\\") ||
    relative
      .split("/")
      .some((part) => part === "" || part === "." || part === "..")
  ) {
    throw new Error(
      `unsafe repository-relative path ${JSON.stringify(relative)}`,
    );
  }
  const resolvedRoot = path.resolve(root);
  const resolved = path.resolve(resolvedRoot, relative);
  if (
    resolved !== resolvedRoot &&
    !resolved.startsWith(`${resolvedRoot}${path.sep}`)
  ) {
    throw new Error(`repository path escapes root ${JSON.stringify(relative)}`);
  }
  return resolved;
}

function contentAddressedPackageRoot(relative) {
  const match = contentAddressedPackageFile.exec(relative);
  return match ? `${match[1]}/${match[2]}` : null;
}

function verifyRetainedPublicationRoots(
  plan,
  { root = repositoryRoot, verifyPackage = verifyWithCore } = {},
) {
  for (const retained of plan.retainedPackages ?? []) {
    const releasePath = resolveRepositoryPath(root, retained.sourcePath);
    if (!pathExists(releasePath)) {
      throw new Error(
        `${retained.formRef.kind}: retained release root ${retained.sourcePath} is missing`,
      );
    }
    const locator = verifyPackage(releasePath, root);
    const expectedLocator = {
      apiVersion: "packages.forms.takoform.com/v1alpha5",
      releaseId: retained.releaseId,
      artifactId: retained.artifactId,
      tag: retained.tag,
      sourcePath: retained.sourcePath,
    };
    if (!sameLocator(locator, expectedLocator)) {
      throw new Error(
        `retained release ${retained.sourcePath}: locator differs from the exact inventory entry`,
      );
    }
    const packageIndex = readJSON(
      path.join(releasePath, "package-index.json"),
      `${retained.formRef.kind} retained release package index`,
    );
    if (
      JSON.stringify(packageIndex.formRef) !== JSON.stringify(retained.formRef)
    ) {
      throw new Error(
        `retained release ${retained.sourcePath}: FormRef differs from the exact inventory entry`,
      );
    }
    if (locator.artifactId !== retained.packageDigest.replace(":", "-")) {
      throw new Error(
        `retained release ${retained.sourcePath}: Core package identity differs from the exact inventory digest`,
      );
    }
  }
}

function verifyEvidenceOnlyPublicationRoots(
  plan,
  { root = repositoryRoot, verifyPackage = verifyWithCore } = {},
) {
  for (const evidence of plan.evidenceOnlyPackages ?? []) {
    const releasePath = resolveRepositoryPath(root, evidence.sourcePath);
    if (!pathExists(releasePath)) {
      throw new Error(
        `${evidence.formRef.kind}: abandoned evidence-only release root ${evidence.sourcePath} is missing`,
      );
    }
    const locator = verifyPackage(releasePath, root);
    const expectedLocator = {
      apiVersion: "packages.forms.takoform.com/v1alpha5",
      releaseId: evidence.releaseId,
      artifactId: evidence.artifactId,
      tag: evidence.tag,
      sourcePath: evidence.sourcePath,
    };
    if (!sameLocator(locator, expectedLocator)) {
      throw new Error(
        `abandoned evidence-only release ${evidence.sourcePath}: locator differs from the exact manifest entry`,
      );
    }
    const packageIndex = readJSON(
      path.join(releasePath, "package-index.json"),
      `${evidence.formRef.kind} abandoned evidence-only release package index`,
    );
    if (
      JSON.stringify(packageIndex.formRef) !== JSON.stringify(evidence.formRef)
    ) {
      throw new Error(
        `abandoned evidence-only release ${evidence.sourcePath}: FormRef differs from the exact manifest entry`,
      );
    }
    if (locator.artifactId !== evidence.packageDigest.replace(":", "-")) {
      throw new Error(
        `abandoned evidence-only release ${evidence.sourcePath}: Core package identity differs from the exact manifest digest`,
      );
    }
  }
}

function sameLocator(left, right) {
  return (
    left?.apiVersion === right?.apiVersion &&
    left?.releaseId === right?.releaseId &&
    left?.artifactId === right?.artifactId &&
    left?.tag === right?.tag &&
    left?.sourcePath === right?.sourcePath
  );
}

function readJSON(file, label) {
  return parseJSON(readRegularFile(file, label), label);
}

function parseJSON(bytes, label) {
  try {
    return JSON.parse(bytes);
  } catch (error) {
    throw new Error(`${label} is not valid JSON: ${error.message}`);
  }
}

function readRegularFile(file, label) {
  if (!pathExists(file)) throw new Error(`${label} is missing: ${file}`);
  const info = lstatSync(file);
  if (!info.isFile())
    throw new Error(`${label} is not a regular file: ${file}`);
  return readFileSync(file);
}

function inventoryFiles(root) {
  const entries = [];
  if (!pathExists(root)) return entries;
  const walk = (directory, prefix = "") => {
    const info = lstatSync(directory);
    if (!info.isDirectory())
      throw new Error(`${directory}: release root is not a directory`);
    for (const name of readdirSync(directory).sort()) {
      const full = path.join(directory, name);
      const relative = prefix ? `${prefix}/${name}` : name;
      const child = lstatSync(full);
      if (child.isSymbolicLink())
        throw new Error(`${relative}: symlinks are forbidden`);
      if (child.isDirectory()) walk(full, relative);
      else if (child.isFile())
        entries.push([relative, sha256Digest(readFileSync(full))]);
      else throw new Error(`${relative}: unsupported filesystem entry`);
    }
  };
  walk(root);
  return entries;
}

function copyTree(source, destination) {
  const sourceInfo = lstatSync(source);
  if (!sourceInfo.isDirectory())
    throw new Error(`${source}: candidate package is not a directory`);
  mkdirSync(destination, { recursive: true });
  for (const name of readdirSync(source).sort()) {
    const sourcePath = path.join(source, name);
    const destinationPath = path.join(destination, name);
    const info = lstatSync(sourcePath);
    if (info.isSymbolicLink())
      throw new Error(`${name}: candidate package contains a symlink`);
    if (info.isDirectory()) copyTree(sourcePath, destinationPath);
    else if (info.isFile())
      writeFileSync(destinationPath, readFileSync(sourcePath), { mode: 0o644 });
    else
      throw new Error(
        `${name}: candidate package contains an unsupported filesystem entry`,
      );
  }
}

function sha256Digest(bytes) {
  return `sha256:${createHash("sha256").update(bytes).digest("hex")}`;
}

function sha256Hex(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function pathExists(file) {
  try {
    lstatSync(file);
    return true;
  } catch (error) {
    if (error?.code === "ENOENT") return false;
    throw error;
  }
}

function compareStrings(left, right) {
  if (left < right) return -1;
  if (left > right) return 1;
  return 0;
}

export function credentialFreeEnvironment(sourceEnvironment = process.env) {
  const env = { ...sourceEnvironment };
  for (const key of Object.keys(env)) {
    if (
      /(?:TOKEN|SECRET|PASSWORD|PRIVATE_KEY|CREDENTIAL|API_KEY|ACCESS_KEY)/iu.test(
        key,
      ) ||
      /^GIT_CONFIG_(?:COUNT|KEY_|VALUE_)/u.test(key)
    ) {
      delete env[key];
    }
  }
  for (const key of [
    "GIT_CONFIG",
    "GIT_CONFIG_GLOBAL",
    "GIT_CONFIG_NOSYSTEM",
    "GIT_CONFIG_PARAMETERS",
    "GIT_CONFIG_SYSTEM",
    "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    "GIT_ASKPASS",
    "GIT_COMMON_DIR",
    "GIT_DIR",
    "GIT_INDEX_FILE",
    "GIT_NAMESPACE",
    "GIT_OBJECT_DIRECTORY",
    "GIT_OPTIONAL_LOCKS",
    "SSH_ASKPASS",
    "SSH_AUTH_SOCK",
    "GIT_REPLACE_REF_BASE",
    "GIT_SHALLOW_FILE",
    "GIT_TERMINAL_PROMPT",
    "GIT_WORK_TREE",
    "GIT_PROXY_COMMAND",
    "GIT_SSH",
    "GIT_SSH_COMMAND",
    "NODE_OPTIONS",
    "BUN_OPTIONS",
    "NODE_PATH",
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "ALL_PROXY",
    "NO_PROXY",
    "http_proxy",
    "https_proxy",
    "all_proxy",
    "no_proxy",
  ]) {
    delete env[key];
  }
  env.GIT_TERMINAL_PROMPT = "0";
  env.GIT_CONFIG_NOSYSTEM = "1";
  env.GIT_CONFIG_GLOBAL = "/dev/null";
  env.GIT_OPTIONAL_LOCKS = "0";
  env.GOAUTH = "off";
  env.GOENV = "off";
  env.GOFLAGS = "-mod=readonly";
  env.GOINSECURE = "";
  env.GONOPROXY = "";
  env.GONOSUMDB = "";
  env.GOPRIVATE = "";
  env.GOPROXY = "https://proxy.golang.org";
  env.GOSUMDB = "sum.golang.org";
  env.GOTOOLCHAIN = "local";
  env.GOWORK = "off";
  env.NETRC = "/dev/null";
  return env;
}
