#!/usr/bin/env node

// Publisher-local, append-only source identities for authored Edge Forms.
// A frozen source identity is not evidence that it was publicly published.

import { createHash } from "node:crypto";
import { lstatSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, posix, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { loadEdgeV2Docs } from "./edge-v2-docs.mjs";

export const FREEZE_PATH = "spec/forms.freeze.json";
export const FREEZE_KIND = "edge.forms.freeze";
export const FORMS_ORIGIN = "https://edge.forms.takoform.com";
const FORM_ROUTE = /^\/forms\/([A-Za-z][A-Za-z0-9]*)\/(\d+\.\d+\.\d+)\/$/u;
const SHA256 = /^sha256:[0-9a-f]{64}$/u;
const MANIFEST_KEYS = ["forms", "kind"];
const ENTRY_KEYS = ["path", "sha256", "url"];

function sha256(bytes) {
  return `sha256:${createHash("sha256").update(bytes).digest("hex")}`;
}

function sameArray(actual, expected) {
  return (
    Array.isArray(actual) &&
    actual.length === expected.length &&
    actual.every((value, index) => value === expected[index])
  );
}

function sameKeys(value, expected) {
  return (
    value !== null &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    sameArray(Object.keys(value).sort(), expected)
  );
}

function safeRepositoryPath(value) {
  return (
    typeof value === "string" &&
    value !== "" &&
    !value.startsWith("/") &&
    !value.includes("\\") &&
    posix.normalize(value) === value &&
    value !== ".." &&
    !value.startsWith("../")
  );
}

function runGit(root, args, { bytes = false, allowFailure = false } = {}) {
  const result = spawnSync("git", ["--no-replace-objects", ...args], {
    cwd: root,
    encoding: bytes ? null : "utf8",
    maxBuffer: 16 * 1024 * 1024,
    env: { ...process.env, GIT_NO_REPLACE_OBJECTS: "1" },
  });
  if (result.error) throw result.error;
  if (result.status !== 0 && !allowFailure) {
    const stderr = bytes ? result.stderr.toString("utf8") : result.stderr;
    throw new Error(
      stderr.trim() || `git ${args[0]} failed with status ${result.status}`,
    );
  }
  return result;
}

function repositoryAccessor(root) {
  return {
    read(path) {
      if (!safeRepositoryPath(path)) throw new Error("unsafe repository path");
      return readFileSync(resolve(root, path));
    },
  };
}

function commitAccessor(root, commit) {
  return {
    read(path) {
      if (!safeRepositoryPath(path)) throw new Error("unsafe repository path");
      return runGit(root, ["show", `${commit}:${path}`], { bytes: true })
        .stdout;
    },
  };
}

function parseManifest(bytes, problems) {
  try {
    return JSON.parse(bytes.toString("utf8"));
  } catch (error) {
    problems.push(`${FREEZE_PATH} is not valid JSON: ${error.message}`);
    return null;
  }
}

function validateEntryShape(entry, problems) {
  if (!sameKeys(entry, ENTRY_KEYS)) {
    problems.push(
      `${FREEZE_PATH} contains a Form entry outside the exact url/path/sha256 shape`,
    );
    return false;
  }
  if (
    !safeRepositoryPath(entry.path) ||
    !/^spec\/forms\/[A-Za-z][A-Za-z0-9]*\/\d+\.\d+\.\d+\/index\.md$/u.test(
      entry.path,
    )
  ) {
    problems.push(
      `${FREEZE_PATH} contains an invalid authored Form source path`,
    );
    return false;
  }
  if (!SHA256.test(entry.sha256 ?? "")) {
    problems.push(`${FREEZE_PATH} contains an invalid Form source sha256`);
    return false;
  }
  if (typeof entry.url !== "string") {
    problems.push(`${FREEZE_PATH} contains an invalid canonical Form URL`);
    return false;
  }
  let url;
  try {
    url = new URL(entry.url);
  } catch {
    problems.push(`${FREEZE_PATH} contains an invalid canonical Form URL`);
    return false;
  }
  const route = FORM_ROUTE.exec(url.pathname);
  if (
    url.origin !== FORMS_ORIGIN ||
    url.search ||
    url.hash ||
    !route ||
    entry.url !== `${FORMS_ORIGIN}/forms/${route[1]}/${route[2]}/` ||
    entry.path !== `spec/forms/${route[1]}/${route[2]}/index.md`
  ) {
    problems.push(
      `${FREEZE_PATH} Form URL and source path are not the same canonical identity`,
    );
    return false;
  }
  return true;
}

function validateManifest(manifest, problems) {
  if (!sameKeys(manifest, MANIFEST_KEYS)) {
    problems.push(
      `${FREEZE_PATH} has fields outside the append-only Form freeze shape`,
    );
    return false;
  }
  if (manifest.kind !== FREEZE_KIND)
    problems.push(`${FREEZE_PATH} kind must be ${FREEZE_KIND}`);
  if (!Array.isArray(manifest.forms)) {
    problems.push(`${FREEZE_PATH} forms must be an array`);
    return false;
  }
  if (manifest.forms.length === 0)
    problems.push(
      `${FREEZE_PATH} forms must contain at least one frozen Form URL`,
    );
  const urls = new Set();
  const paths = new Set();
  for (const entry of manifest.forms) {
    if (!validateEntryShape(entry, problems)) continue;
    if (urls.has(entry.url))
      problems.push(`${FREEZE_PATH} repeats Form URL ${entry.url}`);
    if (paths.has(entry.path))
      problems.push(`${FREEZE_PATH} repeats Form source ${entry.path}`);
    urls.add(entry.url);
    paths.add(entry.path);
  }
  return true;
}

function inspectManifest(accessor) {
  const problems = [];
  let manifestBytes;
  try {
    manifestBytes = accessor.read(FREEZE_PATH);
  } catch {
    return { present: false, manifest: null, manifestBytes: null, problems };
  }
  const manifest = parseManifest(manifestBytes, problems);
  if (manifest === null) {
    if (problems.length === 0)
      problems.push(`${FREEZE_PATH} must be a JSON object`);
    return { present: true, manifest: null, manifestBytes, problems };
  }
  validateManifest(manifest, problems);
  return { present: true, manifest, manifestBytes, problems };
}

function inspectManifestSources(accessor, manifest, problems) {
  if (!Array.isArray(manifest?.forms)) return;
  for (const entry of manifest.forms) {
    if (!validateEntryShape(entry, problems)) continue;
    let bytes;
    try {
      bytes = accessor.read(entry.path);
    } catch {
      problems.push(`frozen Form source is missing: ${entry.path}`);
      continue;
    }
    if (sha256(bytes) !== entry.sha256)
      problems.push(
        `frozen Form source differs from its recorded sha256: ${entry.url}`,
      );
  }
}

/** Derive all currently authored Forms; guides and package candidates are excluded. */
export function deriveEdgeFormFreezeCandidates(root) {
  return loadEdgeV2Docs(root)
    .filter((entry) => entry.kind === "form")
    .map((entry) => {
      const bytes = readFileSync(resolve(root, entry.file));
      return {
        url: `${FORMS_ORIGIN}${entry.route}`,
        path: entry.file,
        sha256: sha256(bytes),
      };
    })
    .sort((left, right) => left.url.localeCompare(right.url));
}

function repositoryHistory(root) {
  const inside = runGit(root, ["rev-parse", "--is-inside-work-tree"], {
    allowFailure: true,
  });
  if (inside.status !== 0 || inside.stdout.trim() !== "true")
    return { problem: `${root} is not a Git worktree` };
  const graftPathResult = runGit(root, [
    "rev-parse",
    "--git-path",
    "info/grafts",
  ]);
  const graftPathText = graftPathResult.stdout.trim();
  if (graftPathText === "")
    return {
      problem:
        "cannot resolve the Git graft path; Form identity provenance is unknown",
    };
  try {
    lstatSync(resolve(root, graftPathText));
    return {
      problem:
        "legacy Git graft file exists; cannot prove Form identity history",
    };
  } catch (error) {
    if (error.code !== "ENOENT")
      return {
        problem: `cannot inspect Git graft path; Form identity provenance is unknown: ${error.message}`,
      };
  }
  const shallow =
    runGit(root, ["rev-parse", "--is-shallow-repository"]).stdout.trim() ===
    "true";
  const addLog = runGit(
    root,
    [
      "log",
      "--full-history",
      "--reverse",
      "--format=%H",
      "--diff-filter=A",
      "--root",
      "HEAD",
      "--",
      FREEZE_PATH,
    ],
    { allowFailure: true },
  );
  const deleteLog = runGit(
    root,
    [
      "log",
      "--full-history",
      "--format=%H",
      "--diff-filter=D",
      "--root",
      "HEAD",
      "--",
      FREEZE_PATH,
    ],
    { allowFailure: true },
  );
  if (addLog.status !== 0 || deleteLog.status !== 0) {
    const diagnostic = [addLog.stderr, deleteLog.stderr]
      .filter(Boolean)
      .join("\n")
      .trim();
    return {
      problem: `cannot inspect Form freeze add/delete history${diagnostic ? `: ${diagnostic}` : ""}`,
    };
  }
  const additions = addLog.stdout.trim().split(/\s+/u).filter(Boolean);
  const deletions = deleteLog.stdout.trim().split(/\s+/u).filter(Boolean);
  const atHead =
    runGit(root, ["cat-file", "-e", `HEAD:${FREEZE_PATH}`], {
      allowFailure: true,
    }).status === 0;
  return {
    shallow,
    additions,
    deletions,
    firstAddCommit: additions[0] ?? null,
    atHead,
  };
}

function manifestSnapshot(root, commit) {
  const accessor = commitAccessor(root, commit);
  const inspected = inspectManifest(accessor);
  const problems = [...inspected.problems];
  if (!inspected.present)
    problems.push(`${FREEZE_PATH} is missing at ${commit}`);
  if (inspected.manifest !== null)
    inspectManifestSources(accessor, inspected.manifest, problems);
  return { ...inspected, problems: [...new Set(problems)] };
}

function sameEntry(left, right) {
  return (
    left?.url === right?.url &&
    left?.path === right?.path &&
    left?.sha256 === right?.sha256
  );
}

function verifyAppendOnlyHistory(root, history, problems) {
  if (history.deletions.length !== 0) {
    problems.push(
      `${FREEZE_PATH} was deleted in Git history; deletion and re-addition are forbidden`,
    );
    return;
  }
  if (history.additions.length !== 1) {
    problems.push(
      `${FREEZE_PATH} must have exactly one first-add commit; found ${history.additions.length}`,
    );
    return;
  }
  const ancestor = runGit(
    root,
    ["merge-base", "--is-ancestor", history.firstAddCommit, "HEAD"],
    { allowFailure: true },
  );
  if (ancestor.status !== 0) {
    problems.push(
      "first-add commit is not an ancestor of HEAD; provenance is unknown",
    );
    return;
  }
  const firstSnapshot = manifestSnapshot(root, history.firstAddCommit);
  problems.push(
    ...firstSnapshot.problems.map((problem) => `first-add: ${problem}`),
  );
  const descendantsResult = runGit(
    root,
    [
      "rev-list",
      "--ancestry-path",
      "--topo-order",
      "--reverse",
      "--parents",
      `${history.firstAddCommit}..HEAD`,
    ],
    { allowFailure: true },
  );
  if (descendantsResult.status !== 0) {
    problems.push(
      `cannot inspect Form source history: ${descendantsResult.stderr.trim()}`,
    );
    return;
  }
  const descendants = descendantsResult.stdout
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => line.split(/\s+/u));
  const descendantCommits = new Set(descendants.map(([commit]) => commit));
  const snapshots = new Map([[history.firstAddCommit, firstSnapshot]]);
  for (const [commit, ...parents] of descendants) {
    const current = manifestSnapshot(root, commit);
    snapshots.set(commit, current);
    problems.push(
      ...current.problems.map((problem) => `history ${commit}: ${problem}`),
    );
    const nextForms = current.manifest?.forms;
    if (!Array.isArray(nextForms)) continue;
    for (const parent of parents) {
      if (parent !== history.firstAddCommit && !descendantCommits.has(parent))
        continue;
      const previousForms = snapshots.get(parent)?.manifest?.forms;
      if (!Array.isArray(previousForms)) continue;
      if (
        nextForms.length < previousForms.length ||
        previousForms.some(
          (entry, index) => !sameEntry(entry, nextForms[index]),
        )
      ) {
        problems.push(`${FREEZE_PATH} is not append-only at commit ${commit}`);
      }
    }
  }
}

function pendingForms(candidates, frozenForms) {
  const frozenUrls = new Set(
    (Array.isArray(frozenForms) ? frozenForms : []).map((entry) => entry.url),
  );
  return candidates.filter((entry) => !frozenUrls.has(entry.url));
}

/**
 * Verify publisher-local authored Form URL anchors. Bootstrap checks a candidate
 * without writing a manifest; frozen status does not imply public publication.
 */
export function verifyEdgeFormFreeze(
  root,
  { mode = "check", requireFrozen = false } = {},
) {
  if (mode !== "check" && mode !== "bootstrap")
    throw new Error(`unsupported mode: ${mode}`);
  if (typeof requireFrozen !== "boolean")
    throw new Error("requireFrozen must be a boolean");
  if (mode === "bootstrap" && requireFrozen)
    throw new Error("requireFrozen cannot be used with bootstrap mode");

  const candidates = deriveEdgeFormFreezeCandidates(root);
  const current = inspectManifest(repositoryAccessor(root));
  let history;
  try {
    history = repositoryHistory(root);
  } catch (error) {
    return {
      status: "INVALID",
      problems: [
        ...current.problems,
        `cannot inspect Git history: ${error.message}`,
      ],
      firstAddCommit: null,
      frozen: [],
      pending: candidates,
    };
  }
  const problems = [...current.problems];
  if (history.problem)
    return {
      status: "INVALID",
      problems: [...problems, history.problem],
      firstAddCommit: null,
      frozen: [],
      pending: candidates,
    };
  if (history.shallow)
    return {
      status: "INVALID",
      problems: [
        ...problems,
        "cannot prove first-add Form identity history from a shallow Git history",
      ],
      firstAddCommit: history.firstAddCommit,
      frozen: [],
      pending: candidates,
    };

  if (mode === "bootstrap") {
    if (history.firstAddCommit !== null || history.atHead)
      problems.push(
        "bootstrap is forbidden after the Form freeze manifest entered Git history",
      );
    if (!current.present)
      problems.push(
        `${FREEZE_PATH} candidate is missing; bootstrap validates but never writes it`,
      );
    if (current.present && current.manifest !== null)
      inspectManifestSources(
        repositoryAccessor(root),
        current.manifest,
        problems,
      );
    const frozenForms = Array.isArray(current.manifest?.forms)
      ? current.manifest.forms
      : [];
    return {
      status: problems.length === 0 ? "BOOTSTRAP_CANDIDATE" : "INVALID",
      problems: [...new Set(problems)],
      firstAddCommit: history.firstAddCommit,
      frozen: frozenForms,
      pending: pendingForms(candidates, frozenForms),
    };
  }

  if (history.firstAddCommit === null) {
    if (history.atHead || current.present) {
      problems.push(
        `${FREEZE_PATH} is present without a first-add commit; use bootstrap before committing it`,
      );
      return {
        status: "INVALID",
        problems: [...new Set(problems)],
        firstAddCommit: null,
        frozen: [],
        pending: candidates,
      };
    }
    if (requireFrozen)
      problems.push(
        `${FREEZE_PATH} is absent; this check requires frozen Edge Form URLs`,
      );
    return {
      status: problems.length === 0 ? "UNFROZEN" : "INVALID",
      problems: [...new Set(problems)],
      firstAddCommit: null,
      frozen: [],
      pending: candidates,
    };
  }
  if (!history.atHead || !current.present)
    problems.push(
      `${FREEZE_PATH} was first added at ${history.firstAddCommit} but is absent at HEAD`,
    );
  verifyAppendOnlyHistory(root, history, problems);
  if (current.manifest !== null)
    inspectManifestSources(
      repositoryAccessor(root),
      current.manifest,
      problems,
    );

  const frozenForms = Array.isArray(current.manifest?.forms)
    ? current.manifest.forms
    : [];
  if (history.atHead && current.manifestBytes !== null) {
    try {
      const headManifest = commitAccessor(root, "HEAD").read(FREEZE_PATH);
      if (!current.manifestBytes.equals(headManifest))
        problems.push(
          `${FREEZE_PATH} differs from HEAD; frozen URL anchors must be committed`,
        );
      for (const entry of frozenForms) {
        const currentBytes = repositoryAccessor(root).read(entry.path);
        const headBytes = commitAccessor(root, "HEAD").read(entry.path);
        if (!currentBytes.equals(headBytes))
          problems.push(
            `frozen Form source differs from HEAD; URL anchors must be committed: ${entry.url}`,
          );
      }
    } catch (error) {
      problems.push(
        `cannot compare frozen Form anchors with HEAD: ${error.message}`,
      );
    }
  }
  const candidatesByUrl = new Map(
    candidates.map((entry) => [entry.url, entry]),
  );
  for (const entry of frozenForms) {
    const candidate = candidatesByUrl.get(entry.url);
    if (!candidate || !sameEntry(entry, candidate))
      problems.push(
        `frozen Form URL is missing or changed in authored source: ${entry.url}`,
      );
    candidatesByUrl.delete(entry.url);
  }
  return {
    status: problems.length === 0 ? "FROZEN" : "INVALID",
    problems: [...new Set(problems)],
    firstAddCommit: history.firstAddCommit,
    frozen: frozenForms,
    pending: [...candidatesByUrl.values()],
  };
}

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const USAGE =
  "usage: bun scripts/edge-form-freeze.mjs [--check [--require-frozen]|--bootstrap]\n";

export function main(argv = process.argv.slice(2)) {
  let mode = "check";
  let requireFrozen = false;
  if (argv.length === 1 && argv[0] === "--bootstrap") mode = "bootstrap";
  else if (argv.length === 0 || (argv.length === 1 && argv[0] === "--check")) {
    mode = "check";
  } else if (
    argv.length === 2 &&
    argv[0] === "--check" &&
    argv[1] === "--require-frozen"
  ) {
    requireFrozen = true;
  } else {
    process.stderr.write(USAGE);
    process.exitCode = 1;
    return;
  }

  let result;
  try {
    result = verifyEdgeFormFreeze(ROOT, { mode, requireFrozen });
  } catch (error) {
    process.stderr.write(`edge-form-freeze: ${error.message}\n`);
    process.exitCode = 1;
    return;
  }
  if (result.problems.length !== 0) {
    for (const problem of result.problems)
      process.stderr.write(`edge-form-freeze: ${problem}\n`);
    process.exitCode = 1;
    return;
  }
  if (result.status === "UNFROZEN")
    process.stdout.write(
      "edge-form-freeze: UNFROZEN (no manifest has entered Git history)\n",
    );
  else if (result.status === "BOOTSTRAP_CANDIDATE")
    process.stdout.write(
      `edge-form-freeze: bootstrap candidate verified; no manifest was written (${result.frozen.length} frozen, ${result.pending.length} pending)\n`,
    );
  else
    process.stdout.write(
      `edge-form-freeze: Form URL anchors verified at first-add commit ${result.firstAddCommit} (${result.frozen.length} frozen, ${result.pending.length} pending)\n`,
    );
}

if (import.meta.main) main();
