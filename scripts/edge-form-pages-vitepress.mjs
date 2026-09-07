import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
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
  const routes = ["en", "ja"].flatMap((locale) => [
    locale === "ja" ? "/ja/" : "/",
    ...forms.map((form) => routeFor(form, locale)),
  ]);
  try {
    // Generated Vue modules resolve their dependencies from their source root.
    // Link only inside this build's fresh directory; no installed files change.
    symlinkSync(
      path.join(root, "node_modules"),
      path.join(temporary, "node_modules"),
      "dir",
    );
    mkdirSync(source);
    for (const locale of ["en", "ja"]) {
      const localeRoot = locale === "ja" ? path.join(source, "ja") : source;
      mkdirSync(localeRoot, { recursive: true });
      writeFileSync(
        path.join(localeRoot, "index.md"),
        renderIndex(forms, isPublic, locale),
      );
      for (const form of forms) {
        const directory = path.join(source, routeFor(form, locale));
        mkdirSync(directory, { recursive: true });
        writeFileSync(
          path.join(directory, "index.md"),
          renderForm(form, trust, isPublic, locale),
        );
      }
      const sidebar = [
        {
          text: locale === "ja" ? "概要" : "Overview",
          link: locale === "ja" ? "/ja/" : "/",
        },
        ...[false, true].map((retained) => ({
          text:
            locale === "ja"
              ? retained
                ? "過去のバージョン"
                : "現在のForms"
              : retained
                ? "Retained versions"
                : "Current Forms",
          collapsed: false,
          items: forms
            .filter((form) => !!form.retained === retained)
            .map((form) => ({
              text: `${form.formRef.kind} ${form.formRef.definitionVersion}`,
              link: routeFor(form, locale),
            })),
        })),
        {
          text: locale === "ja" ? "関連リンク" : "Related links",
          items: [
            {
              text: "Takoform",
              link:
                locale === "ja"
                  ? "https://takoform.com/"
                  : "https://takoform.com/en/",
            },
            {
              text: "GitHub",
              link: "https://github.com/tako0614/takoform-forms",
            },
          ],
        },
      ];
      writeFileSync(
        path.join(temporary, `sidebar-${locale}.json`),
        JSON.stringify(sidebar),
      );
    }
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

function routeFor(form, locale = "en") {
  return `${locale === "ja" ? "/ja" : ""}/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`;
}

function formLink(form, locale = "en") {
  return `[${form.formRef.kind} ${form.formRef.definitionVersion}](${routeFor(form, locale)})`;
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

function status(isPublic, locale = "en") {
  if (locale === "ja")
    return isPublic
      ? "公開パッケージの内容を取得して検証済みです。"
      : "パッケージの内容を手元で検証しています。このビルドでは公開URLから取得できることは確認していません。";
  return isPublic
    ? "Public package readback verified."
    : "Package contents were verified locally. This build does not assert public readability.";
}

function renderIndex(forms, isPublic, locale = "en") {
  const current = forms.filter((form) => !form.retained);
  if (locale === "ja")
    return `---
title: Edge Forms
description: Takoform Edge Formsの設定項目、利用例、パッケージ参照。
---

# Edge Forms

Worker、ストレージ、キュー、ワークフロー、ActorのためのForm定義です。各ページに設定項目、利用例、正確なパッケージ参照を掲載しています。

## Workerアプリケーション {#worker-applications}

ModuleWorker → WorkerVersion → WorkerDeploymentの順に、アプリケーションの識別子、コードと設定のバージョン、トラフィックの割り振りを定義します。
WorkerBundleはコードの成果物を特定します。WorkerEndpointとWorkerCustomDomainは、デプロイへの接続方法を定義します。

## 現在のForms {#current-forms}

署名付きのEdge Formsは${current.length}種類です。${status(isPublic, locale)}

| Form | 用途 |
| --- | --- |
${current.map((form) => `| ${formLink(form, locale)} | ${literal(form.guide.ja.purpose)} |`).join("\n")}

## 定義を使う {#using-these-definitions}

Formを選び、利用するHostがその正確なバージョンに対応しているかを確認し、設定のスキーマをクライアントで使います。掲載している例はJSONデータであり、OpenTofuの設定や完全なHost APIリクエストではありません。

Hostの対応や受け入れ判断は、パッケージの公開とは別です。パッケージを読むだけでリソースが作成されたり、ホスティングが提供されたりすることはありません。
検証方法は[Takoformの導入ガイド](https://takoform.com/start/)、クライアントやHostの実装は[実装ガイド](https://takoform.com/guides/)を参照してください。

## 過去のバージョン {#retained-versions}

以前の定義も元のURLで取得できます。現在の署名セットには含まれません。

${forms
  .filter((form) => form.retained)
  .map((form) => `- ${formLink(form, locale)}`)
  .join("\n")}
`;
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
See [Takoform's getting started guide](https://takoform.com/en/start/) for package verification and [implementation guides](https://takoform.com/en/guides/) for clients and Hosts.

## Retained versions

These older definitions remain available at their original URLs. They are not members of the current signed set.

${forms
  .filter((form) => form.retained)
  .map((form) => `- ${formLink(form)}`)
  .join("\n")}
`;
}

function renderForm(form, trust, isPublic, locale = "en") {
  const { definition, formRef } = form;
  const t = (en, ja) => (locale === "ja" ? ja : en);
  const link = (entry) => formLink(entry, locale);
  const guide = locale === "ja" ? form.guide?.ja : form.guide;
  const required = new Set(definition.desiredSchema.required ?? []);
  const properties = Object.entries(definition.desiredSchema.properties ?? {});
  const sourceUrl = `https://github.com/tako0614/takoform-forms/tree/${encodeURIComponent(form.locator.tag)}/${form.locator.sourcePath}`;
  const fields = properties
    .map(
      ([name, schema]) =>
        `### ${literal(name)}\n\n**${required.has(name) ? t("Required", "必須") : t("Optional", "任意")}** · ${literal(schema.type ?? (schema.oneOf ? "oneOf" : "schema"))}\n\n<span lang="en">${literal(schema.description ?? "See the full schema below.")}</span>\n\n${constraints(schema)}`,
    )
    .join("\n\n");
  return `---
title: ${JSON.stringify(`${definition.title} ${formRef.definitionVersion}`)}
description: ${JSON.stringify(`${formRef.kind} ${formRef.definitionVersion}: ${guide?.purpose ?? t(definition.description, "以前のForm定義。設定とパッケージ参照を掲載しています。")}`)}
---

# ${literal(definition.title)} ${formRef.definitionVersion}

${literal(guide?.purpose ?? t(`Historical ${definition.title} definition. See the version links below for the current definition.`, `${definition.title}の過去の定義です。現在の定義は下のバージョン一覧から確認できます。`))}

${form.retained ? `::: warning ${t("Retained version", "過去のバージョン")}\n${t("This historical version is not part of the current signed set.", "この過去のバージョンは、現在の署名セットに含まれません。")} ${status(isPublic, locale)}\n:::` : status(isPublic, locale)}

${t("Host support and admission are separate from package publication. Check your Host before using this version.", "Hostの対応や受け入れ判断は、パッケージの公開とは別です。このバージョンを使う前に、利用先のHostで確認してください。")}

${form.versions.length ? `${t("Other versions", "他のバージョン")}: ${form.versions.map(link).join(" · ")}` : ""}

${guide ? `## ${t("Usage", "使い方")} {#usage}\n\n${literal(guide.note)}\n\n${t("Related Forms", "関連するForms")}: ${form.related.map(link).join(" · ")}` : ""}

::: details ${t("Full contract description", "定義の説明（英語原文）")}
<span lang="en">${literal(definition.description)}</span>
:::

## ${t("Desired state", "設定項目")} {#fields-title}

${locale === "ja" ? "項目名とスキーマ内の説明は、パッケージに含まれる英語の原文です。日本語の解説は定義に要件を追加するものではありません。\n" : ""}
${fields || t("No configurable desired-state fields. Use the empty object shown below.", "設定項目はありません。下の例の空のオブジェクトを使います。")}

## ${t("Example desired state", "設定の例")} {#example-title}

${t(`This is the exact ${literal(form.desiredPath)} from this package, not a complete Host API request or OpenTofu configuration. Replace example resource references and artifact digests with your own. This JSON does not provision anything by itself.`, `パッケージに含まれる${literal(form.desiredPath)}をそのまま掲載しています。完全なHost APIリクエストやOpenTofuの設定ではありません。例のリソース参照とArtifactのダイジェストは、実際の値に置き換えてください。このJSONだけでリソースが作成されることはありません。`)}

${json(form.example)}

[${t("Read source fixture", "例の原文を見る")}](${sourceUrl}/${form.desiredPath})

<div id="desired-schema">

::: details ${t("Full desired-state schema", "設定のスキーマ全文（英語原文）")}
${t("Nested fields, alternatives and all schema constraints are listed below. The contract description also defines semantics checked by the Host.", "入れ子の項目、選択肢、すべてのスキーマ制約を以下に掲載しています。Hostが検証する意味上の規則は、定義の説明にも記載されています。")}

${json(definition.desiredSchema)}
:::

</div>

## ${t("Lifecycle", "ライフサイクル")} {#lifecycle-title}

${t("Role", "役割")}: ${literal(definition.role)}

${(definition.lifecycleCapabilities ?? []).map((capability) => `- ${literal(capability)}`).join("\n") || t("None.", "ありません。")}

## ${t("Provided interfaces", "提供するInterface")} {#provided-interfaces}

${(definition.providedInterfaces ?? []).map((entry) => `- ${literal(`${entry.name}@${entry.version}`)} — ${literal(entry.schemaDigest)}`).join("\n") || t("None.", "ありません。")}

## ${t("Package reference", "パッケージ参照")} {#locator-title}

| ${t("Item", "項目")} | ${t("Value", "値")} |
| --- | --- |
| Kind | ${literal(formRef.kind)} |
| Package digest | ${literal(form.packageDigest)} |
| Schema digest | ${literal(formRef.schemaDigest)} |
| Tag | ${literal(form.locator.tag)} |
| Source path | ${literal(form.locator.sourcePath)} |
| ${form.retained ? `${t("History", "履歴")} | ${t("Retained package; not a current signed-set member", "過去のパッケージ。現在の署名セットには含まれません。")}` : `${t("Signed set", "署名セット")} | ${literal(trust.setId)}`} |

[${t("Open immutable package", "変更不可のパッケージを開く")}](${sourceUrl})

::: details ${t("Four-field FormRef", "4項目のFormRef")}
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
