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
import { verifyEdgeFormFreeze } from "./edge-form-freeze.mjs";
import { validateEdgeV2PublicationEntries } from "./edge-v2-docs.mjs";

import {
  ABANDONED_PREPUBLICATION_SET_ID,
  derivePublicationPlan,
  verifyPublicationTree,
  verifyWithCore,
} from "./form-publication.mjs";

export const EDGE_FORM_PAGES_ORIGIN = "https://edge.forms.takoform.com";
export const EDGE_FORM_PAGES_SURFACE = "edge-form-pages";
export const EDGE_FORM_PAGES_WORKER = "takoform-edge-form-pages";
export const PUBLISHED_V1_SET_ID = "e7f8a39311dd011b8467e97e7f300cabb9a6b06c";
export const PUBLISHED_V1_GUIDE_COMMIT =
  "79a31729e80f9210b951fc6c0e4f857816e4d2ab";
export const PUBLISHED_V1_GUIDE_SHA256 =
  "b9f4f498428c94000019f3e1e2928142a7f34510e9aeb2b5473779f80de1eb24";

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
  publishedForms = [],
  readingGuide,
  root = repositoryRoot,
}) {
  if (!outputDirectory) throw new Error("outputDirectory is required");
  validateEdgeV2PublicationEntries(publishedForms);
  if (publishedForms.length) {
    const freeze = verifyEdgeFormFreeze(root, {
      mode: "check",
      requireFrozen: true,
    });
    if (freeze.status !== "FROZEN")
      throw new Error("invalid Edge Form freeze manifest");
    const frozen = new Map(freeze.frozen.map((entry) => [entry.url, entry]));
    for (const entry of publishedForms) {
      if (JSON.stringify(frozen.get(entry.url)) !== JSON.stringify(entry))
        throw new Error(`selected Form is not frozen: ${entry.url}`);
    }
  }
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
  const forms = preparePageForms(current, retained, root, readingGuide);
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
  prepareOutputDirectory(outputDirectory);
  const routes = renderVitePressPages({
    root,
    outputDirectory,
    forms,
    trust,
    isPublic,
    origin: EDGE_FORM_PAGES_ORIGIN,
    publishedForms,
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
    publishedV2FormCount: publishedForms.length,
    routes,
    files,
    digest: `sha256:${digest.digest("hex")}`,
  };
}

/** Check-only preview of Core-verified source packages; never a signed build. */
export function buildEdgeFormSourcePreview({
  outputDirectory,
  plan,
  root = repositoryRoot,
}) {
  if (!outputDirectory) throw new Error("outputDirectory is required");
  verifyPublicationTree(plan, { root });
  const current = plan.forms.map((form) => readPagePackage(form, root));
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
  const forms = preparePageForms(current, retained, root);
  prepareOutputDirectory(outputDirectory);
  const routes = renderVitePressPages({
    root,
    outputDirectory,
    forms,
    sourcePreview: true,
    origin: EDGE_FORM_PAGES_ORIGIN,
  });
  const files = assetFiles(outputDirectory);
  return {
    kind: "takoform.edge-form-pages-source-check@v1",
    surface: EDGE_FORM_PAGES_SURFACE,
    publicationStatus: "UNPUBLISHED",
    formCount: current.length,
    retainedCount: retained.length,
    routeCount: routes.length,
    assetCount: files.length,
  };
}

function prepareOutputDirectory(outputDirectory) {
  // Never delete caller-selected directories. Builds use a fresh destination.
  if (
    existsSync(outputDirectory) &&
    (lstatSync(outputDirectory).isSymbolicLink() ||
      readdirSync(outputDirectory).length)
  ) {
    throw new Error("outputDirectory must be an empty, non-symlink directory");
  }
  mkdirSync(outputDirectory, { recursive: true });
}

function preparePageForms(current, retained, root, readingGuide) {
  const guide =
    readingGuide ?? readJSON(path.join(root, "site/reading-guide.json"));
  validateReadingGuide(guide, current);
  const forms = [...current, ...retained];
  const identities = new Set(
    forms.map(
      (form) => `${form.formRef.kind}/${form.formRef.definitionVersion}`,
    ),
  );
  if (identities.size !== forms.length)
    throw new Error("duplicate versioned page identity");
  for (const form of forms) {
    form.related = (form.guide?.related ?? []).map((relation) => ({
      ...relation,
      form: current.find((entry) => entry.formRef.kind === relation.kind),
    }));
    form.versions = forms.filter(
      (entry) => entry.formRef.kind === form.formRef.kind && entry !== form,
    );
  }
  return forms;
}

/** Site-only view of the already signed v1 roster; never promotes current candidates. */
export function publishedV1PagePlan(sourcePlan, trust) {
  if (
    trust?.status !== "verified" ||
    trust?.family !== sourcePlan?.family ||
    !Array.isArray(trust.packages) ||
    trust.packages.length !== trust.packageCount
  )
    throw new Error(
      "cannot derive published v1 pages without an exact verified set",
    );
  const activeTags = new Set(trust.packages.map((entry) => entry.locator?.tag));
  if (activeTags.size !== trust.packages.length || activeTags.has(undefined))
    throw new Error("signed v1 page roster has duplicate or missing locators");
  const retainedPackages = (sourcePlan.retainedPackages ?? []).filter(
    (entry) => !activeTags.has(entry.tag),
  );
  return {
    ...sourcePlan,
    formCount: trust.packageCount,
    forms: trust.packages,
    retainedPackages,
  };
}

/** Exact last pre-successor reading guide from this repository's public history. */
export function readPublishedV1Guide(root = repositoryRoot) {
  const ancestor = spawnSync(
    "git",
    [
      "--no-replace-objects",
      "merge-base",
      "--is-ancestor",
      PUBLISHED_V1_GUIDE_COMMIT,
      "HEAD",
    ],
    {
      cwd: root,
      encoding: "utf8",
      env: { ...process.env, GIT_NO_REPLACE_OBJECTS: "1" },
    },
  );
  if (ancestor.status !== 0)
    throw new Error("published v1 reading-guide source is not in HEAD history");
  const read = spawnSync(
    "git",
    [
      "--no-replace-objects",
      "show",
      `${PUBLISHED_V1_GUIDE_COMMIT}:site/reading-guide.json`,
    ],
    {
      cwd: root,
      encoding: null,
      env: { ...process.env, GIT_NO_REPLACE_OBJECTS: "1" },
    },
  );
  if (
    read.status !== 0 ||
    `sha256:${createHash("sha256").update(read.stdout).digest("hex")}` !==
      `sha256:${PUBLISHED_V1_GUIDE_SHA256}`
  )
    throw new Error(
      "published v1 reading-guide bytes differ from pinned history",
    );
  return JSON.parse(read.stdout.toString("utf8"));
}

export function validateReadingGuide(guide, current) {
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
      typeof form.guide.useCase !== "string" ||
      !form.guide.useCase.trim() ||
      typeof form.guide.ja?.purpose !== "string" ||
      !form.guide.ja.purpose.trim() ||
      typeof form.guide.ja?.note !== "string" ||
      !form.guide.ja.note.trim() ||
      typeof form.guide.ja?.useCase !== "string" ||
      !form.guide.ja.useCase.trim() ||
      !Array.isArray(form.guide.related) ||
      !form.guide.related.length ||
      new Set(form.guide.related.map((entry) => entry.kind)).size !==
        form.guide.related.length ||
      !form.guide.related.every(
        (entry) =>
          guide[entry.kind] &&
          entry.kind !== form.formRef.kind &&
          typeof entry.relation === "string" &&
          entry.relation.trim() &&
          typeof entry.ja === "string" &&
          entry.ja.trim(),
      )
    )
      throw new Error(
        `${form.formRef.kind}: reading guide requires review for this exact definition version`,
      );
  }
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
  if (args.length === 1 && args[0] === "--check-source")
    return { mode: "source-check" };
  if (
    [3, 4, 5].includes(args.length) &&
    args[0] === "--check" &&
    args[1] === "--trust-set" &&
    commitPattern.test(args[2]) &&
    (args.length === 3 ||
      (args.length === 4 && args[3] === "--include-frozen") ||
      (args.length === 5 && args[3] === "--published-forms"))
  )
    return {
      mode: "check",
      setId: args[2],
      includeFrozen: args.length === 4,
      publishedForms:
        args.length === 5 ? parsePublishedFormsArgument(args[4]) : [],
    };
  if (args.length === 1 && args[0] === "--check") return { mode: "check" };
  if (
    [4, 5].includes(args.length) &&
    args[0] === "--trust-set" &&
    commitPattern.test(args[1]) &&
    args[2] === "--output" &&
    (args.length === 4 || args[4] === "--include-frozen")
  ) {
    return {
      mode: "build",
      setId: args[1],
      outputDirectory: path.resolve(args[3]),
      includeFrozen: args.length === 5,
    };
  }
  throw new Error(
    "usage: bun scripts/edge-form-pages.mjs --check-source | --check [--trust-set <40-hex-set> [--published-forms <base64url-json>|--include-frozen]] | --trust-set <40-hex-set> --output <directory> [--include-frozen]",
  );
}

function parsePublishedFormsArgument(value) {
  if (
    typeof value !== "string" ||
    value.length > 65536 ||
    !/^[A-Za-z0-9_-]+$/u.test(value)
  )
    throw new Error("invalid --published-forms argument");
  let entries;
  try {
    entries = JSON.parse(Buffer.from(value, "base64url").toString("utf8"));
  } catch {
    throw new Error("invalid --published-forms JSON");
  }
  return validateEdgeV2PublicationEntries(entries);
}

function runCLI(args) {
  const invocation = parseCLI(args);
  const sourcePlan = derivePublicationPlan();
  let setId = invocation.setId;
  let outputDirectory = invocation.outputDirectory;
  let temporary;
  if (invocation.mode !== "source-check" && !setId) setId = PUBLISHED_V1_SET_ID;
  if (!outputDirectory) {
    temporary = mkdtempSync(path.join(tmpdir(), "edge-form-pages-check-"));
    outputDirectory = path.join(temporary, "assets");
  }
  try {
    const trust =
      invocation.mode === "source-check" ? null : readInstalledTrustSet(setId);
    const plan = trust ? publishedV1PagePlan(sourcePlan, trust) : sourcePlan;
    const freeze = invocation.includeFrozen
      ? verifyEdgeFormFreeze(repositoryRoot, {
          mode: "check",
          requireFrozen: true,
        })
      : null;
    if (freeze && (freeze.status !== "FROZEN" || !freeze.frozen.length))
      throw new Error(
        `no frozen authored Form closure for publication build: ${freeze.status}`,
      );
    const publishedForms = freeze?.frozen ?? invocation.publishedForms ?? [];
    const result =
      invocation.mode === "source-check"
        ? buildEdgeFormSourcePreview({ outputDirectory, plan })
        : buildEdgeFormPages({
            outputDirectory,
            plan,
            trust,
            publishedForms,
            readingGuide: readPublishedV1Guide(),
          });
    if (invocation.mode === "check" || invocation.mode === "source-check") {
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
    if (temporary && invocation.mode !== "build")
      rmSync(temporary, { recursive: true, force: true });
  }
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
