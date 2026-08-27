#!/usr/bin/env bun

// The Forms publisher has one mutation surface: the canonical public Git
// repository. Package identities are content-addressed and create-only. This
// entrypoint intentionally has no GitHub Release, registry, signing, delete,
// retag, force, or retry path.

import { spawnSync } from "node:child_process";
import {
  existsSync,
  lstatSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  credentialFreeEnvironment,
  derivePublicationPlan,
} from "./form-publication.mjs";

export const RELEASE_SURFACE = "form-packages-edge";
export const REPOSITORY_URL = "https://github.com/tako0614/takoform-forms.git";
export const REPOSITORY = "tako0614/takoform-forms";
export const OWNER_GATE = "bun run check";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export const DEPLOY_CONTRACT = Object.freeze({
  kind: "takos.deploy-contract@v2",
  surfaces: [
    {
      surface: RELEASE_SURFACE,
      target: `${REPOSITORY_URL}:main + forms/<release-id>/sha256-<digest>`,
      covers: [
        "forms/candidates/current-family-index.json",
        "forms/candidates/edge.forms.takoform.com",
        "forms/releases",
        "cmd/form-package",
        "scripts/form-publication.mjs",
        "scripts/deploy.mjs",
      ],
      requiresScripts: ["check", "deploy"],
      requiresTools: ["git", "bun", "go"],
      requiresEnv: [],
      triggers: ["published-identity"],
      obligations: {
        provenance:
          "The one clean canonical main commit is gated once, all 16 candidate closures are verified by released Core v1.0.1, and each unsigned Git tag and forms/releases path is derived by Core PublicationLocatorFor from the exact FormRef and package digest. Candidate source is not publication evidence.",
        "post-conditions":
          "After one ordinary non-force push of main and the 16 exact tags, credential-free verification reads origin main and every tag, fetches the public commit into fresh temporary storage, compares every release path byte-for-byte, and reruns Core v1.0.1 package verification for all 16 packages.",
        reversal:
          "Tags and release paths are immutable and are never deleted, retagged, or overwritten. A bad publication cannot be rolled back in place; forward-repair uses a changed package digest and therefore a new Core-derived tag/path while the prior identity remains readable.",
        "failure-handling":
          "The entrypoint prints bounded command diagnostics and stops before mutation whenever source identity, exact candidate closure, or remote tag absence is uncertain. A failure during or after the single atomic push is reported as indeterminate; no local tags are created and there is no blind retry, cleanup, deletion, or force path.",
        "no-overwrite":
          "Immediately before mutation, every expected local and remote tag is checked and any existing tag is refused even when it already points at the expected commit. The push carries main and all 16 tags once without force; no overwrite or retag operation exists.",
      },
    },
  ],
});

class DeployBlocked extends Error {
  constructor(message, mutationStarted = false) {
    super(message);
    this.mutationStarted = mutationStarted;
  }
}

/**
 * Dependency seam used by focused tests. The real implementation is
 * credential-free for --verify; publication keeps the operator's normal Git
 * credential helper available without interpolating credentials into args.
 */
export function defaultDependencies() {
  return {
    readPlan() {
      return derivePublicationPlan();
    },
    run(command, args) {
      const result = spawnSync(command, args, {
        cwd: root,
        encoding: "utf8",
        maxBuffer: 64 * 1024 * 1024,
        stdio: ["ignore", "pipe", "pipe"],
        // Mutation uses the operator's normal local Git authentication and
        // credential helper. No credential is ever interpolated into args or
        // output; the credential-free route below is used by --verify.
        env: process.env,
      });
      return {
        exitCode: result.status ?? 1,
        stdout: result.stdout ?? "",
        stderr:
          (result.stderr ?? "") +
          (result.error ? `${result.error.message}\n` : ""),
      };
    },
    runReadOnly(command, args) {
      const invocation = credentialFreeInvocation(command, args);
      const result = spawnSync(invocation.command, invocation.args, {
        cwd: root,
        encoding: "utf8",
        maxBuffer: 64 * 1024 * 1024,
        stdio: ["ignore", "pipe", "pipe"],
        env: invocation.env,
      });
      return {
        exitCode: result.status ?? 1,
        stdout: result.stdout ?? "",
        stderr:
          (result.stderr ?? "") +
          (result.error ? `${result.error.message}\n` : ""),
      };
    },
    stdout(value) {
      process.stdout.write(value);
    },
    stderr(value) {
      process.stderr.write(value);
    },
  };
}

export function parseDeployInvocation(args) {
  if (!Array.isArray(args)) throw new Error(usage());
  if (args.length === 1 && args[0] === "--contract") {
    return { mode: "contract" };
  }
  if (args[0] !== RELEASE_SURFACE) throw new Error(usage());
  if (args.length === 1) return { mode: "publish" };
  if (args.length === 2 && args[1] === "--dry-run") {
    return { mode: "dry-run" };
  }
  if (args.length === 2 && args[1] === "--verify") {
    return { mode: "verify" };
  }
  throw new Error(usage());
}

export function runDeploy(args, dependencies = defaultDependencies()) {
  let invocation;
  try {
    invocation = parseDeployInvocation(args);
  } catch (error) {
    dependencies.stderr(`${error.message}\n`);
    return 2;
  }
  if (invocation.mode === "contract") {
    dependencies.stdout(`${JSON.stringify(DEPLOY_CONTRACT, null, 2)}\n`);
    return 0;
  }

  try {
    if (invocation.mode === "verify") {
      const plan = readPlan(dependencies);
      const evidence = verifyPublicPublication(plan, dependencies);
      outputJSON(dependencies, evidence);
      return 0;
    }
    const before = readPlan(dependencies);
    const firstCommit = requireSourceIdentity(dependencies, {
      allowEmptyOrigin: true,
    });
    runOwnerGate(dependencies);
    const commit = requireSourceIdentity(dependencies, {
      allowEmptyOrigin: true,
    });
    if (commit !== firstCommit) {
      throw new DeployBlocked(
        `source commit changed during the owner gate: ${firstCommit} -> ${commit}`,
      );
    }
    // The owner gate includes this check, but the publication route explicitly
    // re-runs it after the gate and compares the exact tag/path set used below.
    requireSuccess(
      dependencies,
      "bun",
      ["run", "check:publication"],
      "post-gate Form Package publication check",
    );
    const after = readPlan(dependencies);
    assertPlansEqual(before, after);
    requireNoExistingIdentities(dependencies, after);

    if (invocation.mode === "dry-run") {
      outputJSON(dependencies, dryRunEvidence(after, commit));
      return 0;
    }

    pushAll(dependencies, after, commit);
    const evidence = verifyPublicPublication(after, dependencies, {
      expectedCommit: commit,
      mutationStarted: true,
    });
    outputJSON(dependencies, {
      ...evidence,
      status: "PUBLISHED",
    });
    return 0;
  } catch (error) {
    const blocked =
      error instanceof DeployBlocked
        ? error
        : new DeployBlocked(
            error instanceof Error ? error.message : String(error),
          );
    const prefix = blocked.mutationStarted
      ? "deploy failed after publication mutation started: publication is indeterminate"
      : "deploy blocked";
    dependencies.stderr(`${prefix}: ${blocked.message}\n`);
    if (blocked.mutationStarted) {
      dependencies.stderr(
        "inspect origin main, every expected tag, and the public release paths before any retry; do not overwrite or delete an identity\n",
      );
    }
    return 1;
  }
}

function readPlan(dependencies) {
  return typeof dependencies.readPlan === "function"
    ? dependencies.readPlan()
    : derivePublicationPlan();
}

function requireSourceIdentity(
  dependencies,
  { credentialFree = false, allowEmptyOrigin = false } = {},
) {
  const status = requireSuccess(
    dependencies,
    "git",
    ["status", "--porcelain=v1", "--untracked-files=all"],
    "cannot inspect the worktree",
    false,
    credentialFree,
  );
  if (status !== "") {
    throw new DeployBlocked(
      `the worktree is not clean; release bytes must belong to one commit:\n${status}`,
    );
  }
  const branch = requireSuccess(
    dependencies,
    "git",
    ["symbolic-ref", "--quiet", "--short", "HEAD"],
    "cannot determine the current branch",
    false,
    credentialFree,
  );
  if (branch !== "main") {
    throw new DeployBlocked(
      `release publication requires main, found ${branch || "<empty>"}`,
    );
  }
  const remote = requireSuccess(
    dependencies,
    "git",
    ["remote", "get-url", "origin"],
    "cannot determine origin URL",
    false,
    credentialFree,
  );
  if (remote !== REPOSITORY_URL) {
    throw new DeployBlocked(
      `release publication requires canonical origin ${REPOSITORY_URL}, found ${remote || "<empty>"}`,
    );
  }
  const pushRemote = requireSuccess(
    dependencies,
    "git",
    ["remote", "get-url", "--push", "origin"],
    "cannot determine origin push URL",
    false,
    credentialFree,
  );
  if (pushRemote !== REPOSITORY_URL) {
    throw new DeployBlocked(
      `release publication requires canonical origin push URL ${REPOSITORY_URL}, found ${pushRemote || "<empty>"}`,
    );
  }
  const commit = requireSuccess(
    dependencies,
    "git",
    ["rev-parse", "HEAD"],
    "cannot resolve HEAD",
    false,
    credentialFree,
  );
  if (!/^[a-f0-9]{40}$/u.test(commit)) {
    throw new DeployBlocked(`HEAD resolved to an invalid commit id: ${commit}`);
  }
  const remoteMain = requireSuccess(
    dependencies,
    "git",
    ["ls-remote", "origin", "refs/heads/main"],
    "cannot read origin main",
    false,
    credentialFree,
  );
  const remoteCommit = parseRemoteRef(remoteMain, "refs/heads/main");
  if (remoteCommit === "" && allowEmptyOrigin) {
    const remoteRefs = requireSuccess(
      dependencies,
      "git",
      ["ls-remote", "--heads", "--tags", "origin"],
      "cannot prove that origin is empty before first publication",
      false,
      credentialFree,
    );
    if (remoteRefs !== "") {
      throw new DeployBlocked(
        "origin has refs but no main; first publication requires an empty repository",
      );
    }
    return commit;
  }
  if (remoteCommit !== commit) {
    throw new DeployBlocked(
      `local main ${commit} does not equal origin main ${remoteCommit || "<missing>"}`,
    );
  }
  return commit;
}

function runOwnerGate(dependencies) {
  // stdout is the deterministic machine-readable result channel. Gate
  // progress and its ordinary check output are diagnostics on stderr.
  dependencies.stderr(`==> ${OWNER_GATE}\n`);
  const gate = dependencies.run("bun", ["run", "check"]);
  if (gate.stdout) dependencies.stderr(gate.stdout);
  if (gate.stderr) dependencies.stderr(gate.stderr);
  if (gate.exitCode !== 0) throw new DeployBlocked(`${OWNER_GATE} failed`);
}

function requireNoExistingIdentities(dependencies, plan) {
  for (const form of plan.forms) {
    const tag = form.locator.tag;
    const local = runDependency(dependencies, "git", [
      "show-ref",
      "--verify",
      "--quiet",
      `refs/tags/${tag}`,
    ]);
    if (local.exitCode === 0) {
      throw new DeployBlocked(`local tag ${tag} already exists`);
    }
    if (local.exitCode !== 1) {
      throw new DeployBlocked(
        `cannot prove local tag ${tag} is absent${commandDetail(local) ? `:\n${commandDetail(local)}` : ""}`,
      );
    }
    const remote = requireSuccess(
      dependencies,
      "git",
      [
        "ls-remote",
        "--tags",
        "origin",
        `refs/tags/${tag}`,
        `refs/tags/${tag}^{}`,
      ],
      `cannot inspect origin tag ${tag}`,
    );
    if (remote !== "") {
      throw new DeployBlocked(`remote tag ${tag} already exists`);
    }
  }
}

function pushAll(dependencies, plan, commit) {
  const refs = ["refs/heads/main:refs/heads/main"];
  for (const form of plan.forms) {
    // A <commit>:<tag-ref> refspec creates a lightweight unsigned tag on the
    // remote without first creating a partially complete local tag set.
    refs.push(`${commit}:refs/tags/${form.locator.tag}`);
  }
  requireSuccess(
    dependencies,
    "git",
    ["push", "--atomic", "origin", ...refs],
    `push of main and ${plan.formCount} Form Package tags did not complete cleanly`,
    true,
  );
}

/**
 * Read the public repository without credentials, check all expected refs, and
 * verify the fetched release tree through the same Core CLI used locally.
 */
export function verifyPublicPublication(
  plan,
  dependencies,
  { expectedCommit, mutationStarted = false } = {},
) {
  const localCommit =
    expectedCommit ??
    requireSourceIdentity(dependencies, { credentialFree: true });
  const remoteMain = requireSuccess(
    dependencies,
    "git",
    ["ls-remote", "origin", "refs/heads/main"],
    "cannot read public main",
    mutationStarted,
    true,
  );
  const publicCommit = parseRemoteRef(remoteMain, "refs/heads/main");
  if (publicCommit !== localCommit) {
    throw new DeployBlocked(
      `public main is ${publicCommit || "<missing>"}, expected ${localCommit}`,
      mutationStarted,
    );
  }

  for (const form of plan.forms) {
    const tagOutput = requireSuccess(
      dependencies,
      "git",
      [
        "ls-remote",
        "--tags",
        "origin",
        `refs/tags/${form.locator.tag}`,
        `refs/tags/${form.locator.tag}^{}`,
      ],
      `cannot read public tag ${form.locator.tag}`,
      mutationStarted,
      true,
    );
    const direct = parseRemoteRef(tagOutput, `refs/tags/${form.locator.tag}`);
    const peeled = parseRemoteRef(
      tagOutput,
      `refs/tags/${form.locator.tag}^{}`,
    );
    if (direct !== localCommit && peeled !== localCommit) {
      throw new DeployBlocked(
        `public tag ${form.locator.tag} points to ${direct || "<missing>"}, expected ${localCommit}`,
        mutationStarted,
      );
    }
  }

  let temporary;
  try {
    temporary = mkdtempSync(path.join(tmpdir(), "takoform-public-verify-"));
    requireSuccess(
      dependencies,
      "git",
      [
        "clone",
        "--quiet",
        "--no-checkout",
        "--depth=1",
        "--branch",
        "main",
        REPOSITORY_URL,
        temporary,
      ],
      "cannot fetch the public main commit",
      mutationStarted,
      true,
    );
    const fetchedCommit = requireSuccess(
      dependencies,
      "git",
      ["-C", temporary, "rev-parse", "HEAD"],
      "cannot resolve fetched public commit",
      mutationStarted,
      true,
    );
    if (fetchedCommit !== localCommit) {
      throw new DeployBlocked(
        `fetched public commit is ${fetchedCommit}, expected ${localCommit}`,
        mutationStarted,
      );
    }
    requireSuccess(
      dependencies,
      "git",
      ["-C", temporary, "checkout", "--quiet", "--detach", localCommit],
      "cannot materialize fetched public paths",
      mutationStarted,
      true,
    );
    try {
      verifyFetchedReleaseTree(plan, temporary, dependencies, mutationStarted);
    } catch (error) {
      if (error instanceof DeployBlocked) throw error;
      throw new DeployBlocked(
        error instanceof Error ? error.message : String(error),
        mutationStarted,
      );
    }
  } catch (error) {
    if (error instanceof DeployBlocked) throw error;
    throw new DeployBlocked(
      error instanceof Error ? error.message : String(error),
      mutationStarted,
    );
  } finally {
    if (temporary) rmSync(temporary, { recursive: true, force: true });
  }
  return {
    kind: "takoform.form-package-publication-verification@v1",
    surface: RELEASE_SURFACE,
    repository: REPOSITORY_URL,
    commit: localCommit,
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => ({
      tag: form.locator.tag,
      releaseId: form.locator.releaseId,
      artifactId: form.locator.artifactId,
      sourcePath: form.locator.sourcePath,
      packageDigest: form.packageDigest,
    })),
    postConditions: [
      "PUBLIC_MAIN_READBACK",
      "ALL_16_TAGS_READBACK",
      "FRESH_TREE_BYTE_COMPARISON",
      "CORE_V1_0_1_PACKAGE_VERIFICATION",
    ],
    status: "VERIFIED",
  };
}

function verifyFetchedReleaseTree(
  plan,
  fetchedRoot,
  dependencies,
  mutationStarted,
) {
  const localRoot = plan.repositoryRoot ?? root;
  const expected = new Map();
  for (const form of plan.forms) {
    const releaseRoot = path.join(
      fetchedRoot,
      ...form.locator.sourcePath.split("/"),
    );
    const candidateReleaseRoot = path.join(
      localRoot,
      ...form.locator.sourcePath.split("/"),
    );
    if (!existsSync(releaseRoot)) {
      throw new DeployBlocked(
        `${form.locator.sourcePath}: public release path is missing`,
        mutationStarted,
      );
    }
    for (const relative of inventoryRelativeFiles(releaseRoot)) {
      expected.set(
        `${form.locator.sourcePath}/${relative}`,
        path.join(releaseRoot, relative),
      );
    }
    const localFiles = inventoryRelativeFiles(candidateReleaseRoot);
    if (localFiles.length !== inventoryRelativeFiles(releaseRoot).length) {
      throw new DeployBlocked(
        `${form.kind}: public release closure file count differs from local`,
        mutationStarted,
      );
    }
    for (const relative of localFiles) {
      const localPath = path.join(candidateReleaseRoot, relative);
      const publicPath = path.join(releaseRoot, relative);
      if (!existsSync(publicPath) || !bytesEqual(localPath, publicPath)) {
        throw new DeployBlocked(
          `${form.kind}: public release bytes differ at ${relative}`,
          mutationStarted,
        );
      }
    }
    const locator = runCoreLocatorForFetched(
      dependencies,
      releaseRoot,
      mutationStarted,
    );
    if (
      locator.apiVersion !== form.locator.apiVersion ||
      locator.releaseId !== form.locator.releaseId ||
      locator.artifactId !== form.locator.artifactId ||
      locator.tag !== form.locator.tag ||
      locator.sourcePath !== form.locator.sourcePath
    ) {
      throw new DeployBlocked(
        `${form.kind}: public release locator differs from candidate`,
        mutationStarted,
      );
    }
  }
  const allFiles = inventoryRelativeFiles(
    path.join(fetchedRoot, "forms", "releases"),
  );
  for (const relative of allFiles) {
    if (!expected.has(`forms/releases/${relative}`)) {
      throw new DeployBlocked(
        `forms/releases/${relative}: extra public release file`,
        mutationStarted,
      );
    }
  }
}

function runCoreLocatorForFetched(dependencies, packageRoot, mutationStarted) {
  const result = runDependency(
    dependencies,
    "go",
    ["run", "./cmd/form-package", "verify", packageRoot],
    true,
  );
  if (result.exitCode !== 0) {
    throw new DeployBlocked(
      `Core v1.0.1 verification failed for public package${commandDetail(result) ? `:\n${commandDetail(result)}` : ""}`,
      mutationStarted,
    );
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    throw new DeployBlocked(
      "Core v1.0.1 verifier returned non-JSON for a public package",
      mutationStarted,
    );
  }
}

function bytesEqual(left, right) {
  const a = readFileSync(left);
  const b = readFileSync(right);
  return a.length === b.length && a.equals(b);
}

function inventoryRelativeFiles(directory) {
  if (!existsSync(directory)) return [];
  const info = lstatSync(directory);
  if (!info.isDirectory())
    throw new Error(`${directory}: release root is not a directory`);
  const found = [];
  const walk = (current, prefix = "") => {
    for (const name of readdirSync(current).sort()) {
      const full = path.join(current, name);
      const relative = prefix ? `${prefix}/${name}` : name;
      const child = lstatSync(full);
      if (child.isSymbolicLink())
        throw new Error(`${relative}: symlink in public release tree`);
      if (child.isDirectory()) walk(full, relative);
      else if (child.isFile()) found.push(relative);
      else throw new Error(`${relative}: unsupported public release entry`);
    }
  };
  walk(directory);
  return found;
}

function assertPlansEqual(before, after) {
  const left = before.forms
    .map(
      (form) =>
        `${form.locator.tag}:${form.locator.sourcePath}:${form.packageDigest}`,
    )
    .join("\n");
  const right = after.forms
    .map(
      (form) =>
        `${form.locator.tag}:${form.locator.sourcePath}:${form.packageDigest}`,
    )
    .join("\n");
  if (left !== right)
    throw new DeployBlocked(
      "publication locator set changed during the owner gate",
    );
}

function dryRunEvidence(plan, commit) {
  return {
    kind: "takoform.form-package-publication-dry-run@v1",
    surface: RELEASE_SURFACE,
    repository: REPOSITORY_URL,
    commit,
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => form.locator.tag),
    status: "DRY_RUN_VERIFIED",
  };
}

function requireSuccess(
  dependencies,
  command,
  args,
  label,
  mutationStarted = false,
  credentialFree = false,
) {
  const result = runDependency(dependencies, command, args, credentialFree);
  if (result.exitCode !== 0) {
    throw new DeployBlocked(
      `${label} failed${commandDetail(result) ? `:\n${commandDetail(result)}` : ""}`,
      mutationStarted,
    );
  }
  return result.stdout.trim();
}

function runDependency(dependencies, command, args, credentialFree = false) {
  if (credentialFree && typeof dependencies.runReadOnly === "function") {
    return dependencies.runReadOnly(command, args);
  }
  return dependencies.run(command, args);
}

function commandDetail(result) {
  return [result.stdout, result.stderr]
    .map((value) => String(value ?? "").trim())
    .filter(Boolean)
    .join("\n");
}

function parseRemoteRef(output, ref) {
  for (const line of String(output ?? "").split(/\r?\n/u)) {
    const parts = line.trim().split(/\s+/u);
    if (parts[1] === ref && parts[0]) return parts[0];
  }
  return "";
}

function outputJSON(dependencies, value) {
  dependencies.stdout(`${JSON.stringify(value, null, 2)}\n`);
}

function usage() {
  return `usage: bun run deploy -- [--contract] | ${RELEASE_SURFACE} [--dry-run|--verify]`;
}

export function credentialFreeInvocation(
  command,
  args,
  sourceEnvironment = process.env,
) {
  return {
    command,
    args:
      command === "git"
        ? ["-c", "credential.helper=", "-c", "http.extraHeader=", ...args]
        : [...args],
    env: credentialFreeEnvironment(sourceEnvironment),
  };
}

if (import.meta.main) {
  process.exitCode = runDeploy(process.argv.slice(2));
}
