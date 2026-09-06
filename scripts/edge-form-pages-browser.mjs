// Explicit browser lane: no browser download, production access or user profile.
import { chromium } from "playwright-core";
import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const executablePath =
  process.env.TAKOFORM_BROWSER ||
  [
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    chromium.executablePath(),
  ].find((file) => existsSync(file));
if (!executablePath || !existsSync(executablePath))
  throw new Error(
    "Chrome unavailable; set TAKOFORM_BROWSER to an installed browser",
  );
const result = spawnSync("bun", ["run", "build:edge-form-pages"], {
  cwd: root,
  encoding: "utf8",
});
if (result.status) throw new Error(`site build failed: ${result.stderr}`);
const build = JSON.parse(result.stdout);
const mime = {
  ".html": "text/html",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".woff2": "font/woff2",
  ".xml": "application/xml",
  ".txt": "text/plain",
  ".js": "text/javascript",
  ".json": "application/json",
};
const headers = Object.fromEntries(
  readFileSync(path.join(build.outputDirectory, "_headers"), "utf8")
    .split("\n")
    .filter((line) => line.startsWith("  "))
    .map((line) => {
      const colon = line.indexOf(":");
      return [line.slice(0, colon).trim(), line.slice(colon + 1).trim()];
    }),
);
const server = createServer((request, response) => {
  const route = new URL(request.url, "http://localhost").pathname;
  const relative = route.endsWith("/")
    ? `${route.slice(1)}index.html`
    : route.slice(1);
  const found = build.files.includes(relative) && relative !== "_headers";
  response.writeHead(found ? 200 : 404, {
    ...headers,
    "Content-Type": `${mime[path.extname(found ? relative : "404.html")] ?? "application/octet-stream"}`,
  });
  response.end(
    readFileSync(
      path.join(build.outputDirectory, found ? relative : "404.html"),
    ),
  );
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser;
try {
  browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox"],
  });
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("requestfailed", (request) => errors.push(request.url()));
  await context.route("**/*", (route) =>
    route.request().url().startsWith(origin) ? route.continue() : route.abort(),
  );
  for (const width of [320, 375, 414, 768]) {
    await page.setViewportSize({ width, height: 900 });
    const descriptions = new Set();
    for (const route of build.routes) {
      const response = await page.goto(origin + route, {
        waitUntil: "networkidle",
      });
      if (response.status() !== 200)
        throw new Error(`HTTP failure at ${route}`);
      await page.evaluate(() => document.fonts.ready);
      const problems = await page.evaluate(() => {
        const failures = [];
        if (
          document.documentElement.scrollWidth > innerWidth ||
          document.body.scrollWidth > innerWidth
        )
          failures.push(
            `horizontal page overflow: ${[
              ...document.querySelectorAll("body *"),
            ]
              .map((node) => ({
                tag: node.tagName,
                class: node.className,
                width: Math.round(node.getBoundingClientRect().width),
                left: Math.round(node.getBoundingClientRect().left),
                right: Math.round(node.getBoundingClientRect().right),
              }))
              .filter(
                (node) => node.width > innerWidth || node.right > innerWidth,
              )
              .slice(0, 8)
              .map(
                (node) =>
                  `${node.tag}.${node.class}:${node.left}..${node.right}`,
              )
              .join(" ")}`,
          );
        if (
          document.querySelectorAll("main").length !== 1 ||
          document.querySelectorAll("h1").length !== 1
        )
          failures.push("main/heading structure");
        if (
          [...document.querySelectorAll("dt,dd")].some(
            (node) => !node.closest("dl"),
          )
        )
          failures.push("invalid definition list");
        const ids = [...document.querySelectorAll("[id]")].map(
          (node) => node.id,
        );
        if (new Set(ids).size !== ids.length) failures.push("duplicate IDs");
        for (const link of document.querySelectorAll('a[href^="#"]'))
          if (!document.getElementById(decodeURIComponent(link.hash.slice(1))))
            failures.push("missing anchor");
        for (const node of document.querySelectorAll(
          "a,button,input,summary",
        )) {
          if (
            !node.checkVisibility({
              checkOpacity: true,
              checkVisibilityCSS: true,
            })
          )
            continue;
          if (
            node.closest(".VPSkipLink") ||
            node.classList.contains("header-anchor")
          )
            continue;
          const sidebar = node.closest(".VPSidebar");
          if (
            sidebar &&
            !sidebar.classList.contains("open") &&
            innerWidth < 960
          )
            continue;
          const r = node.getBoundingClientRect();
          if (
            r.width &&
            !node.classList.contains("skip-link") &&
            (r.left < -1 || r.right > innerWidth + 1)
          )
            failures.push("clipped interactive element");
        }
        for (const pre of document.querySelectorAll("pre")) {
          try {
            JSON.parse(pre.textContent);
          } catch {
            failures.push("invalid JSON example");
          }
        }
        return failures;
      });
      if (problems.length)
        throw new Error(`${width}px ${route}: ${problems.join(", ")}`);
      const description = await page
        .locator('meta[name="description"]')
        .getAttribute("content");
      if (!description || descriptions.has(description))
        throw new Error(`${route}: missing/duplicate page description`);
      descriptions.add(description);
      const links = await page
        .locator('a[href^="/"]')
        .evaluateAll((nodes) => nodes.map((node) => node.getAttribute("href")));
      for (const link of links)
        if (!build.routes.includes(new URL(link, origin).pathname))
          throw new Error(`${route}: missing internal route ${link}`);
    }
  }
  await page.setViewportSize({ width: 320, height: 900 });
  await page.goto(`${origin}/forms/WorkerVersion/0.3.0/`);
  const summary = page.locator("#desired-schema details > summary");
  await summary.focus();
  await page.keyboard.press("Enter");
  if (
    !(await page
      .locator("#desired-schema details")
      .evaluate((node) => node.open))
  )
    throw new Error("keyboard schema disclosure failed");
  const focus = await summary.evaluate(
    (node) => getComputedStyle(node).outlineStyle,
  );
  if (focus === "none")
    throw new Error("schema control has no visible keyboard focus");
  await page.keyboard.press("Enter");
  const menu = page.locator(".VPLocalNav button.menu");
  await menu.focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() =>
    document.querySelector(".VPSidebar")?.classList.contains("open"),
  );
  await page.keyboard.press("Escape");
  await page.waitForFunction(
    () => !document.querySelector(".VPSidebar")?.classList.contains("open"),
  );
  await page.locator(".VPNavBarSearch button").click();
  await page.locator("#localsearch-input").fill("bucketBindings");
  const searchResult = page
    .locator('.VPLocalSearchBox a[href*="WorkerVersion/0.3.0/"]')
    .first();
  await searchResult.waitFor();
  await searchResult.click();
  await page.waitForFunction(
    () => !document.querySelector(".VPLocalSearchBox"),
  );
  await page.screenshot({
    path: "/tmp/edge-form-docs-mobile.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(origin);
  for (const colorScheme of ["dark", "light"]) {
    await page.emulateMedia({ colorScheme });
    await page.reload({ waitUntil: "networkidle" });
    if (
      (await page
        .locator("html")
        .evaluate((node) => node.classList.contains("dark"))) !==
      (colorScheme === "dark")
    )
      throw new Error(`${colorScheme}: theme did not follow system preference`);
  }
  await page.goto(`${origin}/forms/WorkerVersion/0.3.0/`);
  const next = page.locator("a.pager-link.next");
  const nextRoute = await next.getAttribute("href");
  await next.click();
  await page.waitForURL(origin + nextRoute);
  await page.goto(origin);
  await page.screenshot({
    path: "/tmp/edge-form-docs-desktop.png",
    fullPage: true,
  });
  if (errors.length) throw new Error(errors.join("\n"));
  console.log(
    `edge-form-pages-browser: ${build.routes.length * 4} responsive pages, JSON, navigation and keyboard disclosure passed (${browser.version()})`,
  );
} finally {
  await browser?.close();
  await new Promise((resolve) => server.close(resolve));
  rmSync(build.outputDirectory, { recursive: true, force: true });
}
