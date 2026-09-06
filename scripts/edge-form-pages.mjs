#!/usr/bin/env bun

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  cpSync,
  existsSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

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
  const fontsDirectory = path.join(outputDirectory, "fonts");
  mkdirSync(fontsDirectory, { recursive: true });
  for (const [source, destination] of [
    [
      "node_modules/@fontsource/space-grotesk/files/space-grotesk-latin-600-normal.woff2",
      "space-grotesk-latin-600-normal.woff2",
    ],
    [
      "node_modules/@fontsource/ibm-plex-sans/files/ibm-plex-sans-latin-400-normal.woff2",
      "ibm-plex-sans-latin-400-normal.woff2",
    ],
    [
      "node_modules/@fontsource/space-grotesk/LICENSE",
      "SPACE-GROTESK-LICENSE.txt",
    ],
    [
      "node_modules/@fontsource/ibm-plex-sans/LICENSE",
      "IBM-PLEX-SANS-LICENSE.txt",
    ],
  ]) {
    cpSync(path.join(root, source), path.join(fontsDirectory, destination));
  }
  cpSync(
    path.join(root, "site", "tokens.css"),
    path.join(outputDirectory, "tokens.css"),
  );
  cpSync(
    path.join(root, "site", "site.css"),
    path.join(outputDirectory, "site.css"),
  );

  const routes = ["/"];
  writeFileSync(
    path.join(outputDirectory, "index.html"),
    renderIndex(forms, trust, isPublic),
    { mode: 0o644 },
  );
  for (const form of forms) {
    const route = `/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`;
    const destination = path.join(
      outputDirectory,
      "forms",
      form.formRef.kind,
      form.formRef.definitionVersion,
    );
    mkdirSync(destination, { recursive: true });
    writeFileSync(
      path.join(destination, "index.html"),
      renderForm(form, trust, isPublic),
      {
        mode: 0o644,
      },
    );
    routes.push(route);
  }
  writeFileSync(
    path.join(outputDirectory, "404.html"),
    document({
      title: "Form page not found — Edge Forms",
      canonical: `${EDGE_FORM_PAGES_ORIGIN}/404`,
      description:
        "This Form page does not exist. Browse the current Edge Form contracts.",
      body: '<main class="page-shell form-page" id="content"><h1>Form page not found</h1><p>This address does not identify a page in the current publisher index.</p><a class="source-link" href="/">Browse all Forms</a></main>',
    }),
  );
  writeFileSync(
    path.join(outputDirectory, "robots.txt"),
    `User-agent: *\nAllow: /\nSitemap: ${EDGE_FORM_PAGES_ORIGIN}/sitemap.xml\n`,
  );
  writeFileSync(
    path.join(outputDirectory, "sitemap.xml"),
    `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${routes.map((route) => `<url><loc>${EDGE_FORM_PAGES_ORIGIN}${route}</loc></url>`).join("")}</urlset>\n`,
  );
  writeFileSync(
    path.join(outputDirectory, "_headers"),
    "/*\n  X-Content-Type-Options: nosniff\n  Referrer-Policy: strict-origin-when-cross-origin\n  Content-Security-Policy: default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'\n",
  );
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

function renderIndex(forms, trust, isPublic) {
  const cards = forms
    .filter((form) => !form.retained)
    .map(
      (form) => `<li class="catalogue__item">
  <a class="form-link" href="/forms/${h(form.formRef.kind)}/${h(form.formRef.definitionVersion)}/">
    <span class="form-link__kind">${h(form.formRef.kind)}</span>
    <span class="form-link__title">${h(form.guide.purpose)}</span>
    <span class="form-link__version">${h(form.formRef.definitionVersion)}</span>
  </a>
</li>`,
    )
    .join("\n");
  return document({
    title: "Edge Forms — Takoform",
    canonical: `${EDGE_FORM_PAGES_ORIGIN}/`,
    body: `<header class="masthead">
  <a class="wordmark" href="/">Takoform / Edge Forms</a>
  <span class="masthead__set">set ${h(shortSet(trust.setId))}</span>
</header>
<main class="page-shell" id="content">
  <section class="catalogue-head">
    <p class="catalogue-head__count">${forms.filter((form) => !form.retained).length} signed Edge Forms</p>
    <h1>Choose a resource contract</h1>
    <p>Worker applications, storage, queues, workflows and actors. Pick a Form to read what it means, which fields it accepts, and an exact package example.</p>
    <p>For a Worker application, follow <strong>ModuleWorker → WorkerVersion → WorkerDeployment</strong>: identity, immutable code and configuration, then traffic. WorkerBundle identifies the committed code artifact; WorkerEndpoint or WorkerCustomDomain makes the active deployment reachable.</p>
    <p>These are contracts, not a hosting service. <strong>Host support and admission are separate</strong> from publishing or reading a package. <a href="https://takoform.com/start/">Start with the Takoform model</a> or <a href="https://takoform.com/guides/">choose an integration guide</a>.</p>
    ${statusNote(isPublic)}
  </section>
  <ol class="catalogue">${cards}</ol>
  <section class="form-section history"><h2>Retained versions</h2><p>Historical contracts stay available at their original versioned addresses. They are not members of the current signed set.</p><ul>${forms
    .filter((form) => form.retained)
    .map(
      (form) =>
        `<li><a href="/forms/${h(form.formRef.kind)}/${h(form.formRef.definitionVersion)}/">${h(form.formRef.kind)} ${h(form.formRef.definitionVersion)}</a></li>`,
    )
    .join("")}</ul></section>
</main>
${footer()}`,
  });
}

function renderForm(form, trust, isPublic) {
  const definition = form.definition;
  const required = new Set(definition.desiredSchema?.required ?? []);
  const capabilities = list(definition.lifecycleCapabilities);
  const interfaces = Array.isArray(definition.providedInterfaces)
    ? definition.providedInterfaces
        .map(
          (entry) =>
            `<li><code>${h(entry.name)}@${h(entry.version)}</code><span>${h(entry.schemaDigest)}</span></li>`,
        )
        .join("")
    : "";
  const properties = Object.entries(definition.desiredSchema?.properties ?? {})
    .map(
      ([name, schema]) =>
        `<div class="spec-row"><dt><code>${h(name)}</code><span class="field-meta">${required.has(name) ? "Required" : "Optional"} · ${h(schema.type ?? (schema.oneOf ? "oneOf" : "schema"))}</span></dt><dd>${h(schema.description ?? "See the exact schema below.")}${constraints(schema)}</dd></div>`,
    )
    .join("");
  const desiredState = properties
    ? `<section class="form-section">
    <h2 id="fields-title">Desired state</h2>
    <dl class="spec-table">${properties}</dl>
  </section>`
    : '<section class="form-section"><h2 id="fields-title">Desired state</h2><p>No configurable desired-state fields. Use the empty object shown below.</p></section>';
  const sourceUrl = `https://github.com/tako0614/takoform-forms/tree/${encodeURIComponent(form.locator.tag)}/${form.locator.sourcePath}`;
  return document({
    title: `${definition.title} ${form.formRef.definitionVersion} — Edge Forms`,
    description: `${form.formRef.kind} ${form.formRef.definitionVersion}: ${definition.description}`,
    canonical: `${EDGE_FORM_PAGES_ORIGIN}/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`,
    body: `<header class="masthead">
  <a class="wordmark" href="/">Takoform / Edge Forms</a>
  <a class="masthead__back" href="/">All Forms</a>
</header>
<main class="page-shell form-page" id="content">
  <header class="form-head">
    <p class="form-head__identity"><code>${h(form.formRef.apiVersion)}/${h(form.formRef.kind)}@${h(form.formRef.definitionVersion)}</code></p>
    <h1>${h(definition.title)}</h1>
    <p class="form-head__description">${h(form.guide?.purpose ?? "Read the original contract for this retained version.")}</p>
    ${form.retained ? `<p class="status-note">Retained historical version. Not part of the current signed set.${isPublic ? " Public package bytes verified." : " Public readability is not asserted by this local build."}</p>` : statusNote(isPublic)}
    <p>Host support and admission are separate from package publication. These fields describe portable desired state, not a provider-specific deployment command.</p>
    <nav class="page-nav" aria-label="On this page"><a href="#fields-title">Fields</a><a href="#example-title">Example &amp; schema</a><a href="#lifecycle-title">Lifecycle &amp; interfaces</a><a href="#locator-title">Package identity</a></nav>
  </header>
  ${form.guide ? `<section class="form-section"><h2>How it fits</h2><p>${h(form.guide.note)}</p><p>Works with: ${form.related.map((entry) => `<a href="/forms/${h(entry.formRef.kind)}/${h(entry.formRef.definitionVersion)}/">${h(entry.definition.title)}</a>`).join(" · ")}</p></section>` : ""}
  <details class="contract-description"><summary>Full contract description</summary><p>${h(definition.description)}</p></details>
  ${form.versions.length ? `<p>Other versions: ${form.versions.map((entry) => `<a href="/forms/${h(entry.formRef.kind)}/${h(entry.formRef.definitionVersion)}/">${h(entry.formRef.definitionVersion)} (${entry.retained ? "retained" : "current"})</a>`).join(" · ")}</p>` : ""}
  <dl class="detail-grid" aria-label="Form identity">
    ${datum("Kind", form.formRef.kind)}
    ${datum("Definition version", form.formRef.definitionVersion)}
    ${datum("Role", definition.role)}
  </dl>
  ${desiredState}
  <section class="form-section" aria-labelledby="example-title">
    <h2 id="example-title">Example desired state</h2>
    <p>This is the exact <code>${h(form.desiredPath)}</code> from this package, not a complete Host API request or OpenTofu configuration. Replace example resource references and artifact digests with your own; it does not provision anything by itself.</p>
    <pre tabindex="0" aria-label="Example desired-state JSON"><code>${h(JSON.stringify(form.example, null, 2))}</code></pre>
    <a class="source-link" href="${h(sourceUrl)}/${h(form.desiredPath)}">Read source fixture</a>
    <details id="desired-schema"><summary>Full desired-state schema</summary><p>Nested fields, allowed alternatives and all schema constraints. Host-validated semantics also apply; read the contract description above.</p><pre tabindex="0" aria-label="Complete desired-state schema"><code>${h(JSON.stringify(definition.desiredSchema, null, 2))}</code></pre></details>
  </section>
  <section class="form-section split-section">
    <div><h2 id="lifecycle-title">Lifecycle</h2><ul class="plain-list">${capabilities}</ul></div>
    <div><h2>Provided interfaces</h2><ul class="interface-list">${interfaces || "<li>None</li>"}</ul></div>
  </section>
  <section class="locator" aria-labelledby="locator-title">
    <h2 id="locator-title">Canonical package locator</h2>
    <dl>
      ${datum("Package digest", form.packageDigest)}
      ${datum("Schema digest", form.formRef.schemaDigest)}
      ${datum("Tag", form.locator.tag)}
      ${datum("Source path", form.locator.sourcePath)}
      ${form.retained ? datum("History", "Retained package; not a current signed-set member") : datum("Signed set", trust.setId)}
    </dl>
    <a class="source-link" href="${h(sourceUrl)}">Open immutable package</a>
    <details><summary>Four-field FormRef</summary><pre tabindex="0" aria-label="Exact FormRef"><code>${h(JSON.stringify(form.formRef, null, 2))}</code></pre></details>
  </section>
</main>
${footer()}`,
  });
}

function constraints(schema) {
  const values = Object.entries(schema).filter(([key]) =>
    [
      "default",
      "enum",
      "const",
      "minimum",
      "maximum",
      "minLength",
      "maxLength",
      "pattern",
      "format",
      "minItems",
      "maxItems",
      "uniqueItems",
      "additionalProperties",
    ].includes(key),
  );
  return values.length
    ? `<ul class="field-constraints">${values.map(([key, value]) => `<li><code>${h(key)}: ${h(JSON.stringify(value))}</code></li>`).join("")}</ul>`
    : "";
}

function document({
  title,
  canonical,
  body,
  description = "Choose an Edge Form contract: fields, constraints, exact package examples, and canonical source identities.",
}) {
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>${h(title)}</title>
  <meta name="description" content="${h(description)}">
  <meta property="og:title" content="${h(title)}">
  <meta property="og:description" content="${h(description)}">
  <meta property="og:url" content="${h(canonical)}">
  <link rel="canonical" href="${h(canonical)}">
  <link rel="stylesheet" href="/tokens.css">
  <link rel="stylesheet" href="/site.css">
</head>
<body><a class="skip-link" href="#content">Skip to content</a>${body}</body>
</html>
`;
}

function statusNote(isPublic) {
  return isPublic
    ? '<p class="status-note status-note--verified"><span aria-hidden="true">✓</span> Public package readback verified</p>'
    : '<p class="status-note">Signed package closure. This build does not assert public readability.</p>';
}

function datum(term, value) {
  return `<div class="datum"><dt>${h(term)}</dt><dd><code>${h(value ?? "")}</code></dd></div>`;
}

function list(values) {
  return Array.isArray(values)
    ? values.map((value) => `<li><code>${h(value)}</code></li>`).join("")
    : "";
}

function footer() {
  return '<footer class="footer"><span>Takoform publisher</span><span>Human pages only — package locators remain authoritative.</span></footer>';
}

function shortSet(setId) {
  return `${setId.slice(0, 10)}…${setId.slice(-6)}`;
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

function h(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
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
