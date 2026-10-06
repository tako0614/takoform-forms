import { readFileSync } from "node:fs";
import path from "node:path";
import { defineConfig } from "vitepress";
import { renderForSearch } from "./search.mjs";

const buildRoot = process.env.EDGE_DOCS_BUILD_ROOT;
if (!buildRoot)
  throw new Error(
    "Build verified package pages with bun run build:edge-form-pages",
  );
const origin = "https://edge.forms.takoform.com";
const sourcePreview = process.env.EDGE_FORM_SOURCE_PREVIEW === "1";

export default defineConfig({
  lang: "en",
  title: "Edge Forms",
  description:
    "Settings, examples and package references for Takoform Edge Forms.",
  locales: {
    root: { label: "English", lang: "en" },
    ja: {
      label: "日本語",
      lang: "ja-JP",
      description: "Takoform Edge Formsの設定項目、利用例、パッケージ参照。",
      themeConfig: {
        nav: JSON.parse(
          readFileSync(path.join(buildRoot, "nav-ja.json"), "utf8"),
        ),
        sidebar: JSON.parse(
          readFileSync(path.join(buildRoot, "sidebar-ja.json"), "utf8"),
        ),
        docFooter: { prev: "前へ", next: "次へ" },
        darkModeSwitchLabel: "配色",
        returnToTopLabel: "先頭へ",
        sidebarMenuLabel: "目次",
        outlineTitle: "このページ",
        langMenuLabel: "言語を切り替える",
        notFound: {
          title: "ページがありません",
          quote: "URLを確認するか、目次からFormを探してください。",
          linkText: "Forms一覧",
        },
      },
    },
  },
  srcDir: path.join(buildRoot, "docs"),
  cacheDir: path.join(buildRoot, "cache"),
  // MiniSearch document IDs must not depend on concurrent rendering order.
  buildConcurrency: 1,
  cleanUrls: true,
  lastUpdated: false,
  markdown: {
    config(md) {
      md.core.ruler.after("block", "edge-authored-japanese-source", (state) => {
        if (state.env.frontmatter?.edgeSourceLanguage !== "ja") return;
        for (const token of state.tokens) {
          if (token.type !== "html_block") continue;
          if (token.content.includes("<!-- edge-source-ja:start -->"))
            token.content = '<div lang="ja">\n';
          else if (token.content.includes("<!-- edge-source-ja:end -->"))
            token.content = "</div>\n";
        }
      });
    },
  },
  vite: { build: { target: "esnext" } },
  head: [["link", { rel: "icon", href: "data:," }]],
  themeConfig: {
    search: {
      provider: "local",
      options: {
        _render: renderForSearch,
        locales: {
          ja: {
            translations: {
              button: { buttonText: "検索", buttonAriaLabel: "検索" },
              modal: {
                displayDetails: "詳細を表示",
                resetButtonTitle: "検索をクリア",
                backButtonTitle: "閉じる",
                noResultsText: "見つかりませんでした",
                footer: {
                  selectText: "選択",
                  navigateText: "移動",
                  closeText: "閉じる",
                },
              },
            },
          },
        },
      },
    },
    outline: { level: [2, 3] },
    nav: JSON.parse(readFileSync(path.join(buildRoot, "nav-en.json"), "utf8")),
    sidebar: JSON.parse(
      readFileSync(path.join(buildRoot, "sidebar-en.json"), "utf8"),
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
    if (sourcePreview)
      return [["meta", { name: "robots", content: "noindex,nofollow" }]];
    const route = `/${pageData.relativePath.replace(/index\.md$/u, "").replace(/\.md$/u, "")}`;
    return [
      ["link", { rel: "canonical", href: `${origin}${route}` }],
      ["meta", { property: "og:url", content: `${origin}${route}` }],
      ["meta", { property: "og:title", content: title }],
      ["meta", { property: "og:description", content: description }],
    ];
  },
});
