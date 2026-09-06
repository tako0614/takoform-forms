import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

// Input is the verified package closure assembled by edge-form-pages.mjs.
// Only presentation lives here; this renderer does not resolve or trust packages.
export function renderVitePressPages({
  root,
  outputDirectory,
  forms,
  trust,
  isPublic,
  origin,
}) {
  const temporary = mkdtempSync(path.join(tmpdir(), "edge-vitepress-source-"));
  const source = path.join(temporary, "docs");
  const routes = ["/", ...forms.map(routeFor)];
  try {
    // Generated Vue modules resolve their dependencies from their source root.
    // Link only inside this build's fresh directory; no installed files change.
    symlinkSync(
      path.join(root, "node_modules"),
      path.join(temporary, "node_modules"),
      "dir",
    );
    mkdirSync(source);
    writeFileSync(path.join(source, "index.md"), renderIndex(forms, isPublic));
    for (const form of forms) {
      const directory = path.join(source, routeFor(form));
      mkdirSync(directory, { recursive: true });
      writeFileSync(
        path.join(directory, "index.md"),
        renderForm(form, trust, isPublic),
      );
    }
    const sidebar = [
      { text: "Overview", link: "/" },
      ...[false, true].map((retained) => ({
        text: retained ? "Retained versions" : "Current Forms",
        collapsed: retained,
        items: forms
          .filter((form) => !!form.retained === retained)
          .map((form) => ({
            text: `${form.formRef.kind} ${form.formRef.definitionVersion}`,
            link: routeFor(form),
          })),
      })),
    ];
    writeFileSync(
      path.join(temporary, "sidebar.json"),
      JSON.stringify(sidebar),
    );
    const result = spawnSync(
      path.join(root, "node_modules/.bin/vitepress"),
      [
        "build",
        path.join(root, "site"),
        "--outDir",
        path.resolve(outputDirectory),
      ],
      {
        cwd: root,
        encoding: "utf8",
        maxBuffer: 64 * 1024 * 1024,
        env: { ...process.env, EDGE_DOCS_BUILD_ROOT: temporary },
      },
    );
    if (result.status !== 0)
      throw new Error(
        `VitePress build failed:\n${result.stderr}\n${result.stdout}`,
      );
    cpSync(
      path.join(root, "site/icon.svg"),
      path.join(outputDirectory, "icon.svg"),
    );
    writeFileSync(
      path.join(outputDirectory, "robots.txt"),
      `User-agent: *\nAllow: /\nSitemap: ${origin}/sitemap.xml\n`,
    );
    writeFileSync(
      path.join(outputDirectory, "sitemap.xml"),
      `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${routes.map((route) => `<url><loc>${origin}${route}</loc></url>`).join("")}</urlset>\n`,
    );
    writeFileSync(
      path.join(outputDirectory, "_headers"),
      headersFor(outputDirectory, routes),
    );
    return routes;
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}

function routeFor(form) {
  return `/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`;
}

function formLink(form) {
  return `[${form.formRef.kind} ${form.formRef.definitionVersion}](${routeFor(form)})`;
}

// Package prose is data, never Markdown syntax or a Vue template. v-pre also
// protects literal {{ ... }} in descriptions. Newlines/pipes cannot create rows.
function literal(value) {
  const escaped = String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;")
    .replaceAll("|", "&#124;")
    .replaceAll("\n", " ")
    .replaceAll("`", "&#96;")
    .replaceAll("*", "&#42;")
    .replaceAll("_", "&#95;")
    .replaceAll("[", "&#91;")
    .replaceAll("]", "&#93;");
  return `<span v-pre>${escaped}</span>`;
}

function json(value) {
  // A longer fence cannot be terminated by data inside a JSON string.
  const text = JSON.stringify(value, null, 2);
  const longest = Math.max(
    2,
    ...(text.match(/`+/gu) ?? []).map((run) => run.length),
  );
  const fence = "`".repeat(longest + 1);
  return `${fence}json\n${text}\n${fence}`;
}

function status(isPublic) {
  return isPublic
    ? "Public package readback verified."
    : "Package contents were verified locally. This build does not assert public readability.";
}

function renderIndex(forms, isPublic) {
  const current = forms.filter((form) => !form.retained);
  return `---
title: Edge Forms
description: Settings, examples and package references for Takoform Edge Forms.
---

# Edge Forms

Form definitions for workers, storage, queues, workflows and actors. Each page lists the accepted settings, an example and the exact package reference.

## Worker applications

ModuleWorker → WorkerVersion → WorkerDeployment: create an application identity, describe a version of its code and configuration, then select the version that receives traffic.
WorkerBundle identifies the code artifact. WorkerEndpoint or WorkerCustomDomain describes how the deployed application is reached.

## Current Forms

${current.length} signed Edge Forms. ${status(isPublic)}

| Form | Purpose |
| --- | --- |
${current.map((form) => `| ${formLink(form)} | ${literal(form.guide.purpose)} |`).join("\n")}

## Using these definitions

Choose a Form, check whether your Host supports that exact version, and use its desired-state schema with your client. The examples here are JSON data, not OpenTofu configurations or complete Host API requests.

Host support and admission are separate from package publication. Reading a package does not create a resource or provide hosting.
See [Takoform's getting started guide](https://takoform.com/start/) for package verification and [implementation guides](https://takoform.com/guides/) for clients and Hosts.

## Retained versions

These older definitions remain available at their original URLs. They are not members of the current signed set.

${forms
  .filter((form) => form.retained)
  .map((form) => `- ${formLink(form)}`)
  .join("\n")}
`;
}

function renderForm(form, trust, isPublic) {
  const { definition, formRef } = form;
  const required = new Set(definition.desiredSchema.required ?? []);
  const properties = Object.entries(definition.desiredSchema.properties ?? {});
  const sourceUrl = `https://github.com/tako0614/takoform-forms/tree/${encodeURIComponent(form.locator.tag)}/${form.locator.sourcePath}`;
  const fields = properties
    .map(
      ([name, schema]) =>
        `### ${literal(name)}\n\n**${required.has(name) ? "Required" : "Optional"}** · ${literal(schema.type ?? (schema.oneOf ? "oneOf" : "schema"))}\n\n${literal(schema.description ?? "See the full schema below.")}\n\n${constraints(schema)}`,
    )
    .join("\n\n");
  return `---
title: ${JSON.stringify(`${definition.title} ${formRef.definitionVersion}`)}
description: ${JSON.stringify(`${formRef.kind} ${formRef.definitionVersion}: ${definition.description}`)}
---

# ${literal(definition.title)} ${formRef.definitionVersion}

${literal(form.guide?.purpose ?? `Historical ${definition.title} definition. See the version links below for the current definition.`)}

${form.retained ? `::: warning Retained version\nThis historical version is not part of the current signed set. ${isPublic ? "Public package bytes verified." : "Public readability is not asserted by this local build."}\n:::` : status(isPublic)}

Host support and admission are separate from package publication. Check your Host before using this version.

${form.versions.length ? `Other versions: ${form.versions.map(formLink).join(" · ")}` : ""}

${form.guide ? `## Usage\n\n${literal(form.guide.note)}\n\nRelated Forms: ${form.related.map(formLink).join(" · ")}` : ""}

::: details Full contract description
${literal(definition.description)}
:::

## Desired state {#fields-title}

${fields || "No configurable desired-state fields. Use the empty object shown below."}

## Example desired state {#example-title}

This is the exact ${literal(form.desiredPath)} from this package, not a complete Host API request or OpenTofu configuration. Replace example resource references and artifact digests with your own. This JSON does not provision anything by itself.

${json(form.example)}

[Read source fixture](${sourceUrl}/${form.desiredPath})

<div id="desired-schema">

::: details Full desired-state schema
Nested fields, alternatives and all schema constraints are listed below. The contract description also defines semantics checked by the Host.

${json(definition.desiredSchema)}
:::

</div>

## Lifecycle {#lifecycle-title}

Role: ${literal(definition.role)}

${(definition.lifecycleCapabilities ?? []).map((capability) => `- ${literal(capability)}`).join("\n") || "None."}

## Provided interfaces

${(definition.providedInterfaces ?? []).map((entry) => `- ${literal(`${entry.name}@${entry.version}`)} — ${literal(entry.schemaDigest)}`).join("\n") || "None."}

## Package reference {#locator-title}

| Item | Value |
| --- | --- |
| Kind | ${literal(formRef.kind)} |
| Package digest | ${literal(form.packageDigest)} |
| Schema digest | ${literal(formRef.schemaDigest)} |
| Tag | ${literal(form.locator.tag)} |
| Source path | ${literal(form.locator.sourcePath)} |
| ${form.retained ? `History | Retained package; not a current signed-set member` : `Signed set | ${literal(trust.setId)}`} |

[Open immutable package](${sourceUrl})

::: details Four-field FormRef
${json(formRef)}
:::
`;
}

function constraints(schema) {
  const keys = [
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
  ];
  return Object.entries(schema)
    .filter(([key]) => keys.includes(key))
    .map(([key, value]) => `- ${literal(`${key}: ${JSON.stringify(value)}`)}`)
    .join("\n");
}

function headersFor(outputDirectory, routes) {
  // VitePress has two small inline bootstrap scripts. Authorize exact generated
  // bytes, never arbitrary inline scripts. Styles include Shiki/runtime styles.
  const hashes = new Set();
  for (const file of [
    ...routes.map((route) => `${route}index.html`),
    "/404.html",
  ]) {
    const html = readFileSync(path.join(outputDirectory, file), "utf8");
    for (const [, attributes, script] of html.matchAll(
      /<script\b([^>]*)>([\s\S]*?)<\/script>/gu,
    )) {
      if (!/\bsrc=/u.test(attributes))
        hashes.add(
          `'sha256-${createHash("sha256").update(script).digest("base64")}'`,
        );
    }
  }
  return `/*\n  Cache-Control: public, max-age=0, must-revalidate, no-transform\n  X-Content-Type-Options: nosniff\n  Referrer-Policy: strict-origin-when-cross-origin\n  Content-Security-Policy: default-src 'none'; script-src 'self' ${[...hashes].sort().join(" ")}; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'\n`;
}
