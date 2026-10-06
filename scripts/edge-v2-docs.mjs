import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";

export const EDGE_V2_PUBLICATION_ASSET = "_edge-forms-publication.json";
const formRoute = /^\/forms\/([A-Za-z][A-Za-z0-9]*)\/(\d+\.\d+\.\d+)\/$/u;
const digest = /^sha256:[a-f0-9]{64}$/u;

const FORM_PUBLICATION_NOTICE = `::: warning Form specification not formally published
This authored Form specification is not a formally published Form release. Host and provider support are separate and are not implied here.
:::`;
const FORM_PUBLICATION_NOTICE_JA = `::: warning Form仕様は未公開
このForm仕様は正式に公開されたFormリリースではありません。Host・providerの対応は別であり、この文書では断定しません。
:::`;
const FORM_PUBLISHED_NOTICE = `::: info Version-fixed Form specification
This version-fixed URL serves the Japanese normative source. <a href="__SOURCE_URL__">Read the exact source</a>. Publication does not imply Host or provider support.
:::`;
const FORM_PUBLISHED_NOTICE_JA = `::: info 版が固定されたForm仕様
この版で固定されたURLの規範本文は日本語です。<a href="__SOURCE_URL__">原文を読む</a>。公開はHostやproviderの対応を意味しません。
:::`;

const FORM_SOURCE = /^spec\/forms\/([^/]+)\/([^/]+)\/index\.md$/u;
const GUIDE_SOURCE = /^spec\/guides\/([^/]+)\.md$/u;

/** Read the authored, Japanese-source v2 documents without deriving package or Host availability. */
export function loadEdgeV2Docs(root) {
  const files = [
    ...walkMarkdown(path.join(root, "spec/forms"), "spec/forms"),
    ...walkMarkdown(path.join(root, "spec/guides"), "spec/guides"),
  ].sort();
  const entries = [];
  for (const file of files) {
    const formMatch = FORM_SOURCE.exec(file);
    const guideMatch = GUIDE_SOURCE.exec(file);
    if (!formMatch && !guideMatch)
      throw new Error(`unsupported authored v2 documentation path: ${file}`);
    const source = readFileSync(path.join(root, file), "utf8");
    if (!source.trim())
      throw new Error(`authored documentation is empty: ${file}`);
    const isForm = !!formMatch;
    if (
      formMatch &&
      (!/^[A-Za-z][A-Za-z0-9]*$/u.test(formMatch[1]) ||
        !/^\d+\.\d+\.\d+$/u.test(formMatch[2]))
    )
      throw new Error(`invalid authored v2 Form route identity: ${file}`);
    if (
      guideMatch &&
      guideMatch[1] !== "index" &&
      !/^[a-z0-9]+(?:-[a-z0-9]+)*$/u.test(guideMatch[1])
    )
      throw new Error(`invalid authored v2 guide slug: ${file}`);
    const route = isForm
      ? `/forms/${formMatch[1]}/${formMatch[2]}/`
      : guideMatch[1] === "index"
        ? "/v2/"
        : guideMatch[1] === "home"
          ? "/"
          : `/v2/${guideMatch[1]}/`;
    validateAuthoredDocument({ source, route, isForm, file });
    entries.push({ file, route, source, kind: isForm ? "form" : "guide" });
  }
  entries.sort((left, right) => {
    if (left.kind !== right.kind) return left.kind === "guide" ? -1 : 1;
    return left.route.localeCompare(right.route);
  });
  assertUniqueRoutes(entries);
  return entries;
}

/** Materialize both locale routes. Japanese remains the canonical authored body. */
export function writeEdgeV2Docs({
  root,
  docsRoot,
  existingRoutes = [],
  selectedForms,
  sourcePreview = true,
}) {
  const all = loadEdgeV2Docs(root);
  const selected =
    selectedForms === undefined
      ? null
      : validateEdgeV2PublicationEntries(selectedForms);
  const selectedUrls = new Set((selected ?? []).map((entry) => entry.url));
  const entries = sourcePreview
    ? all
    : selectedUrls.size
      ? all.filter(
          (entry) =>
            entry.kind === "guide" ||
            selectedUrls.has(`https://edge.forms.takoform.com${entry.route}`),
        )
      : [];
  if (!sourcePreview) {
    for (const entry of selected ?? []) {
      if (!entries.some((candidate) => candidate.file === entry.path))
        throw new Error(`published Form source is not authored: ${entry.path}`);
    }
    for (const entry of entries) {
      for (const match of entry.source.matchAll(
        /\]\((\/forms\/[A-Za-z][A-Za-z0-9]*\/\d+\.\d+\.\d+\/)(?:#[^)]*)?\)/gu,
      )) {
        if (!selectedUrls.has(`https://edge.forms.takoform.com${match[1]}`))
          throw new Error(
            `published document links to an unpublished Form: ${entry.file} -> ${match[1]}`,
          );
      }
    }
  }
  const occupied = new Set(existingRoutes.map(normalizeRoute));
  const routes = [];
  for (const entry of entries) {
    const sourceRoute = entry.route;
    for (const locale of ["en", "ja"]) {
      const route = `${locale === "ja" ? "/ja" : ""}${sourceRoute}`;
      const normalized = normalizeRoute(route);
      const replacesLegacyHome =
        entry.file === "spec/guides/home.md" && ["/", "/ja/"].includes(route);
      if (occupied.has(normalized) && !replacesLegacyHome)
        throw new Error(`authored v2 documentation route collision: ${route}`);
      occupied.add(normalized);
      const relative = route.replace(/^\//u, "");
      const directory = path.join(docsRoot, relative);
      mkdirSync(directory, { recursive: true });
      const notices = [];
      if (locale === "en")
        notices.push(`::: info Japanese source
The original specification and guide are authored in Japanese. The Japanese route contains the same source text; this English route is a reading aid, not a separate translation or an implementation-availability claim.
:::`);
      if (entry.kind === "form")
        notices.push(
          sourcePreview
            ? locale === "ja"
              ? FORM_PUBLICATION_NOTICE_JA
              : FORM_PUBLICATION_NOTICE
            : (locale === "ja"
                ? FORM_PUBLISHED_NOTICE_JA
                : FORM_PUBLISHED_NOTICE
              ).replace("__SOURCE_URL__", `${entry.route}source.md`),
        );
      const localizedSource =
        locale === "ja"
          ? localizeInternalMarkdownLinks(entry.source)
          : entry.source;
      writeFileSync(
        path.join(directory, "index.md"),
        withNotices(localizedSource, notices),
      );
      routes.push(route);
    }
  }
  return { entries, routes };
}

/** Publisher-local deployment inventory, not a Takoform API or Form identity. */
export function validateEdgeV2PublicationEntries(entries) {
  if (!Array.isArray(entries) || entries.length > 256)
    throw new Error("invalid Edge Form publication inventory size");
  const seenUrls = new Set();
  const seenPaths = new Set();
  for (const entry of entries) {
    if (
      !entry ||
      Object.keys(entry).sort().join() !== "path,sha256,url" ||
      typeof entry.url !== "string" ||
      typeof entry.path !== "string" ||
      typeof entry.sha256 !== "string" ||
      !digest.test(entry.sha256)
    )
      throw new Error("invalid Edge Form publication inventory entry");
    const url = new URL(entry.url);
    const match = formRoute.exec(url.pathname);
    if (
      url.origin !== "https://edge.forms.takoform.com" ||
      !match ||
      entry.url !== `https://edge.forms.takoform.com${url.pathname}` ||
      entry.path !== `spec/forms/${match[1]}/${match[2]}/index.md` ||
      seenUrls.has(entry.url) ||
      seenPaths.has(entry.path)
    )
      throw new Error("invalid or duplicate Edge Form publication identity");
    seenUrls.add(entry.url);
    seenPaths.add(entry.path);
  }
  return entries;
}

export function edgeV2PublicationBytes(entries) {
  return `${JSON.stringify({ kind: "edge.forms.site-publication", forms: validateEdgeV2PublicationEntries(entries) })}\n`;
}

export function parseEdgeV2PublicationBytes(bytes) {
  if (!bytes || bytes.length > 131072)
    throw new Error("invalid Edge Form public inventory size");
  let value;
  try {
    value = JSON.parse(bytes.toString("utf8"));
  } catch {
    throw new Error("invalid Edge Form public inventory JSON");
  }
  if (
    !value ||
    Object.keys(value).sort().join() !== "forms,kind" ||
    value.kind !== "edge.forms.site-publication"
  )
    throw new Error("invalid Edge Form public inventory kind");
  return validateEdgeV2PublicationEntries(value.forms);
}

export function writeEdgeV2PublicationAssets({
  root,
  outputDirectory,
  entries,
}) {
  for (const entry of validateEdgeV2PublicationEntries(entries)) {
    const source = readFileSync(path.join(root, entry.path));
    if (
      `sha256:${createHash("sha256").update(source).digest("hex")}` !==
      entry.sha256
    )
      throw new Error(`frozen Form source bytes changed: ${entry.path}`);
    const route = new URL(entry.url).pathname;
    const destination = path.join(outputDirectory, route.slice(1), "source.md");
    mkdirSync(path.dirname(destination), { recursive: true });
    writeFileSync(destination, source);
  }
  writeFileSync(
    path.join(outputDirectory, EDGE_V2_PUBLICATION_ASSET),
    edgeV2PublicationBytes(entries),
  );
}

export function edgeV2SidebarItems(entries, locale = "en") {
  const ja = locale === "ja";
  const routeFor = (entry) => `${ja ? "/ja" : ""}${entry.route}`;
  return entries.map((entry) => ({
    text: documentTitle(entry.source, entry),
    link: routeFor(entry),
  }));
}

function documentTitle(source, entry) {
  return parseFrontmatter(source).title;
}

function validateAuthoredDocument({ source, route, isForm, file }) {
  const frontmatter = parseFrontmatter(source);
  if (!frontmatter.title?.trim() || !frontmatter.description?.trim())
    throw new Error(
      `authored documentation requires title and description: ${file}`,
    );
  const body = source.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n/u, "");
  if ([...body.matchAll(/^#\s+.+$/gmu)].length !== 1)
    throw new Error(`authored documentation requires exactly one H1: ${file}`);
  if (isForm) {
    const expectedUrl = `https://edge.forms.takoform.com${route}`;
    if (frontmatter.formUrl !== expectedUrl)
      throw new Error(
        `Form formUrl must match canonical route ${expectedUrl}: ${file}`,
      );
    if (frontmatter.hostApi !== "forms.takoform.com/v2")
      throw new Error(`Form hostApi must be forms.takoform.com/v2: ${file}`);
  }
  for (const match of body.matchAll(
    /^(`{3,}|~{3,})json[^\n]*\n([\s\S]*?)\n\1\s*$/gmu,
  )) {
    try {
      JSON.parse(match[2]);
    } catch (error) {
      throw new Error(`invalid JSON example in ${file}: ${error.message}`);
    }
  }
}

function parseFrontmatter(source) {
  const match = source.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n/u);
  if (!match) return {};
  return Object.fromEntries(
    [...match[1].matchAll(/^([A-Za-z][A-Za-z0-9_-]*):\s*(.*?)\s*$/gmu)].map(
      ([, key, value]) => [
        key,
        value.replace(
          /^(?:"([^"]*)"|'([^']*)')$/u,
          (_, double, single) => double ?? single,
        ),
      ],
    ),
  );
}

function walkMarkdown(absolute, relative) {
  let entries;
  try {
    entries = readdirSync(absolute, { withFileTypes: true });
  } catch (error) {
    if (error.code === "ENOENT") return [];
    throw error;
  }
  return entries.flatMap((entry) => {
    const childRelative = `${relative}/${entry.name}`;
    const childAbsolute = path.join(absolute, entry.name);
    if (entry.isSymbolicLink())
      throw new Error(
        `authored documentation cannot be a symlink: ${childRelative}`,
      );
    if (entry.isDirectory()) return walkMarkdown(childAbsolute, childRelative);
    if (!entry.isFile() || !entry.name.endsWith(".md")) return [];
    return [childRelative];
  });
}

function assertUniqueRoutes(entries) {
  const seen = new Set();
  for (const entry of entries) {
    const route = normalizeRoute(entry.route);
    if (seen.has(route))
      throw new Error(`duplicate authored v2 route: ${entry.route}`);
    seen.add(route);
  }
}

function normalizeRoute(route) {
  return route.endsWith("/") ? route : `${route}/`;
}

function withNotices(source, notices) {
  const frontmatter = source.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n/u)?.[0];
  if (!frontmatter)
    throw new Error("authored documentation requires YAML frontmatter");
  const body = source.slice(frontmatter.length).replace(/^\s+/u, "");
  const metadata = "edgeSourceLanguage: ja\n";
  const amendedFrontmatter = frontmatter.replace(
    /---\r?\n$/u,
    `${metadata}---\n`,
  );
  return `${amendedFrontmatter}${notices.length ? `${notices.join("\n\n")}\n\n` : ""}<!-- edge-source-ja:start -->\n\n${body}\n\n<!-- edge-source-ja:end -->\n`;
}

function localizeInternalMarkdownLinks(source) {
  const frontmatter = source.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n/u)?.[0];
  if (!frontmatter) return source;
  const body = source.slice(frontmatter.length);
  let inFence = false;
  let fenceMarker = "";
  const localizedBody = body
    .split(/(?<=\n)/u)
    .map((line) => {
      const fence = line.match(/^\s*(`{3,}|~{3,})/u)?.[1];
      if (fence) {
        if (!inFence) {
          inFence = true;
          fenceMarker = fence[0];
        } else if (fence[0] === fenceMarker) {
          inFence = false;
          fenceMarker = "";
        }
        return line;
      }
      if (inFence) return line;
      return line.replace(/(\]\()\/(forms|v2|v1)(?=\/|\))/gu, "$1/ja/$2");
    })
    .join("");
  return `${frontmatter}${localizedBody}`;
}
