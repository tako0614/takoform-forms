import { readFileSync } from "node:fs";
import path from "node:path";
import { defineConfig } from "vitepress";

const buildRoot = process.env.EDGE_DOCS_BUILD_ROOT;
if (!buildRoot)
  throw new Error(
    "Build verified package pages with bun run build:edge-form-pages",
  );
const origin = "https://edge.forms.takoform.com";

export default defineConfig({
  lang: "en",
  title: "Edge Forms",
  description:
    "Settings, examples and package references for Takoform Edge Forms.",
  srcDir: path.join(buildRoot, "docs"),
  cacheDir: path.join(buildRoot, "cache"),
  tempDir: path.join(buildRoot, "temp"),
  // MiniSearch document IDs must not depend on concurrent rendering order.
  buildConcurrency: 1,
  cleanUrls: true,
  lastUpdated: false,
  vite: { build: { target: "esnext" } },
  head: [["link", { rel: "icon", href: "data:," }]],
  themeConfig: {
    search: { provider: "local" },
    outline: { level: [2, 3] },
    nav: [
      { text: "Forms", link: "/" },
      { text: "Takoform", link: "https://takoform.com/" },
    ],
    sidebar: JSON.parse(
      readFileSync(path.join(buildRoot, "sidebar.json"), "utf8"),
    ),
    socialLinks: [
      { icon: "github", link: "https://github.com/tako0614/takoform-forms" },
    ],
    notFound: {
      title: "Form page not found",
      quote: "Check the URL or browse the Form definitions.",
      linkText: "All Forms",
    },
  },
  transformHead({ pageData, title, description }) {
    const route = `/${pageData.relativePath.replace(/index\.md$/u, "").replace(/\.md$/u, "")}`;
    return [
      ["link", { rel: "canonical", href: `${origin}${route}` }],
      ["meta", { property: "og:url", content: `${origin}${route}` }],
      ["meta", { property: "og:title", content: title }],
      ["meta", { property: "og:description", content: description }],
    ];
  },
});
