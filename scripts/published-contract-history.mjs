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

function gitOutput(root, args) {
  const result = spawnSync(
    "git",
    ["--no-replace-objects", "--no-lazy-fetch", ...args],
    { cwd: root, maxBuffer: 32 * 1024 * 1024 },
  );
  if (result.error || result.status !== 0)
    throw new Error(
      `published Git history is unavailable locally: git ${args.join(" ")}`,
    );
  return result.stdout;
}

function taggedBlob(root, ref, relative) {
  if (!safeRelative(relative))
    throw new Error("unsafe tagged publication path");
  return gitOutput(root, ["cat-file", "blob", `${ref}:${relative}`]);
}

function taggedPaths(root, ref, prefix) {
  const output = gitOutput(root, [
    "ls-tree",
    "-r",
    "--name-only",
    "-z",
    ref,
    "--",
    prefix,
  ]);
  return output
    .toString("utf8")
    .split("\0")
    .filter(Boolean)
    .map((relative) => {
      if (!safeRelative(relative) || !relative.startsWith(`${prefix}/`))
        throw new Error(
          `${ref}: unsafe tagged publication path ${JSON.stringify(relative)}`,
        );
      return relative;
    });
}

function tagRefs(root, prefix) {
  return gitOutput(root, [
    "for-each-ref",
    "--format=%(refname)",
    `refs/tags/${prefix}`,
  ])
    .toString("utf8")
    .trim()
    .split("\n")
    .filter(Boolean)
    .sort();
}

function requireExactTaggedFile(root, ref, relative, seen) {
  const expected = taggedBlob(root, ref, relative);
  const previous = seen.get(relative);
  if (previous && !previous.equals(expected))
    throw new Error(`${relative}: published tags disagree on immutable bytes`);
  seen.set(relative, expected);
  let actual;
  try {
    actual = regularBytes(root, relative);
  } catch {
    throw new Error(`${relative}: published tagged release root is missing`);
  }
  if (!expected.equals(actual))
    throw new Error(`${relative}: published tagged release bytes changed`);
  return expected;
}

function anchoredPublishedSetRefs(root) {
  // The tracked public main is a second local anchor. It detects deleting a
  // newer published set ref while leaving its previously published evidence
  // tree in the fetched public source. The deploy preflight must refresh and
  // compare against authoritative origin; this offline check cannot do that.
  const publicRef = "refs/remotes/origin/main";
  const abandoned = JSON.parse(
    taggedBlob(
      root,
      publicRef,
      "forms/trust/abandoned-prepublication.json",
    ).toString("utf8"),
  );
  const checkpointPaths = taggedPaths(
    root,
    publicRef,
    "forms/trust/sets",
  ).filter((relative) =>
    /^forms\/trust\/sets\/[0-9a-f]{40}\/revocations\/checkpoint\.sigstore\.json$/u.test(
      relative,
    ),
  );
  return new Set(
    checkpointPaths
      .map((relative) => `refs/tags/forms/sets/${relative.split("/")[3]}`)
      .filter((ref) => !ref.endsWith(`/${abandoned.setId}`)),
  );
}

// Local Git set/package tags plus the fetched public-main evidence tree, not
// the mutable current retained manifest, anchor published closures. This is
// an offline byte/identity guard, not proof that an arbitrary local tag is
// trusted or that cached origin/main is fresh. The publication gate verifies
// trust and compares exact refs with authoritative public origin.
export function verifyPublishedSetHistory(root) {
  const setRefs = tagRefs(root, "forms/sets/");
  const genesis = `refs/tags/forms/sets/${PUBLISHED_BASELINE_SOURCE}`;
  if (!setRefs.includes(genesis))
    throw new Error("published genesis set tag is unavailable locally");
  for (const ref of anchoredPublishedSetRefs(root))
    if (!setRefs.includes(ref))
      throw new Error(
        `${ref}: previously published set tag is unavailable locally`,
      );
  const allTagRefs = tagRefs(root, "forms/");
  const packageRefs = allTagRefs.filter((ref) =>
    /^refs\/tags\/forms\/[^/]+\/sha256-[0-9a-f]{64}$/u.test(ref),
  );
  const packageRefSet = new Set(packageRefs);
  const releaseFiles = new Map();
  const publishedDefinitions = new Map();
  const expectedPackageRefs = new Set();

  for (const setRef of setRefs) {
    const setId = setRef.slice("refs/tags/forms/sets/".length);
    if (!/^[0-9a-f]{40}$/u.test(setId))
      throw new Error(`invalid published set tag ${setRef}`);
    for (const relative of taggedPaths(root, setRef, "forms/releases"))
      requireExactTaggedFile(root, setRef, relative, releaseFiles);
    const trustPrefix = `forms/trust/sets/${setId}`;
    const trustPaths = taggedPaths(root, setRef, trustPrefix);
    if (
      !trustPaths.includes(
        `${trustPrefix}/revocations/checkpoint.sigstore.json`,
      )
    )
      throw new Error(`${setRef}: published signed checkpoint is missing`);
    for (const relative of trustPaths)
      requireExactTaggedFile(root, setRef, relative, releaseFiles);
    for (const prefix of [
      "forms/candidates",
      "interfaces/candidates",
      "bindings/candidates",
    ])
      for (const relative of taggedPaths(root, setRef, prefix))
        if (relative.endsWith("/definition.json"))
          publishedDefinitions.set(
            `${setRef}:${relative}`,
            taggedBlob(root, setRef, relative),
          );

    const releaseIndexes = taggedPaths(root, setRef, "forms/releases").filter(
      (relative) => relative.endsWith("/package-index.json"),
    );
    const rootsByArtifact = new Map();
    for (const relative of releaseIndexes) {
      const parts = relative.split("/");
      const artifact = parts[3];
      const packageRef = `refs/tags/forms/${parts[2]}/${artifact}`;
      if (rootsByArtifact.has(artifact))
        throw new Error(`${setRef}: duplicate release artifact ${artifact}`);
      rootsByArtifact.set(artifact, packageRef);
    }
    const candidateSet = JSON.parse(
      taggedBlob(
        root,
        setRef,
        "forms/candidates/edge.forms.takoform.com/candidate-set.json",
      ).toString("utf8"),
    );
    if (!Array.isArray(candidateSet.forms) || candidateSet.forms.length !== 17)
      throw new Error(`${setRef}: invalid published current Form set`);
    for (const entry of candidateSet.forms) {
      const artifact = String(entry.packageDigest ?? "").replace(":", "-");
      const ref = rootsByArtifact.get(artifact);
      if (!ref)
        throw new Error(
          `${setRef}: published current package ${artifact} is missing`,
        );
      expectedPackageRefs.add(ref);
    }
    const retained = JSON.parse(
      taggedBlob(root, setRef, "forms/retained-packages.json").toString("utf8"),
    );
    if (!Array.isArray(retained.packages))
      throw new Error(`${setRef}: invalid published retained inventory`);
    for (const entry of retained.packages) {
      const ref = `refs/tags/${entry.tag}`;
      if (
        !rootsByArtifact.get(entry.artifactId) ||
        rootsByArtifact.get(entry.artifactId) !== ref
      )
        throw new Error(`${setRef}: published retained locator is missing`);
      expectedPackageRefs.add(ref);
    }
  }

  for (const ref of expectedPackageRefs)
    if (!packageRefSet.has(ref))
      throw new Error(`${ref}: published package tag is unavailable locally`);
  for (const ref of packageRefs) {
    const rootPath = `forms/releases/${ref.slice("refs/tags/forms/".length)}`;
    const paths = taggedPaths(root, ref, rootPath);
    if (!paths.includes(`${rootPath}/package-index.json`))
      throw new Error(`${ref}: published package tag lacks its package index`);
    for (const relative of paths)
      requireExactTaggedFile(root, ref, relative, releaseFiles);
  }

  const currentSet = JSON.parse(
    regularBytes(
      root,
      "forms/candidates/edge.forms.takoform.com/candidate-set.json",
    ).toString("utf8"),
  );
  const currentArtifacts = new Set(
    currentSet.forms.map((entry) =>
      String(entry.packageDigest ?? "").replace(":", "-"),
    ),
  );
  const retained = JSON.parse(
    regularBytes(root, "forms/retained-packages.json").toString("utf8"),
  );
  const retainedTags = new Set(
    retained.packages.map((entry) => `refs/tags/${entry.tag}`),
  );
  for (const ref of packageRefs) {
    const artifact = ref.split("/").at(-1);
    if (!currentArtifacts.has(artifact) && !retainedTags.has(ref))
      throw new Error(
        `${ref}: published package disappeared from current or retained inventory`,
      );
  }
  return publishedDefinitions;
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
  for (const [relative, bytes] of verifyPublishedSetHistory(root))
    published.set(relative, bytes);
  assertPublishedDefinitionsUnchanged(published, current);
  return baseline;
}
