#!/usr/bin/env node

// The Forms repository is intentionally a definition project. This check is
// a local import/path firewall: it must not grow a provider, Host runtime,
// account/deploy surface, or a dependency on another checkout.
import { existsSync, lstatSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const forbiddenDirectories = new Set([
  ".wrangler",
  "provider",
  "providers",
  "host",
  "deploy",
  "release",
  "account",
  "accounts",
  "releases",
]);
const forbiddenPathFragments = [
  "terraform-plugin-framework",
  "terraform-plugin-go",
  "github.com/tako0614/terraform-provider-takoform",
  "github.com/tako0614/takoform-forms/formpackage",
  "/root/dev/takos/",
];

const failures = [];
const sourceFiles = [];

function walk(directory, relative = "") {
  for (const name of readdirSync(directory).sort()) {
    if (name === ".git" || name === "node_modules") continue;
    const absolute = path.join(directory, name);
    const entry = relative ? `${relative}/${name}` : name;
    const info = lstatSync(absolute);
    if (info.isSymbolicLink()) {
      failures.push(`${entry}: symlinks are not allowed`);
    } else if (info.isDirectory()) {
      // Package publication is the one release-owned surface in this repo.
      // Keep the generic release/deploy boundary closed everywhere else while
      // allowing the exact content-addressed source tree.
      const allowedPublicationRoot = entry === "forms/releases";
      if (forbiddenDirectories.has(name) && !allowedPublicationRoot)
        failures.push(`${entry}: forbidden owner boundary`);
      walk(absolute, entry);
    } else if (info.isFile()) {
      sourceFiles.push({ entry, absolute });
    } else {
      failures.push(`${entry}: unsupported filesystem entry`);
    }
  }
}

walk(root);
for (const { entry, absolute } of sourceFiles) {
  if (!/\.(?:go|mjs|json|md|mod|sum|yaml|yml|toml)$/u.test(entry)) continue;
  // This checker necessarily contains the deny-list literals itself.
  if (entry === "scripts/check-boundary.mjs") continue;
  const text = readFileSync(absolute, "utf8");
  for (const fragment of forbiddenPathFragments) {
    if (text.includes(fragment))
      failures.push(`${entry}: forbidden dependency ${fragment}`);
  }
}

const packageJson = JSON.parse(
  readFileSync(path.join(root, "package.json"), "utf8"),
);
for (const [name, command] of Object.entries(packageJson.scripts ?? {})) {
  const publicationEntrypoint =
    name === "deploy" && command === "bun scripts/deploy.mjs";
  if (
    !publicationEntrypoint &&
    /\b(?:deploy|publish|release|wrangler)\b/iu.test(command)
  ) {
    failures.push(
      `package script ${name}: release/deploy mutation is not allowed`,
    );
  }
}
const goMod = readFileSync(path.join(root, "go.mod"), "utf8");
if (/terraform-plugin|terraform-provider/iu.test(goMod)) {
  failures.push("go.mod: provider implementation dependency detected");
}
if (!goMod.startsWith("module github.com/tako0614/takoform-forms\n")) {
  failures.push("go.mod: unexpected module path");
}
if (!/^\s*github\.com\/tako0614\/takoform\s+v1\.0\.1(?:\s|$)/mu.test(goMod)) {
  failures.push("go.mod: public Takoform Core v1.0.1 dependency is required");
}
if (/^\s*replace(?:\s|\(|$)/mu.test(goMod)) {
  failures.push("go.mod: replace directives are forbidden");
}

if (packageJson.private !== true || packageJson.version !== "0.0.0-private") {
  failures.push(
    "package.json: private tooling metadata must use version 0.0.0-private",
  );
}

for (const [relative, label] of [
  ["formpackage", "local Core copy"],
  ["formpackage/schemas", "local embedded schema copy"],
  ["spec/schemas", "local schema tree"],
]) {
  if (existsSync(path.join(root, relative))) {
    failures.push(
      `${relative}: ${label} is forbidden; consume schemas from public Core`,
    );
  }
}

if (failures.length) {
  throw new Error(`definition boundary check failed:\n${failures.join("\n")}`);
}
console.log(
  `definition boundary is closed (${sourceFiles.length} files inspected)`,
);
