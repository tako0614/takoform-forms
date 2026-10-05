import { describe, expect, test } from "bun:test";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import { loadEdgeV2Docs, writeEdgeV2Docs } from "./edge-v2-docs.mjs";

describe("authored Host API v2 documentation routes", () => {
  test("loads Japanese Form and guide sources into stable localized routes", () => {
    const root = fixtureRoot();
    const docsRoot = path.join(root, "generated");
    try {
      write(root, formPath("HostAPI", "2.0.0"), formSource());
      write(root, "spec/guides/index.md", guideSource("Host API v2 guide"));
      write(
        root,
        "spec/guides/client-setup.md",
        guideSource(
          "クライアント設定",
          "手順。draftという語は本文の説明として残せる。",
        ),
      );
      const result = writeEdgeV2Docs({ root, docsRoot });
      expect(result.entries.map(({ route }) => route)).toEqual([
        "/v2/",
        "/v2/client-setup/",
        "/forms/HostAPI/2.0.0/",
      ]);
      expect(result.routes).toEqual([
        "/v2/",
        "/ja/v2/",
        "/v2/client-setup/",
        "/ja/v2/client-setup/",
        "/forms/HostAPI/2.0.0/",
        "/ja/forms/HostAPI/2.0.0/",
      ]);

      const ja = readFileSync(
        path.join(docsRoot, "ja/forms/HostAPI/2.0.0/index.md"),
        "utf8",
      );
      expect(ja).toContain("Form仕様は未公開");
      expect(ja).toContain("日本語の規範本文。");
      expect(ja).toContain("edgeSourceLanguage: ja");
      expect(ja).toContain("edge-source-ja:start");
      const en = readFileSync(
        path.join(docsRoot, "forms/HostAPI/2.0.0/index.md"),
        "utf8",
      );
      expect(en).toContain("authored in Japanese");
      expect(en).toContain("日本語の規範本文。");
      expect(en).not.toContain("This English translation");
      expect(en).not.toMatch(/\bdraft\b/iu);
      expect(loadEdgeV2Docs(root)).toHaveLength(result.entries.length);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("rejects exact collisions with existing v1 routes before writing", () => {
    const root = fixtureRoot();
    const docsRoot = path.join(root, "generated");
    try {
      write(
        root,
        formPath("WorkerVersion", "0.4.0"),
        formSource("WorkerVersion", "0.4.0"),
      );
      expect(() =>
        writeEdgeV2Docs({
          root,
          docsRoot,
          existingRoutes: [
            "/forms/WorkerVersion/0.4.0/",
            "/ja/forms/WorkerVersion/0.4.0/",
          ],
        }),
      ).toThrow("route collision");
      expect(() =>
        readFileSync(path.join(docsRoot, "forms/WorkerVersion/0.4.0/index.md")),
      ).toThrow();
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("maps the authored home page to root and keeps the guide index at /v2", () => {
    const root = fixtureRoot();
    const docsRoot = path.join(root, "generated");
    try {
      write(
        root,
        "spec/guides/home.md",
        guideSource(
          "Edge Forms",
          "[ガイド](/v2/) [Form](/forms/HostAPI/2.0.0/) [過去資料](/v1/)",
        ),
      );
      write(root, "spec/guides/index.md", guideSource("API v2を使う"));
      const result = writeEdgeV2Docs({
        root,
        docsRoot,
        existingRoutes: ["/", "/ja/"],
      });
      expect(result.routes).toEqual(["/", "/ja/", "/v2/", "/ja/v2/"]);
      expect(readFileSync(path.join(docsRoot, "index.md"), "utf8")).toContain(
        "edgeSourceLanguage: ja",
      );
      expect(
        readFileSync(path.join(docsRoot, "v2/index.md"), "utf8"),
      ).toContain("API v2を使う");
      const jaHome = readFileSync(path.join(docsRoot, "ja/index.md"), "utf8");
      expect(jaHome).toContain("[ガイド](/ja/v2/)");
      expect(jaHome).toContain("[Form](/ja/forms/HostAPI/2.0.0/)");
      expect(jaHome).toContain("[過去資料](/ja/v1/)");
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("validates canonical Form metadata and parses authored JSON examples", () => {
    const root = fixtureRoot();
    try {
      write(
        root,
        formPath("HostAPI", "2.0.0"),
        `${formSource()}\n\n\`\`\`json\n{"valid":true}\n\`\`\``,
      );
      write(
        root,
        "spec/guides/broken.md",
        `${guideSource("Broken")}\n\n\`\`\`json\n{broken}\n\`\`\``,
      );
      expect(() => loadEdgeV2Docs(root)).toThrow("invalid JSON example");

      rmSync(path.join(root, "spec/guides/broken.md"));
      write(
        root,
        formPath("WorkerVersion", "0.5.0"),
        formSource("WorkerVersion", "0.5.0").replace(
          "WorkerVersion/0.5.0/",
          "WorkerVersion/0.4.0/",
        ),
      );
      expect(() => loadEdgeV2Docs(root)).toThrow(
        "formUrl must match canonical route",
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  test("allows empty fixture roots and ignores non-Markdown assets", () => {
    const root = fixtureRoot();
    try {
      expect(loadEdgeV2Docs(root)).toEqual([]);
      write(root, "spec/guides/alpha.md", guideSource("A"));
      write(root, "spec/guides/assets/diagram.svg", "<svg/>");
      expect(loadEdgeV2Docs(root)).toHaveLength(1);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});

function fixtureRoot() {
  return mkdtempSync(path.join(tmpdir(), "edge-v2-docs-"));
}

function formPath(kind, version) {
  return `spec/forms/${kind}/${version}/index.md`;
}

function formSource(kind = "HostAPI", version = "2.0.0") {
  return `---\ntitle: ${kind} ${version}\ndescription: Formの説明\nformUrl: https://edge.forms.takoform.com/forms/${kind}/${version}/\nhostApi: forms.takoform.com/v2\n---\n\n# ${kind} ${version}\n\n日本語の規範本文。`;
}

function guideSource(title, body = "接続方法を説明する。") {
  return `---\ntitle: ${title}\ndescription: Guideの説明\n---\n\n# ${title}\n\n${body}`;
}

function write(root, relative, value) {
  const file = path.join(root, relative);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, value);
}
