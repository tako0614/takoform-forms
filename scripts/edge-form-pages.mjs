#!/usr/bin/env bun

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderVitePressPages } from "./edge-form-pages-vitepress.mjs";

import {
  ABANDONED_PREPUBLICATION_SET_ID,
  derivePublicationPlan,
  verifyWithCore,
} from "./form-publication.mjs";

export const EDGE_FORM_PAGES_ORIGIN = "https://edge.forms.takoform.com";
export const EDGE_FORM_PAGES_SURFACE = "edge-form-pages";
export const EDGE_FORM_PAGES_WORKER = "takoform-edge-form-pages";

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const commitPattern = /^[0-9a-f]{40}$/u;

/**
 * Render the publisher's own signed package set as human-readable static
 * pages. Package definitions and canonical locators are the only semantic
 * inputs; this generator is neither a registry nor a discovery API.
 */
export function buildEdgeFormPages({
  outputDirectory,
  plan,
  trust,
  publicReadback,
  root = repositoryRoot,
}) {
  if (!outputDirectory) throw new Error("outputDirectory is required");
  const current = exactSignedForms(plan, trust, root);
  const retained = (plan.retainedPackages ?? []).map((entry) =>
    readPagePackage(
      {
        ...entry,
        locator: { tag: entry.tag, sourcePath: entry.sourcePath },
        retained: true,
      },
      root,
    ),
  );
  const guide = readJSON(path.join(root, "site/reading-guide.json"));
  if (
    Object.keys(guide).sort().join() !==
    current
      .map((form) => form.formRef.kind)
      .sort()
      .join()
  )
    throw new Error("reading guide must cover every current Form exactly");
  for (const form of current) {
    form.guide = guide[form.formRef.kind];
    if (
      form.guide.version !== form.formRef.definitionVersion ||
      !form.guide.purpose ||
      !form.guide.note ||
      typeof form.guide.ja?.purpose !== "string" ||
      !form.guide.ja.purpose.trim() ||
      typeof form.guide.ja?.note !== "string" ||
      !form.guide.ja.note.trim() ||
      !form.guide.related.every((kind) => guide[kind])
    )
      throw new Error(
        `${form.formRef.kind}: reading guide requires review for this exact definition version`,
      );
  }
  const forms = [...current, ...retained];
  const identities = new Set(
    forms.map(
      (form) => `${form.formRef.kind}/${form.formRef.definitionVersion}`,
    ),
  );
  if (identities.size !== forms.length)
    throw new Error("duplicate versioned page identity");
  for (const form of forms) {
    form.related = (form.guide?.related ?? []).map((kind) =>
      current.find((entry) => entry.formRef.kind === kind),
    );
    form.versions = forms.filter(
      (entry) => entry.formRef.kind === form.formRef.kind && entry !== form,
    );
  }
  const isPublic = validatePublicReadback(publicReadback, current, trust);
  if (
    isPublic &&
    JSON.stringify(
      (publicReadback.retainedTags ?? [])
        .map((entry) => `${entry.tag}\t${entry.packageDigest}`)
        .sort(),
    ) !==
      JSON.stringify(
        retained
          .map((entry) => `${entry.locator.tag}\t${entry.packageDigest}`)
          .sort(),
      )
  )
    throw new Error("public readback does not cover retained package history");

  // Never delete caller-selected directories. Builds use a fresh destination.
  if (
    existsSync(outputDirectory) &&
    (lstatSync(outputDirectory).isSymbolicLink() ||
      readdirSync(outputDirectory).length)
  ) {
    throw new Error("outputDirectory must be an empty, non-symlink directory");
  }
  mkdirSync(outputDirectory, { recursive: true });
  const routes = renderVitePressPages({
    root,
    outputDirectory,
    forms,
    trust,
    isPublic,
    origin: EDGE_FORM_PAGES_ORIGIN,
  });
  const files = assetFiles(outputDirectory);
  const digest = createHash("sha256");
  for (const relative of files)
    digest
      .update(relative)
      .update("\0")
      .update(readFileSync(path.join(outputDirectory, relative)));
  return {
    kind: "takoform.edge-form-pages-build@v1",
    surface: EDGE_FORM_PAGES_SURFACE,
    origin: EDGE_FORM_PAGES_ORIGIN,
    signedSet: trust.setId,
    publicPackageReadback: isPublic,
    formCount: current.length,
    retainedCount: retained.length,
    routes,
    files,
    digest: `sha256:${digest.digest("hex")}`,
  };
}

function assetFiles(directory, prefix = "") {
  return readdirSync(directory, { withFileTypes: true })
    .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    .flatMap((entry) => {
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
      return entry.isDirectory()
        ? assetFiles(path.join(directory, entry.name), relative)
        : [relative];
    });
}

export function exactSignedForms(plan, trust, root = repositoryRoot) {
  if (
    trust?.status !== "verified" ||
    trust.coreVersion !== "v1.1.0" ||
    trust.family !== plan?.family ||
    !commitPattern.test(trust.setId ?? "") ||
    trust.sourceCommit !== trust.setId ||
    trust.packageCount !== plan?.formCount ||
    !Array.isArray(trust.packages) ||
    trust.packages.length !== plan.formCount
  ) {
    throw new Error(
      "signed package closure is not one exact installed Core v1.1.0 set",
    );
  }
  const byTag = new Map(plan.forms.map((form) => [form.locator.tag, form]));
  const forms = trust.packages.map((signed) => {
    const planned = byTag.get(signed?.locator?.tag);
    if (
      !planned ||
      signed.kind !== planned.kind ||
      signed.packageDigest !== planned.packageDigest ||
      JSON.stringify(signed.formRef) !== JSON.stringify(planned.formRef) ||
      JSON.stringify(signed.locator) !== JSON.stringify(planned.locator)
    ) {
      throw new Error(
        "signed package closure differs from the installed publication plan",
      );
    }
    byTag.delete(signed.locator.tag);
    return readPagePackage(signed, root);
  });
  if (byTag.size !== 0) {
    throw new Error(
      "signed package closure differs from the installed publication plan",
    );
  }
  return forms.sort((left, right) =>
    `${left.formRef.kind}@${left.formRef.definitionVersion}`.localeCompare(
      `${right.formRef.kind}@${right.formRef.definitionVersion}`,
    ),
  );
}

function readPagePackage(entry, root) {
  const packageRoot = resolveWithinRoot(root, entry.locator.sourcePath);
  const verified = verifyWithCore(packageRoot, root);
  if (
    verified.tag !== entry.locator.tag ||
    verified.sourcePath !== entry.locator.sourcePath
  )
    throw new Error("page package bytes differ from canonical locator");
  const packageIndex = readJSON(path.join(packageRoot, "package-index.json"));
  const definition = readJSON(
    resolveWithinRoot(packageRoot, packageIndex.definitionPath),
  );
  if (JSON.stringify(packageIndex.formRef) !== JSON.stringify(entry.formRef))
    throw new Error("page package FormRef differs from inventory");
  const fixtures =
    definition.conformanceFixtures?.filter(
      (fixture) => fixture.name === "canonical",
    ) ?? [];
  if (
    fixtures.length !== 1 ||
    !packageIndex.files.some((file) => file.path === fixtures[0].desiredPath)
  )
    throw new Error("page requires one package-declared canonical fixture");
  const desiredPath = fixtures[0].desiredPath;
  const example = readJSON(resolveWithinRoot(packageRoot, desiredPath));
  return { ...entry, definition, example, desiredPath };
}

export function readInstalledTrustSet(setId, root = repositoryRoot) {
  if (
    !commitPattern.test(setId ?? "") ||
    setId === ABANDONED_PREPUBLICATION_SET_ID
  ) {
    throw new Error(`trust set ${setId || "<missing>"} is not deployable`);
  }
  const result = spawnSync(
    "go",
    [
      "run",
      "./cmd/publisher-trust",
      "verify-set",
      "--repository",
      root,
      "--set",
      path.join(root, "forms", "trust", "sets", setId),
    ],
    { cwd: root, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 },
  );
  if (result.status !== 0) {
    throw new Error(
      `signed set verification failed${result.stderr ? `:\n${result.stderr.trim()}` : ""}`,
    );
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    throw new Error("signed set verifier returned non-JSON");
  }
}

function validatePublicReadback(readback, forms, trust) {
  if (readback === undefined) return false;
  const expected = forms
    .map(
      (form) =>
        `${form.locator.tag}\t${form.locator.sourcePath}\t${form.packageDigest}`,
    )
    .sort();
  const actual = Array.isArray(readback?.tags)
    ? readback.tags
        .map(
          (entry) =>
            `${entry?.tag}\t${entry?.sourcePath}\t${entry?.packageDigest}`,
        )
        .sort()
    : [];
  if (
    readback?.kind !== "takoform.edge-form-package-readback@v1" ||
    readback.status !== "VERIFIED" ||
    readback.setId !== trust.setId ||
    expected.join("\n") !== actual.join("\n")
  ) {
    throw new Error(
      "public readback does not cover the exact signed package set",
    );
  }
  return true;
}

function readJSON(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function resolveWithinRoot(root, relative) {
  const resolved = path.resolve(root, relative);
  if (!resolved.startsWith(`${path.resolve(root)}${path.sep}`)) {
    throw new Error(`package path escapes repository: ${relative}`);
  }
  return resolved;
}

function parseCLI(args) {
  if (args.length === 0) return { mode: "build" };
  if (
    args.length === 3 &&
    args[0] === "--check" &&
    args[1] === "--trust-set" &&
    commitPattern.test(args[2])
  )
    return { mode: "check", setId: args[2] };
  if (args.length === 1 && args[0] === "--check") return { mode: "check" };
  if (
    args.length === 4 &&
    args[0] === "--trust-set" &&
    commitPattern.test(args[1]) &&
    args[2] === "--output"
  ) {
    return {
      mode: "build",
      setId: args[1],
      outputDirectory: path.resolve(args[3]),
    };
  }
  throw new Error(
    "usage: bun scripts/edge-form-pages.mjs --check | --trust-set <40-hex-set> --output <directory>",
  );
}

function runCLI(args) {
  const invocation = parseCLI(args);
  const plan = derivePublicationPlan();
  let setId = invocation.setId;
  let outputDirectory = invocation.outputDirectory;
  let temporary;
  if (!setId) setId = selectInstalledPageSet(plan);
  if (!outputDirectory) {
    temporary = mkdtempSync(path.join(tmpdir(), "edge-form-pages-check-"));
    outputDirectory = path.join(temporary, "assets");
  }
  try {
    const trust = readInstalledTrustSet(setId);
    const result = buildEdgeFormPages({ outputDirectory, plan, trust });
    if (invocation.mode === "check") {
      const wrangler = spawnSync(
        path.join(repositoryRoot, "node_modules", ".bin", "wrangler"),
        [
          "deploy",
          "--cwd",
          temporary,
          "--config",
          path.join(repositoryRoot, "site", "wrangler.jsonc"),
          "--assets",
          outputDirectory,
          "--dry-run",
          "--outdir",
          path.join(temporary, "wrangler"),
        ],
        { cwd: repositoryRoot, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 },
      );
      if (wrangler.status !== 0) {
        throw new Error(
          `Wrangler page build dry-run failed${wrangler.stderr ? `:\n${wrangler.stderr.trim()}` : ""}`,
        );
      }
    }
    process.stdout.write(
      `${JSON.stringify({ ...result, outputDirectory }, null, 2)}\n`,
    );
  } finally {
    if (temporary && invocation.mode === "check")
      rmSync(temporary, { recursive: true, force: true });
  }
}

function selectInstalledPageSet(plan) {
  const installed = readdirSync(
    path.join(repositoryRoot, "forms", "trust", "sets"),
  )
    .filter(
      (name) =>
        commitPattern.test(name) && name !== ABANDONED_PREPUBLICATION_SET_ID,
    )
    .sort();
  const matching = [];
  for (const candidate of installed) {
    const trust = readInstalledTrustSet(candidate);
    try {
      exactSignedForms(plan, trust);
      matching.push(trust);
    } catch (error) {
      if (
        !String(error instanceof Error ? error.message : error).includes(
          "signed package closure differs",
        )
      ) {
        throw error;
      }
    }
  }
  if (matching.length === 0) {
    throw new Error(
      "no installed signed set exactly matches the current package closure",
    );
  }
  const highestSequence = Math.max(
    ...matching.map((trust) => trust.checkpoint?.pin?.sequence ?? -1),
  );
  const current = matching.filter(
    (trust) => trust.checkpoint?.pin?.sequence === highestSequence,
  );
  if (highestSequence < 0 || current.length !== 1) {
    throw new Error(
      "installed signed sets do not identify one latest checkpoint for the current package closure",
    );
  }
  return current[0].setId;
}

if (import.meta.main) {
  try {
    runCLI(process.argv.slice(2));
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}
