import { describe, expect, test } from "bun:test";
import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
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
  PUBLISHED_V1_SET_ID,
  buildEdgeFormPages,
  buildEdgeFormSourcePreview,
  publishedV1PagePlan,
  readInstalledTrustSet,
  readPublishedV1Guide,
  validateReadingGuide,
} from "./edge-form-pages.mjs";
import { verifyEdgeFormFreeze } from "./edge-form-freeze.mjs";
import { derivePublicationPlan } from "./form-publication.mjs";
import { loadEdgeV2Docs } from "./edge-v2-docs.mjs";
import { renderVitePressPages } from "./edge-form-pages-vitepress.mjs";
import { renderForSearch, tokenize } from "../site/.vitepress/search.mjs";

const SET_ID = "e7f8a39311dd011b8467e97e7f300cabb9a6b06c";

describe("publisher-owned Edge Form pages", () => {
  test("builds frozen v2 Forms beside all 19 already-published v1 versions without signing new v1 candidates", () => {
    const root = path.resolve(".");
    const trust = readInstalledTrustSet(PUBLISHED_V1_SET_ID);
    const plan = publishedV1PagePlan(derivePublicationPlan(), trust);
    const freeze = verifyEdgeFormFreeze(root, {
      mode: "check",
      requireFrozen: true,
    });
    expect(freeze.status).toBe("FROZEN");
    const output = mkdtempSync(path.join(tmpdir(), "edge-v2-frozen-build-"));
    try {
      const build = buildEdgeFormPages({
        outputDirectory: output,
        plan,
        trust,
        publishedForms: freeze.frozen,
        readingGuide: readPublishedV1Guide(),
      });
      expect(build.formCount).toBe(17);
      expect(build.retainedCount).toBe(2);
      expect(build.publishedV2FormCount).toBe(17);
      expect(build.routes).toContain("/forms/ModuleWorker/0.1.0/");
      expect(build.routes).toContain("/forms/ModuleWorker/0.3.0/");
      expect(build.routes).toContain("/ja/forms/ModuleWorker/0.3.0/");
      expect(
        readFileSync(path.join(output, "forms/ModuleWorker/0.3.0/source.md")),
      ).toEqual(readFileSync("spec/forms/ModuleWorker/0.3.0/index.md"));
      expect(
        readFileSync(
          path.join(output, "forms/ModuleWorker/0.3.0/index.html"),
          "utf8",
        ),
      ).toContain("ModuleWorker 0.3.0");
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  }, 60_000);
  test("does not let authored drafts replace a missing v1 production roster", () => {
    const output = mkdtempSync(path.join(tmpdir(), "edge-v2-prod-guard-"));
    try {
      expect(() =>
        renderVitePressPages({
          root: path.resolve("."),
          outputDirectory: output,
          forms: [],
          trust: undefined,
          isPublic: false,
          origin: EDGE_FORM_PAGES_ORIGIN,
        }),
      ).toThrow("current WorkerVersion Form is required");
      expect(readdirSync(output)).toEqual([]);
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  });

  test("source check renders an honest unpublished preview without a signed set or durable output", () => {
    const result = spawnSync(
      process.execPath,
      ["scripts/edge-form-pages.mjs", "--check-source"],
      { cwd: path.resolve("."), encoding: "utf8", maxBuffer: 64 * 1024 * 1024 },
    );
    expect(result.status).toBe(0);
    const report = JSON.parse(result.stdout);
    expect(report.publicationStatus).toBe("UNPUBLISHED");
    expect(report.signedSet).toBeUndefined();
    expect(report.formCount).toBe(17);
    expect(report.retainedCount).toBe(11);
    expect(existsSync(report.outputDirectory)).toBe(false);
  }, 60_000);

  test("source preview renders real pages without public metadata or unborn package links", () => {
    const plan = derivePublicationPlan();
    const output = mkdtempSync(
      path.join(tmpdir(), "edge-form-source-preview-"),
    );
    try {
      const report = buildEdgeFormSourcePreview({
        outputDirectory: output,
        plan,
      });
      expect(report.publicationStatus).toBe("UNPUBLISHED");
      expect(report.signedSet).toBeUndefined();
      expect(existsSync(path.join(output, "sitemap.xml"))).toBe(false);
      expect(existsSync(path.join(output, "_headers"))).toBe(false);
      expect(readFileSync(path.join(output, "robots.txt"), "utf8")).toContain(
        "Disallow: /",
      );
      const hasV2Home = loadEdgeV2Docs(path.resolve(".")).some(
        (entry) => entry.route === "/",
      );
      const authoredEntries = loadEdgeV2Docs(path.resolve("."));
      const expectedRoutes = new Set([
        ...(hasV2Home ? ["/", "/ja/", "/v1/", "/ja/v1/"] : ["/", "/ja/"]),
        ...[...plan.forms, ...plan.retainedPackages].flatMap((form) => [
          `/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`,
          `/ja/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`,
        ]),
        ...authoredEntries.flatMap(({ route }) => [route, `/ja${route}`]),
      ]);
      const htmlRoutes = tree(output)
        .filter((relative) => relative.endsWith("index.html"))
        .map((relative) =>
          relative === "index.html"
            ? "/"
            : `/${relative.replace(/index\.html$/u, "")}`,
        )
        .sort();
      expect(htmlRoutes).toEqual([...expectedRoutes].sort());
      expect(report.routeCount).toBe(expectedRoutes.size);
      for (const relative of tree(output).filter((entry) =>
        entry.endsWith(".html"),
      )) {
        const html = readFileSync(path.join(output, relative), "utf8");
        expect(html).toContain("noindex,nofollow");
        expect(html).not.toContain('rel="canonical"');
        expect(html).not.toContain('property="og:url"');
        expect(html).not.toContain("Public package readback verified");
        expect(html).not.toContain("/tree/forms%2F");
        expect(html).not.toContain("Open immutable package");
      }
      expect(
        readFileSync(
          path.join(output, hasV2Home ? "v1/index.html" : "index.html"),
          "utf8",
        ),
      ).toContain("UNPUBLISHED source preview");
      expect(
        readFileSync(
          path.join(output, "forms/WorkerVersion/0.4.0/index.html"),
          "utf8",
        ),
      ).toContain("UNPUBLISHED source preview");
      for (const relative of [
        "forms/ObjectBucket/0.1.0/index.html", // already published, unchanged
        "forms/WorkerVersion/0.4.0/index.html", // new unsigned successor
      ]) {
        const html = readFileSync(path.join(output, relative), "utf8");
        expect(html).toContain("The selected source roster is UNPUBLISHED");
        expect(html).toContain("Core-derived tag (publication not asserted)");
        expect(html).not.toContain(
          "This candidate has not been signed or published",
        );
        expect(html).not.toContain("Candidate tag (not published)");
        expect(html).not.toContain("not a signed-set member");
      }
      const retained = readFileSync(
        path.join(output, "forms/WorkerVersion/0.3.0/index.html"),
        "utf8",
      );
      expect(retained).toContain(
        "Historical package in unpublished source preview",
      );
      expect(retained).not.toContain(
        "This candidate has not been signed or published",
      );
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  }, 60_000);

  test("source preview rejects stale guide and a missing selected package before rendering", () => {
    const plan = derivePublicationPlan();
    const guide = JSON.parse(readFileSync("site/reading-guide.json", "utf8"));
    guide.WorkerVersion.version = "0.3.0";
    expect(() => validateReadingGuide(guide, plan.forms)).toThrow(
      /requires review/,
    );
    const missing = structuredClone(plan);
    missing.forms[0].locator.sourcePath += "-missing";
    const output = mkdtempSync(
      path.join(tmpdir(), "edge-form-source-missing-"),
    );
    try {
      expect(() =>
        buildEdgeFormSourcePreview({ outputDirectory: output, plan: missing }),
      ).toThrow();
      expect(readdirSync(output)).toEqual([]);
    } finally {
      rmSync(output, { recursive: true, force: true });
    }
  }, 60_000);

  test("source check cannot be given a trust set or durable output", () => {
    for (const extra of [
      ["--trust-set", SET_ID],
      ["--output", "/tmp/forbidden"],
    ]) {
      const result = spawnSync(
        process.execPath,
        ["scripts/edge-form-pages.mjs", "--check-source", ...extra],
        {
          cwd: path.resolve("."),
          encoding: "utf8",
        },
      );
      expect(result.status).not.toBe(0);
      expect(result.stderr).toContain("usage:");
    }
  });

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
  }, 30_000);
  test("renders one deterministic human page per exact signed package", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    const fixture = legacyFixtureRoot();
    const first = mkdtempSync(path.join(tmpdir(), "edge-form-pages-a-"));
    const second = mkdtempSync(path.join(tmpdir(), "edge-form-pages-b-"));
    try {
      const previousPreviewFlag = process.env.EDGE_FORM_SOURCE_PREVIEW;
      process.env.EDGE_FORM_SOURCE_PREVIEW = "1";
      let firstResult;
      try {
        firstResult = buildEdgeFormPages({
          outputDirectory: first,
          plan,
          trust,
          root: fixture,
        });
      } finally {
        if (previousPreviewFlag === undefined)
          delete process.env.EDGE_FORM_SOURCE_PREVIEW;
        else process.env.EDGE_FORM_SOURCE_PREVIEW = previousPreviewFlag;
      }
      const secondResult = buildEdgeFormPages({
        outputDirectory: second,
        plan,
        trust,
        root: fixture,
      });

      expect(firstResult).toEqual(secondResult);
      expect(
        readFileSync(
          path.join(fixture, "site/.vitepress/.temp/keep.txt"),
          "utf8",
        ),
      ).toBe("another build owns this directory");
      expect(firstResult.formCount).toBe(plan.formCount);
      const authoredEntries = loadEdgeV2Docs(fixture);
      const hasV2Home = authoredEntries.some((entry) => entry.route === "/");
      const expectedV1Routes = ["en", "ja"].flatMap((locale) => {
        const prefix = locale === "ja" ? "/ja" : "";
        return [
          ...(hasV2Home
            ? [locale === "ja" ? "/ja/v1/" : "/v1/"]
            : [locale === "ja" ? "/ja/" : "/"]),
          ...[...plan.forms, ...plan.retainedPackages].map(
            (form) =>
              `${prefix}/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`,
          ),
        ];
      });
      const authoredV2Routes = authoredEntries.flatMap(({ route }) => [
        route,
        `/ja${route}`,
      ]);
      const v1RouteSet = new Set(expectedV1Routes);
      const actualV1Routes = firstResult.routes.filter((route) =>
        v1RouteSet.has(route),
      );
      const actualAuthoredRoutes = firstResult.routes.filter(
        (route) => !v1RouteSet.has(route),
      );
      expect(new Set(actualV1Routes)).toEqual(v1RouteSet);
      expect(actualV1Routes).toHaveLength(expectedV1Routes.length);
      expect(actualAuthoredRoutes).toEqual(authoredV2Routes);
      expect(firstResult.routes[0]).toBe("/");
      expect(tree(first)).toEqual(tree(second));
      for (const relative of tree(first)) {
        expect(readFileSync(path.join(first, relative))).toEqual(
          readFileSync(path.join(second, relative)),
        );
      }

      const root = readFileSync(path.join(first, "index.html"), "utf8");
      const legacyRoot = readFileSync(
        path.join(first, hasV2Home ? "v1/index.html" : "index.html"),
        "utf8",
      );
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
          `href="https://takoform.com/${japanese ? "" : "en/"}host-api/"`,
        );
        expect(sidebar).toContain(
          japanese ? "TakoformのHost API" : "Takoform Host API",
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
          if (
            (!hasV2Home && route === "/ja/") ||
            (hasV2Home && route === "/ja/v1/")
          )
            expect(html).toContain("定義を使う");
        }
      }
      const expectedFormRoutes = [
        ...[...plan.forms, ...plan.retainedPackages].map(
          (form) =>
            `/forms/${form.formRef.kind}/${form.formRef.definitionVersion}/`,
        ),
      ];
      const actualFormRoutes = [
        ...new Set(
          [...root.matchAll(/href="(\/forms\/[^"#]+)"/gu)].map(
            (match) => match[1],
          ),
        ),
      ];
      expect(actualFormRoutes.sort()).toEqual(expectedFormRoutes.sort());
      expect(legacyRoot).toContain(`${plan.formCount} signed Edge Forms`);
      if (hasV2Home) {
        expect(root).toContain("Edge Forms");
        expect(root).toContain('<div lang="ja">');
      }
      expect(root).toContain('rel="canonical"');
      expect(root).not.toContain("noindex,nofollow");
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
        path.join(first, "forms/WorkerVersion/0.4.0/index.html"),
        "utf8",
      );
      expect(worker).toContain("Required");
      expect(worker).toContain("Optional");
      expect(worker).toContain("Example desired state");
      expect(worker).toContain('id="desired-schema"');
      expect(worker).toContain("additionalProperties");
      expect(worker).toContain("Host support and admission are separate");
      expect(worker).toContain("fixtures/desired.json");
      expect(legacyRoot).toContain("Settings, examples and package references");
      expect(legacyRoot).toContain(
        "ModuleWorker → WorkerVersion → WorkerDeployment",
      );
      expect(legacyRoot).toContain("Where the shared model and API live");
      expect(legacyRoot).toContain('href="https://takoform.com/en/host-api/"');
      expect(legacyRoot).toContain("No separate specification version");
      const workerVersion = plan.forms.find(
        (form) => form.formRef.kind === "WorkerVersion",
      );
      expect(legacyRoot).toContain(
        `href="/forms/WorkerVersion/${workerVersion.formRef.definitionVersion}/#example-title"`,
      );
      const japaneseRoot = readFileSync(
        path.join(first, hasV2Home ? "ja/v1/index.html" : "ja/index.html"),
        "utf8",
      );
      expect(japaneseRoot).toContain("共通モデルとAPIの説明先");
      expect(japaneseRoot).toContain('href="https://takoform.com/host-api/"');
      expect(japaneseRoot).toContain(
        "これらを同期させる独立した仕様の版はありません",
      );
      expect(japaneseRoot).toContain(
        `href="/ja/forms/WorkerVersion/${workerVersion.formRef.definitionVersion}/#example-title"`,
      );
      for (const anchor of [
        "choose-by-task",
        "queue-workflow-actor",
        "composition",
      ]) {
        expect(legacyRoot).toContain(`id="${anchor}"`);
      }
      expect(legacyRoot).toContain(
        "WorkerVersion producer binding → AtLeastOnceQueue → QueueConsumer → ModuleWorker queue handler",
      );
      expect(legacyRoot).toContain("side effects before recording may repeat");
      const guide = JSON.parse(readFileSync("site/reading-guide.json", "utf8"));
      for (const form of plan.forms) {
        for (const prefix of ["", "ja/"]) {
          const content = readFileSync(
            path.join(
              first,
              prefix,
              "forms",
              form.formRef.kind,
              form.formRef.definitionVersion,
              "index.html",
            ),
            "utf8",
          );
          expect(content).toContain('id="connections"');
          expect(content).toContain('id="next-steps"');
          expect(content).toContain(
            `https://takoform.com/${prefix ? "" : "en/"}client/`,
          );
          expect(content).not.toContain("undefined");
          const heading = prefix
            ? "組み合わせと比較対象"
            : "Connections and alternatives";
          expect(content).toContain(heading);
          for (const relation of guide[form.kind].related) {
            const target = plan.forms.find(
              (entry) => entry.kind === relation.kind,
            );
            expect(content).toContain(
              `href="/${prefix}forms/${target.kind}/${target.formRef.definitionVersion}/"`,
            );
            expect(typeof relation.relation).toBe("string");
            expect(typeof relation.ja).toBe("string");
          }
        }
      }
      expect(existsSync(path.join(first, "sitemap.xml"))).toBe(true);
      const sitemap = readFileSync(path.join(first, "sitemap.xml"), "utf8");
      for (const route of firstResult.routes)
        expect(sitemap).toContain(
          `<loc>${EDGE_FORM_PAGES_ORIGIN}${route}</loc>`,
        );
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
      rmSync(fixture, { recursive: true, force: true });
    }
  }, 60000);

  test("claims public readability only with exact package readback evidence", () => {
    const plan = derivePublicationPlan();
    const trust = signedTrustFor(plan);
    const fixture = legacyFixtureRoot();
    const output = mkdtempSync(path.join(tmpdir(), "edge-form-pages-public-"));
    try {
      expect(() =>
        buildEdgeFormPages({
          outputDirectory: output,
          plan,
          trust,
          root: fixture,
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
        root: fixture,
        publicReadback: publicReadbackFor(plan, trust),
      });
      expect(readFileSync(path.join(output, "index.html"), "utf8")).toContain(
        "Public package readback verified",
      );
    } finally {
      rmSync(output, { recursive: true, force: true });
      rmSync(fixture, { recursive: true, force: true });
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

function legacyFixtureRoot() {
  const root = mkdtempSync(path.join(tmpdir(), "edge-form-v1-fixture-"));
  for (const entry of ["forms", "node_modules", "cmd"])
    symlinkSync(path.resolve(entry), path.join(root, entry), "dir");
  for (const entry of ["go.mod", "go.sum"])
    symlinkSync(path.resolve(entry), path.join(root, entry), "file");
  mkdirSync(path.join(root, "site/.vitepress/.temp"), { recursive: true });
  for (const entry of [
    "site/reading-guide.json",
    "site/.vitepress/config.mts",
    "site/.vitepress/search.mjs",
  ])
    copyFileSync(path.resolve(entry), path.join(root, entry));
  writeFileSync(
    path.join(root, "site/.vitepress/.temp/keep.txt"),
    "another build owns this directory",
  );
  return root;
}
