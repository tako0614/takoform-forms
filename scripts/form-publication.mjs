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
export const EXPECTED_FORM_COUNT = 16;
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
    forms,
  };
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
      `Core v1.0.1 package verification failed for ${path.relative(root, packageRoot)}${detail ? `:\n${detail}` : ""}`,
    );
  }
  let locator;
  try {
    locator = JSON.parse(result.stdout);
  } catch {
    throw new Error(
      `Core v1.0.1 package verifier returned non-JSON for ${path.relative(root, packageRoot)}`,
    );
  }
  return locator;
}

/**
 * Check the tracked release tree against the current candidate closures. This
 * is deliberately strict: missing, extra, changed, symlinked, and non-regular
 * release entries all fail closed.
 */
export function inspectPublicationTree(plan, { root = repositoryRoot } = {}) {
  const expected = new Map();
  for (const form of plan.forms) {
    const releaseRoot = resolveRepositoryPath(root, form.locator.sourcePath);
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

  const actual = new Map();
  const releaseRoot = resolveRepositoryPath(root, publicationRootRelative);
  if (pathExists(releaseRoot)) {
    for (const [relative, digest] of inventoryFiles(releaseRoot)) {
      actual.set(`${publicationRootRelative}/${relative}`, digest);
    }
  }
  const failures = [];
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
  for (const relative of actual.keys()) {
    if (!expected.has(relative))
      failures.push(`${relative}: extra release file`);
  }
  return {
    expected,
    actual,
    failures,
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
    `wrote and verified ${plan.formCount} Edge Form Package release directories\n`,
  );
}

export function checkPublication({
  root = repositoryRoot,
  verifyPackage = verifyWithCore,
} = {}) {
  const plan = derivePublicationPlan({ root, verifyPackage });
  verifyPublicationTree(plan, { root, verifyPackage });
  process.stdout.write(
    `verified ${plan.formCount} Edge Form Package release directories and Core-derived locators\n`,
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
    "GIT_CONFIG_PARAMETERS",
    "GIT_CONFIG_SYSTEM",
    "GIT_ASKPASS",
    "SSH_ASKPASS",
    "SSH_AUTH_SOCK",
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
