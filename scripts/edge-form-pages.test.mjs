import { describe, expect, test } from "bun:test";
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
  symlinkSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import {
  EDGE_FORM_PAGES_ORIGIN,
  buildEdgeFormPages,
} from "./edge-form-pages.mjs";
import { derivePublicationPlan } from "./form-publication.mjs";
import { renderForSearch, tokenize } from "../site/.vitepress/search.mjs";

const SET_ID = "e7f8a39311dd011b8467e97e7f300cabb9a6b06c";

describe("publisher-owned Edge Form pages", () => {
  test("tokenizes Japanese usage text and preserves binding names", () => {
    expect(
      tokenize("たとえばチャットの部屋ごとにActorを使えます。bucketBindings"),
    ).toEqual(
      expect.arrayContaining(["チャット", "部屋", "Actor", "bucketBindings"]),
    );
    expect(tokenize("  。  ")).toEqual([]);
    const html =
      '<h2 id="usage">使い方<a href="#usage">#</a></h2><p>たとえばチャットの部屋ごとにActorを使えます。</p>';
    const output = renderForSearch("", {}, { render: () => html });
    expect(output).toContain(
      '<h2 id="usage">使い方<a href="#usage">#</a></h2>',
    );
    expect(output).toContain("チャット の 部屋");
    expect(
      renderForSearch(
        "",
        { frontmatter: { search: false } },
        { render: () => html },
      ),
    ).toBe("");
  });
  test("does not overwrite nonempty or linked output directories", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    const directory = mkdtempSync(
      path.join(tmpdir(), "edge-form-output-guard-"),
    );
    const link = `${directory}-link`;
    try {
      writeFileSync(path.join(directory, "keep.txt"), "user-owned");
      symlinkSync(directory, link);
      for (const outputDirectory of [directory, link])
        expect(() =>
          buildEdgeFormPages({ outputDirectory, plan, trust }),
        ).toThrow("empty, non-symlink");
      expect(readFileSync(path.join(directory, "keep.txt"), "utf8")).toBe(
        "user-owned",
      );
    } finally {
      rmSync(link, { force: true });
      rmSync(directory, { recursive: true, force: true });
    }
  });
  test("renders one deterministic human page per exact signed package", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    const first = mkdtempSync(path.join(tmpdir(), "edge-form-pages-a-"));
    const second = mkdtempSync(path.join(tmpdir(), "edge-form-pages-b-"));
    try {
      const firstResult = buildEdgeFormPages({
        outputDirectory: first,
        plan,
        trust,
      });
      const secondResult = buildEdgeFormPages({
        outputDirectory: second,
        plan,
        trust,
      });

      expect(firstResult).toEqual(secondResult);
      expect(firstResult.formCount).toBe(plan.formCount);
      expect(firstResult.routes).toHaveLength(
        2 * (plan.formCount + plan.retainedPackages.length + 1),
      );
      expect(firstResult.routes[0]).toBe("/");
      expect(tree(first)).toEqual(tree(second));
      for (const relative of tree(first)) {
        expect(readFileSync(path.join(first, relative))).toEqual(
          readFileSync(path.join(second, relative)),
        );
      }

      const root = readFileSync(path.join(first, "index.html"), "utf8");
      for (const route of firstResult.routes) {
        const html = readFileSync(
          path.join(
            first,
            route === "/" ? "index.html" : `${route.slice(1)}index.html`,
          ),
          "utf8",
        );
        const sidebar = html.match(
          /<aside\b[^>]*class="VPSidebar[^>]*>([\s\S]*?)<\/aside>/u,
        )?.[1];
        expect(html).not.toContain("_vp-fn_");
        expect(html).not.toContain("new Function");
        expect(sidebar).toBeDefined();
        const links = [...sidebar.matchAll(/href="(\/[^"#]*)"/gu)].map(
          (match) => match[1],
        );
        const japanese = route.startsWith("/ja/");
        expect(html).toContain(`lang="${japanese ? "ja-JP" : "en"}"`);
        expect(links.sort()).toEqual(
          firstResult.routes
            .filter((entry) => entry.startsWith("/ja/") === japanese)
            .sort(),
        );
        expect(sidebar).not.toMatch(/class="[^"]*\bcollapsed\b/u);
        expect(sidebar).toContain(
          `href="https://takoform.com/${japanese ? "" : "en/"}"`,
        );
        const counterpart = japanese ? route.slice(3) : `/ja${route}`;
        expect(html).toContain(`href="${counterpart}"`);
        if (japanese) {
          const english = readFileSync(
            path.join(first, `${counterpart.slice(1)}index.html`),
            "utf8",
          );
          const headings = (source) =>
            [...source.matchAll(/<h[1-6]\b[^>]*id="([^"]+)"/gu)].map(
              (match) => match[1],
            );
          const examples = (source) =>
            [
              ...source.matchAll(/<div class="language-json[\s\S]*?<\/pre>/gu),
            ].map((match) => match[0]);
          expect(headings(html)).toEqual(headings(english));
          expect(examples(html)).toEqual(examples(english));
          expect(html).toContain(route === "/ja/" ? "定義を使う" : "設定項目");
        }
      }
      expect(
        new Set(
          [...root.matchAll(/href="(\/forms\/[^"#]+)"/gu)].map(
            (match) => match[1],
          ),
        ).size,
      ).toBe(plan.formCount + plan.retainedPackages.length);
      expect(root).toContain(`${plan.formCount} signed Edge Forms`);
      expect(root).not.toContain("Public package readback verified");
      expect(root).not.toContain("API discovery");

      const objectBucket = plan.forms.find(
        (form) => form.formRef.kind === "ObjectBucket",
      );
      const pagePath = path.join(
        first,
        "forms",
        objectBucket.formRef.kind,
        objectBucket.formRef.definitionVersion,
        "index.html",
      );
      const page = readFileSync(pagePath, "utf8");
      const definition = JSON.parse(
        readFileSync(
          path.join(
            plan.repositoryRoot,
            objectBucket.locator.sourcePath,
            "definition.json",
          ),
          "utf8",
        ),
      );
      expect(page).toContain(definition.title);
      expect(page).toContain(
        "Flat-namespace object store with strong read-after-write consistency",
      );
      expect(page).toContain("contract&#39;s 5 GiB ceiling");
      expect(page).toContain(objectBucket.formRef.schemaDigest);
      expect(page).toContain(objectBucket.locator.tag);
      expect(page).toContain(objectBucket.locator.sourcePath);
      expect(page).toContain(
        `${EDGE_FORM_PAGES_ORIGIN}/forms/ObjectBucket/0.1.0/`,
      );
      expect(page).not.toContain("Public package readback verified");
      const worker = readFileSync(
        path.join(first, "forms/WorkerVersion/0.3.0/index.html"),
        "utf8",
      );
      expect(worker).toContain("Required");
      expect(worker).toContain("Optional");
      expect(worker).toContain("Example desired state");
      expect(worker).toContain('id="desired-schema"');
      expect(worker).toContain("additionalProperties");
      expect(worker).toContain("Host support and admission are separate");
      expect(worker).toContain("fixtures/desired.json");
      expect(root).toContain("Settings, examples and package references");
      expect(root).toContain("ModuleWorker → WorkerVersion → WorkerDeployment");
      expect(existsSync(path.join(first, "sitemap.xml"))).toBe(true);
      expect(existsSync(path.join(first, "404.html"))).toBe(true);
      expect(
        firstResult.files.some((file) => /^assets\/.*\.css$/u.test(file)),
      ).toBe(true);
      expect(root).toContain("VPNavBarSearch");
      expect(worker).toContain("VPDocAsideOutline");
      expect(worker).toContain("VPSidebar");
      expect(worker).toContain("pager-link prev");
      expect(worker).toContain("pager-link next");
      expect(firstResult.files).not.toContain("icon.svg");
      expect(root).toContain('rel="icon" href="data:,"');
      expect(root).not.toContain('class="VPImage logo');
      expect(root).not.toContain('property="og:image"');
      expect(readFileSync(path.join(first, "_headers"), "utf8")).toContain(
        "no-transform",
      );
      expect(firstResult.files.some((file) => file.endsWith(".woff2"))).toBe(
        true,
      );
      const policy = readFileSync(path.join(first, "_headers"), "utf8");
      expect(policy).toContain("script-src 'self' 'sha256-");
      expect(policy).not.toMatch(/script-src[^;]*unsafe-inline/u);
      const retained = readFileSync(
        path.join(first, "forms/WorkerVersion/0.2.0/index.html"),
        "utf8",
      );
      expect(retained).toContain(
        "This historical version is not part of the current signed set",
      );
      expect(retained).not.toContain("<td>Signed set</td>");
      expect(retained).not.toContain("bucketBindings");
      expect(worker).toContain("bucketBindings");
      const queue = readFileSync(
        path.join(first, "forms/AtLeastOnceQueue/0.1.0/index.html"),
        "utf8",
      );
      expect(queue.match(/<h1[^>]*>([\s\S]*?)<\/h1>/u)?.[1]).toContain(
        "At-Least-Once Queue",
      );
      expect(retained).toContain("Historical Worker Version definition");
    } finally {
      rmSync(first, { recursive: true, force: true });
      rmSync(second, { recursive: true, force: true });
    }
  }, 60000);

  test("claims public readability only with exact package readback evidence", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    const output = mkdtempSync(path.join(tmpdir(), "edge-form-pages-public-"));
    try {
      expect(() =>
        buildEdgeFormPages({
          outputDirectory: output,
          plan,
          trust,
          publicReadback: {
            kind: "takoform.edge-form-package-readback@v1",
            status: "VERIFIED",
            setId: trust.setId,
            tags: [],
          },
        }),
      ).toThrow(/exact signed package set/);

      buildEdgeFormPages({
        outputDirectory: output,
        plan,
        trust,
        publicReadback: publicReadbackFor(plan, trust),
      });
      expect(readFileSync(path.join(output, "index.html"), "utf8")).toContain(
        "Public package readback verified",
      );
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  }, 60000);

  test("rejects a signed set that differs from the installed package closure", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    trust.packages[0] = {
      ...trust.packages[0],
      packageDigest:
        "sha256:0000000000000000000000000000000000000000000000000000000000000000",
    };
    const output = mkdtempSync(path.join(tmpdir(), "edge-form-pages-drift-"));
    try {
      expect(() =>
        buildEdgeFormPages({ outputDirectory: output, plan, trust }),
      ).toThrow(/signed package closure differs/);
      expect(existsSync(path.join(output, "index.html"))).toBe(false);
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  });
});

function signedTrustFor(plan) {
  return {
    status: "verified",
    coreVersion: "v1.1.0",
    family: plan.family,
    setId: SET_ID,
    sourceCommit: SET_ID,
    packageCount: plan.formCount,
    packages: plan.forms.map((form) => ({
      kind: form.kind,
      formRef: form.formRef,
      packageDigest: form.packageDigest,
      locator: form.locator,
    })),
  };
}

function publicReadbackFor(plan, trust) {
  return {
    kind: "takoform.edge-form-package-readback@v1",
    status: "VERIFIED",
    setId: trust.setId,
    tags: plan.forms.map((form) => ({
      tag: form.locator.tag,
      sourcePath: form.locator.sourcePath,
      packageDigest: form.packageDigest,
    })),
    retainedTags: plan.retainedPackages.map((entry) => ({
      tag: entry.tag,
      packageDigest: entry.packageDigest,
    })),
  };
}

function tree(directory, prefix = "") {
  const found = [];
  for (const name of readdirSync(directory, { withFileTypes: true }).sort(
    (left, right) => left.name.localeCompare(right.name),
  )) {
    const relative = prefix ? `${prefix}/${name.name}` : name.name;
    if (name.isDirectory())
      found.push(...tree(path.join(directory, name.name), relative));
    else found.push(relative);
  }
  return found;
}
