// Read the already-published snapshot; this is not a second artifact store.
// Candidate generation selects current definitions separately from this history.
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { lstatSync, readFileSync } from "node:fs";
import path from "node:path";

export const PUBLISHED_BASELINE_SOURCE =
  "e7f8a39311dd011b8467e97e7f300cabb9a6b06c";
const BASELINE_PATH = "integrity/source-baseline.json";

function safeRelative(relative) {
  return (
    typeof relative === "string" &&
    relative.length > 0 &&
    !path.isAbsolute(relative) &&
    !/[\\\r\n\0]/u.test(relative) &&
    relative.split("/").every((part) => part && part !== "." && part !== "..")
  );
}

function publishedBlob(root, relative) {
  if (!safeRelative(relative))
    throw new Error("unsafe published snapshot path");
  const result = spawnSync(
    "git",
    [
      "--no-replace-objects",
      "--no-lazy-fetch",
      "cat-file",
      "blob",
      `${PUBLISHED_BASELINE_SOURCE}:${relative}`,
    ],
    { cwd: root, maxBuffer: 16 * 1024 * 1024 },
  );
  if (result.error || result.status !== 0) {
    throw new Error(
      `published snapshot ${PUBLISHED_BASELINE_SOURCE}:${relative} is unavailable locally; ` +
        "provide the exact Git history before checking (no automatic fetch)",
    );
  }
  return result.stdout;
}

function regularBytes(root, relative) {
  if (!safeRelative(relative)) throw new Error("unsafe current contract path");
  // Reject a link at any component, not just at the leaf read by readFileSync.
  let current = root;
  const parts = relative.split("/");
  for (let index = 0; index < parts.length; index++) {
    current = path.join(current, parts[index]);
    const info = lstatSync(current);
    if (index === parts.length - 1 ? !info.isFile() : !info.isDirectory()) {
      throw new Error(
        `${relative}: contract path is not a regular repository file`,
      );
    }
  }
  return readFileSync(current);
}

function identity(bytes, label) {
  const definition = JSON.parse(bytes.toString("utf8"));
  const isContract = ["InterfaceDefinition", "BindingDefinition"].includes(
    definition.kind,
  );
  const name = isContract ? definition.name : definition.kind;
  const version = isContract
    ? definition.version
    : definition.definitionVersion;
  if (
    typeof definition.apiVersion !== "string" ||
    !definition.apiVersion ||
    typeof name !== "string" ||
    !name ||
    typeof version !== "string" ||
    !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/u.test(version)
  ) {
    throw new Error(`${label}: invalid contract identity`);
  }
  return JSON.stringify([
    definition.apiVersion,
    isContract ? definition.kind : "FormDefinition",
    name,
    version,
  ]);
}

// Version is part of the lookup key, never permission to replace another
// published version's bytes. A different version is checked by generation and
// Core through the existing owner gate; this check does not approve its meaning.
export function assertPublishedDefinitionsUnchanged(published, current) {
  const exact = new Map();
  for (const [relative, bytes] of published) {
    const key = identity(bytes, relative);
    const previous = exact.get(key);
    if (previous && !previous.equals(bytes)) {
      throw new Error(
        `${relative}: published snapshot reuses an exact identity`,
      );
    }
    exact.set(key, bytes);
  }
  for (const [relative, bytes] of current) {
    const key = identity(bytes, relative);
    const previous = exact.get(key);
    if (previous && !previous.equals(bytes)) {
      throw new Error(
        `${relative}: published identity has different bytes; author a new version`,
      );
    }
  }
}

export function verifyPublishedBaseline(root) {
  const original = publishedBlob(root, BASELINE_PATH);
  if (!original.equals(regularBytes(root, BASELINE_PATH))) {
    throw new Error(
      "historical source-baseline.json must remain byte-identical",
    );
  }
  const baseline = JSON.parse(original.toString("utf8"));
  const published = new Map();
  const current = new Map();
  for (const [relative, expected] of Object.entries(baseline.files)) {
    const bytes = publishedBlob(root, relative);
    const digest = createHash("sha256").update(bytes).digest("hex");
    if (bytes.length !== expected.bytes || digest !== expected.sha256) {
      throw new Error(
        `${relative}: historical source-baseline byte digest drift`,
      );
    }
    if (relative.endsWith("/definition.json")) {
      published.set(relative, bytes);
      current.set(relative, regularBytes(root, relative));
    }
  }
  // Historical Form versions are already retained by the publisher. Include
  // them so selecting an earlier version cannot reassign its published meaning.
  // The abandoned prepublication singleton is deliberately not publication proof.
  const retained = JSON.parse(
    publishedBlob(root, "forms/retained-packages.json").toString("utf8"),
  );
  for (const entry of retained.packages) {
    const relative = `${entry.sourcePath}/definition.json`;
    published.set(relative, publishedBlob(root, relative));
  }
  assertPublishedDefinitionsUnchanged(published, current);
  return baseline;
}
