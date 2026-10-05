import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";

const FORM_PUBLICATION_NOTICE = `::: warning Form specification not formally published
This authored Form specification is not a formally published Form release. Host and provider support are separate and are not implied here.
:::`;
const FORM_PUBLICATION_NOTICE_JA = `::: warning Form仕様は未公開
このForm仕様は正式に公開されたFormリリースではありません。Host・providerの対応は別であり、この文書では断定しません。
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
export function writeEdgeV2Docs({ root, docsRoot, existingRoutes = [] }) {
  const entries = loadEdgeV2Docs(root);
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
          locale === "ja"
            ? FORM_PUBLICATION_NOTICE_JA
            : FORM_PUBLICATION_NOTICE,
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
