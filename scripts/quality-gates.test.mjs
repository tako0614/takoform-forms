import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const packageJson = JSON.parse(
  readFileSync(resolve(root, "package.json"), "utf8"),
);
const readme = readFileSync(resolve(root, "forms/README.md"), "utf8");

describe("portable quality gates and Form version documentation", () => {
  test("checks Go and tracked JavaScript formatting without writing", () => {
    const formatCheck = packageJson.scripts["format:check"];
    expect(formatCheck).toContain("gofmt -l .");
    expect(formatCheck).toContain("node_modules/.bin/prettier --check");
    expect(formatCheck).toContain("git ls-files -- '*.js' '*.mjs'");
    expect(formatCheck).not.toContain("node --check");

    const formatter = packageJson.scripts.fmt;
    expect(formatter).toContain("gofmt -w");
    expect(formatter).toContain("node_modules/.bin/prettier --write");
    expect(formatter).toContain("git ls-files -- '*.js' '*.mjs'");
  });

  test("runs the JavaScript test corpus from the portable test gate", () => {
    expect(packageJson.scripts["test:portable"]).toContain(
      "bun test scripts/*.test.mjs",
    );
  });

  test("documents exactly the public API and Form definition axes", () => {
    const axisRows = readme
      .split("\n")
      .filter(
        (line) =>
          line.startsWith("| API/Core SemVer |") ||
          line.startsWith("| Form definition |"),
      );

    expect(readme).toContain("exactly two version axes");
    expect(axisRows).toHaveLength(2);
    expect(
      axisRows.some(
        (line) =>
          line.startsWith("| API/Core SemVer |") &&
          line.includes("`1.x`") &&
          line.includes("`/v1`"),
      ),
    ).toBe(true);
    expect(
      axisRows.some(
        (line) =>
          line.startsWith("| Form definition |") &&
          line.includes("`definitionVersion`"),
      ),
    ).toBe(true);
  });
});
