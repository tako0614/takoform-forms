#!/usr/bin/env bun

// The Forms publisher has one mutation surface: the canonical public Git
// repository. Package and signed publisher-evidence identities are
// create-only. Signing happens in the separate manual OIDC preparation
// workflow; this entrypoint has no signer, private key, delete, retag, force,
// or retry path.

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
export const TRUST_SET_TAG_PREFIX = "forms/sets/";
export const PUBLISHER_REPOSITORY = `https://github.com/${REPOSITORY}`;
export const PUBLISHER_WORKFLOW = `${PUBLISHER_REPOSITORY}/.github/workflows/form-package-signing.yml`;
export const PUBLISHER_REF = "refs/heads/main";
export const PUBLISHER_IDENTITY = `${PUBLISHER_WORKFLOW}@${PUBLISHER_REF}`;
export const PUBLISHER_OIDC_ISSUER =
  "https://token.actions.githubusercontent.com";
export const TRUSTED_ROOT_DIGEST =
  "sha256:6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66";
export const GENESIS_DIGEST =
  "sha256:35c5c4cdc6cd6c4beaec8ba273091be10ae02c0d6f49861f97062fd59f9e8f66";
export const GENESIS_ENTRIES_DIGEST =
  "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945";

const commitPattern = /^[0-9a-f]{40}$/u;
const digestPattern = /^sha256:[0-9a-f]{64}$/u;

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export const DEPLOY_CONTRACT = Object.freeze({
  kind: "takos.deploy-contract@v2",
  surfaces: [
    {
      surface: RELEASE_SURFACE,
      target: `${REPOSITORY_URL}:main + forms/<release-id>/sha256-<digest> + forms/sets/<signed-source-commit>`,
      covers: [
        "forms/candidates/current-family-index.json",
        "forms/candidates/edge.forms.takoform.com",
        "forms/releases",
        "forms/trust",
        "cmd/form-package",
        "cmd/publisher-trust",
        "scripts/form-publication.mjs",
        "scripts/deploy.mjs",
        ".github/workflows/form-package-signing.yml",
      ],
      requiresScripts: ["check", "deploy"],
      requiresTools: ["git", "bun", "go"],
      requiresEnv: [],
      triggers: ["authority", "published-identity"],
      obligations: {
        provenance:
          "The one clean canonical main commit is gated once. Released Core v1.1.0 verifies all 16 exact canonical package-index subjects, the exact publisher policy and trusted root, every Sigstore v0.3 bundle, one signed API v1 revocation genesis, and every not-revoked decision. All evidence must report one protected-main publisher/source/workflow/build commit; package subjects and publisher verification code remain byte-exact from that signed commit through publication.",
        "post-conditions":
          "After one ordinary atomic non-force push, credential-free verification reads origin main, every Core-derived package tag, and the create-only forms/sets/<source-commit> tag; fetches the public set commit into fresh storage; compares every package byte; and reruns Core v1.1.0 package, publisher, signature, checkpoint, and revocation verification.",
        reversal:
          "Package tags, release paths, signed trust-set paths, and set tags are immutable and are never deleted, retagged, or overwritten. A bad publication cannot be rolled back in place; forward repair signs a new source commit and creates a new set, while changed package bytes also create a new Core-derived package identity.",
        "failure-handling":
          "The entrypoint prints bounded command diagnostics and stops before mutation whenever the signed trust closure, source identity, exact package closure, or tag state is uncertain. A failure during or after the single atomic push is reported as indeterminate; no local tags are created and there is no blind retry, cleanup, deletion, or force path.",
        "independent-review":
          "The non-authoring TASK-0042 independent architecture review examined this publisher authority boundary and identified this contract omission. Before any publication, a person or agent that did not author the release must review the exact signed source commit, trust-set verification report, immutable tag plan, and atomic refspecs; the operator retains the named reviewer and exact commit outside the repository, and neither the signing workflow nor a green gate substitutes for that review.",
        "no-overwrite":
          "Immediately before mutation, the set tag must be absent locally and remotely. Existing Core-derived package tags are reused only when their tagged package path is byte-identical to the Core-verified signed source; absent package tags are created once. The atomic push has no force, delete, or retag path.",
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
  if (
    args[0] !== RELEASE_SURFACE ||
    args[1] !== "--trust-set" ||
    !commitPattern.test(args[2] ?? "")
  )
    throw new Error(usage());
  const trustSet = args[2];
  if (args.length === 3) return { mode: "publish", trustSet };
  if (args.length === 4 && args[3] === "--dry-run") {
    return { mode: "dry-run", trustSet };
  }
  if (args.length === 4 && args[3] === "--verify") {
    return { mode: "verify", trustSet };
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
      const trust = readTrustSet(dependencies, plan, invocation.trustSet, {
        credentialFree: true,
      });
      const evidence = verifyPublicPublication(plan, trust, dependencies);
      outputJSON(dependencies, evidence);
      return 0;
    }
    const before = readPlan(dependencies);
    const beforeTrust = readTrustSet(
      dependencies,
      before,
      invocation.trustSet,
      { credentialFree: true },
    );
    const firstCommit = requireSourceIdentity(dependencies, {
      expectedRemoteCommit: beforeTrust.sourceCommit,
    });
    requireSignedSourceClosure(dependencies, beforeTrust, firstCommit);
    runOwnerGate(dependencies);
    const commit = requireSourceIdentity(dependencies, {
      expectedRemoteCommit: beforeTrust.sourceCommit,
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
    const afterTrust = readTrustSet(dependencies, after, invocation.trustSet, {
      credentialFree: true,
    });
    assertPlansEqual(before, after);
    assertTrustReportsEqual(beforeTrust, afterTrust);
    requireSignedSourceClosure(dependencies, afterTrust, commit);
    const missingPackageTags = requireCreateOnlyIdentities(
      dependencies,
      after,
      afterTrust,
    );

    if (invocation.mode === "dry-run") {
      outputJSON(
        dependencies,
        dryRunEvidence(after, afterTrust, commit, missingPackageTags),
      );
      return 0;
    }

    pushAll(dependencies, after, afterTrust, commit, missingPackageTags);
    const evidence = verifyPublicPublication(after, afterTrust, dependencies, {
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

function readTrustSet(
  dependencies,
  plan,
  trustSet,
  { credentialFree = false, repositoryRoot = root } = {},
) {
  let report;
  if (typeof dependencies.readTrustSet === "function") {
    report = dependencies.readTrustSet({
      plan,
      trustSet,
      credentialFree,
      repositoryRoot,
    });
  } else {
    const setPath = path.join(
      repositoryRoot,
      "forms",
      "trust",
      "sets",
      trustSet,
    );
    const result = runDependency(
      dependencies,
      "go",
      [
        "run",
        "./cmd/publisher-trust",
        "verify-set",
        "--repository",
        repositoryRoot,
        "--set",
        setPath,
      ],
      credentialFree,
    );
    if (result.exitCode !== 0) {
      throw new DeployBlocked(
        `Core v1.1.0 signed publisher-set verification failed${commandDetail(result) ? `:\n${commandDetail(result)}` : ""}`,
      );
    }
    try {
      report = JSON.parse(result.stdout);
    } catch {
      throw new DeployBlocked(
        "Core v1.1.0 signed publisher-set verifier returned non-JSON",
      );
    }
  }
  validateTrustReport(report, plan, trustSet);
  return report;
}

function validateTrustReport(report, plan, trustSet) {
  if (
    report === null ||
    typeof report !== "object" ||
    report.status !== "verified" ||
    report.coreVersion !== "v1.1.0" ||
    report.family !== plan.family ||
    report.setId !== trustSet ||
    report.setTag !== `${TRUST_SET_TAG_PREFIX}${trustSet}` ||
    report.sourceCommit !== trustSet ||
    report.workflowCommit !== trustSet ||
    report.buildConfigCommit !== trustSet ||
    report.publisherIdentity !== PUBLISHER_IDENTITY ||
    report.packageCount !== plan.formCount ||
    !Array.isArray(report.packages) ||
    report.packages.length !== plan.formCount ||
    report.checkpoint?.status !== "verified"
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} did not return the exact Core v1.1.0 publisher/package/checkpoint report`,
    );
  }
  const expectedTags = plan.forms.map((form) => form.locator.tag);
  const verifiedTags = report.packages.map((entry) => entry?.locator?.tag);
  if (expectedTags.join("\n") !== verifiedTags.join("\n")) {
    throw new DeployBlocked(
      `trust set ${trustSet} package identities differ from the publication plan`,
    );
  }
  const checkpointBundle = report.checkpoint.bundle;
  if (
    report.checkpoint.checkpointVersion !== "0.0.0" ||
    report.checkpoint.entryCount !== 0 ||
    report.checkpoint.pin?.checkpointApiVersion !==
      "trust.forms.takoform.com/v1" ||
    report.checkpoint.pin?.sequence !== 0 ||
    report.checkpoint.pin?.digest !== GENESIS_DIGEST ||
    report.checkpoint.pin?.entriesDigest !== GENESIS_ENTRIES_DIGEST ||
    !exactPublisherBundle(checkpointBundle, GENESIS_DIGEST, trustSet)
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} does not contain the exact signed Core API v1 genesis`,
    );
  }
  for (let index = 0; index < plan.forms.length; index += 1) {
    const form = plan.forms[index];
    const verified = report.packages[index];
    if (
      verified.packageDigest !== form.packageDigest ||
      !exactPublisherBundle(verified.bundle, form.packageDigest, trustSet)
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} has incomplete exact evidence for ${form.kind}`,
      );
    }
  }
}

function exactPublisherBundle(bundle, subjectDigest, sourceCommit) {
  return (
    bundle !== null &&
    typeof bundle === "object" &&
    bundle.status === "verified" &&
    bundle.subjectDigest === subjectDigest &&
    digestPattern.test(bundle.bundleDigest ?? "") &&
    bundle.trustedRootDigest === TRUSTED_ROOT_DIGEST &&
    bundle.oidcIssuer === PUBLISHER_OIDC_ISSUER &&
    bundle.sourceRepository === PUBLISHER_REPOSITORY &&
    bundle.workflow === PUBLISHER_WORKFLOW &&
    bundle.ref === PUBLISHER_REF &&
    bundle.publisherIdentity === PUBLISHER_IDENTITY &&
    bundle.sourceCommit === sourceCommit &&
    bundle.workflowCommit === sourceCommit &&
    bundle.buildConfigCommit === sourceCommit &&
    bundle.transparencyLogVerified === true &&
    bundle.transparencyLogThreshold === 1
  );
}

function requireSourceIdentity(
  dependencies,
  { credentialFree = false, expectedRemoteCommit } = {},
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
  const requiredRemote = expectedRemoteCommit ?? commit;
  if (remoteCommit !== requiredRemote) {
    throw new DeployBlocked(
      `origin main is ${remoteCommit || "<missing>"}, expected ${requiredRemote}`,
    );
  }
  return commit;
}

function requireSignedSourceClosure(dependencies, trust, currentCommit) {
  requireSuccess(
    dependencies,
    "git",
    ["cat-file", "-e", `${trust.sourceCommit}^{commit}`],
    `cannot resolve signed source commit ${trust.sourceCommit}`,
  );
  requireSuccess(
    dependencies,
    "git",
    ["merge-base", "--is-ancestor", trust.sourceCommit, currentCommit],
    `signed source commit ${trust.sourceCommit} is not an ancestor of ${currentCommit}`,
  );
  requireSuccess(
    dependencies,
    "git",
    [
      "diff",
      "--quiet",
      trust.sourceCommit,
      currentCommit,
      "--",
      "forms/releases",
      "forms/candidates/current-family-index.json",
      "forms/candidates/edge.forms.takoform.com",
    ],
    "package subjects changed after the Core-verified signing source",
  );
  requireSuccess(
    dependencies,
    "git",
    [
      "diff",
      "--quiet",
      trust.sourceCommit,
      currentCommit,
      "--",
      ".github/workflows/form-package-signing.yml",
      "cmd/form-package",
      "cmd/publisher-trust",
      "internal/publishertrust",
      "go.mod",
      "go.sum",
      "package.json",
      "scripts/check-boundary.mjs",
      "scripts/deploy.mjs",
      "scripts/form-publication.mjs",
      "forms/trust/publisher-policy.json",
      "forms/trust/trusted-root.json",
    ],
    "publisher trust verifier or authority inputs changed after the signed source",
  );
  const trustChanges = requireSuccess(
    dependencies,
    "git",
    [
      "diff",
      "--name-only",
      trust.sourceCommit,
      currentCommit,
      "--",
      "forms/trust",
    ],
    "cannot inspect publisher trust changes after the signed source",
  );
  const allowedSetRoot = `forms/trust/sets/${trust.setId}/`;
  for (const changed of trustChanges.split("\n").filter(Boolean)) {
    if (!changed.startsWith(allowedSetRoot)) {
      throw new DeployBlocked(
        `publisher trust path ${changed} changed outside signed set ${trust.setId}`,
      );
    }
  }
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

function requireCreateOnlyIdentities(dependencies, plan, trust) {
  const setTag = trust.setTag;
  const localSet = runDependency(dependencies, "git", [
    "show-ref",
    "--verify",
    "--quiet",
    `refs/tags/${setTag}`,
  ]);
  if (localSet.exitCode === 0) {
    throw new DeployBlocked(`local set tag ${setTag} already exists`);
  }
  if (localSet.exitCode !== 1) {
    throw new DeployBlocked(
      `cannot prove local set tag ${setTag} is absent${commandDetail(localSet) ? `:\n${commandDetail(localSet)}` : ""}`,
    );
  }
  const remoteSet = requireSuccess(
    dependencies,
    "git",
    [
      "ls-remote",
      "--tags",
      "origin",
      `refs/tags/${setTag}`,
      `refs/tags/${setTag}^{}`,
    ],
    `cannot inspect origin set tag ${setTag}`,
  );
  if (remoteSet !== "") {
    throw new DeployBlocked(`remote set tag ${setTag} already exists`);
  }

  const missingPackageTags = [];
  for (const form of plan.forms) {
    const tag = form.locator.tag;
    const local = runDependency(dependencies, "git", [
      "show-ref",
      "--verify",
      "--quiet",
      `refs/tags/${tag}`,
    ]);
    if (local.exitCode !== 0 && local.exitCode !== 1) {
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
    if (remote === "") {
      missingPackageTags.push(tag);
      continue;
    }
    const direct = parseRemoteRef(remote, `refs/tags/${tag}`);
    const peeled = parseRemoteRef(remote, `refs/tags/${tag}^{}`);
    const tagCommit = peeled || direct;
    if (!commitPattern.test(tagCommit ?? "")) {
      throw new DeployBlocked(
        `remote package tag ${tag} did not resolve to a commit`,
      );
    }
    requireSuccess(
      dependencies,
      "git",
      ["cat-file", "-e", `${tagCommit}^{commit}`],
      `cannot resolve existing remote package tag ${tag} commit ${tagCommit} locally`,
    );
    requireSuccess(
      dependencies,
      "git",
      [
        "diff",
        "--quiet",
        tagCommit,
        trust.sourceCommit,
        "--",
        form.locator.sourcePath,
      ],
      `existing remote package tag ${tag} does not contain the Core-verified package bytes`,
    );
  }
  return missingPackageTags;
}

function pushAll(dependencies, plan, trust, commit, missingPackageTags) {
  const refs = ["refs/heads/main:refs/heads/main"];
  const missing = new Set(missingPackageTags);
  for (const form of plan.forms) {
    if (missing.has(form.locator.tag)) {
      refs.push(`${trust.sourceCommit}:refs/tags/${form.locator.tag}`);
    }
  }
  refs.push(`${commit}:refs/tags/${trust.setTag}`);
  requireSuccess(
    dependencies,
    "git",
    ["push", "--atomic", "origin", ...refs],
    `push of main, ${missingPackageTags.length} new package tags, and signed set ${trust.setTag} did not complete cleanly`,
    true,
  );
}

/**
 * Read the public repository without credentials, check all expected refs, and
 * verify the fetched release tree through the same Core CLI used locally.
 */
export function verifyPublicPublication(
  plan,
  trust,
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

  const setTagOutput = requireSuccess(
    dependencies,
    "git",
    [
      "ls-remote",
      "--tags",
      "origin",
      `refs/tags/${trust.setTag}`,
      `refs/tags/${trust.setTag}^{}`,
    ],
    `cannot read public signed set tag ${trust.setTag}`,
    mutationStarted,
    true,
  );
  const setDirect = parseRemoteRef(setTagOutput, `refs/tags/${trust.setTag}`);
  const setPeeled = parseRemoteRef(
    setTagOutput,
    `refs/tags/${trust.setTag}^{}`,
  );
  if (setDirect !== localCommit && setPeeled !== localCommit) {
    throw new DeployBlocked(
      `public signed set tag ${trust.setTag} points to ${setPeeled || setDirect || "<missing>"}, expected ${localCommit}`,
      mutationStarted,
    );
  }

  const packageTagCommits = new Map();
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
    const tagCommit = peeled || direct;
    if (!commitPattern.test(tagCommit ?? "")) {
      throw new DeployBlocked(
        `public package tag ${form.locator.tag} did not resolve to a commit`,
        mutationStarted,
      );
    }
    packageTagCommits.set(form.locator.tag, tagCommit);
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
    requireSuccess(
      dependencies,
      "git",
      [
        "-C",
        temporary,
        "fetch",
        "--quiet",
        "--no-tags",
        "origin",
        ...plan.forms.map(
          (form) =>
            `refs/tags/${form.locator.tag}:refs/tags/${form.locator.tag}`,
        ),
      ],
      "cannot fetch the public package tags",
      mutationStarted,
      true,
    );
    try {
      for (const form of plan.forms) {
        const fetchedTagCommit = requireSuccess(
          dependencies,
          "git",
          [
            "-C",
            temporary,
            "rev-parse",
            `refs/tags/${form.locator.tag}^{commit}`,
          ],
          `cannot resolve fetched public package tag ${form.locator.tag}`,
          mutationStarted,
          true,
        );
        if (fetchedTagCommit !== packageTagCommits.get(form.locator.tag)) {
          throw new DeployBlocked(
            `public package tag ${form.locator.tag} changed during readback`,
            mutationStarted,
          );
        }
        requireSuccess(
          dependencies,
          "git",
          [
            "-C",
            temporary,
            "diff",
            "--quiet",
            `refs/tags/${form.locator.tag}^{commit}`,
            localCommit,
            "--",
            form.locator.sourcePath,
          ],
          `public package tag ${form.locator.tag} does not contain the Core-verified package bytes`,
          mutationStarted,
          true,
        );
      }
      verifyFetchedReleaseTree(plan, temporary, dependencies, mutationStarted);
      const publicTrust = readTrustSet(dependencies, plan, trust.setId, {
        credentialFree: true,
        repositoryRoot: temporary,
      });
      assertTrustReportsEqual(trust, publicTrust);
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
    signedSet: {
      setId: trust.setId,
      tag: trust.setTag,
      publisherIdentity: trust.publisherIdentity,
      sourceCommit: trust.sourceCommit,
      workflowCommit: trust.workflowCommit,
      buildConfigCommit: trust.buildConfigCommit,
      checkpointPin: trust.checkpoint.pin,
    },
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => ({
      tag: form.locator.tag,
      commit: packageTagCommits.get(form.locator.tag),
      releaseId: form.locator.releaseId,
      artifactId: form.locator.artifactId,
      sourcePath: form.locator.sourcePath,
      packageDigest: form.packageDigest,
    })),
    postConditions: [
      "PUBLIC_MAIN_READBACK",
      "SIGNED_SET_TAG_READBACK",
      "ALL_16_TAGGED_PACKAGE_BYTES_READBACK",
      "FRESH_TREE_BYTE_COMPARISON",
      "CORE_V1_1_0_PACKAGE_TRUST_REVOCATION_VERIFICATION",
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
      `Core v1.1.0 verification failed for public package${commandDetail(result) ? `:\n${commandDetail(result)}` : ""}`,
      mutationStarted,
    );
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    throw new DeployBlocked(
      "Core v1.1.0 verifier returned non-JSON for a public package",
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

function assertTrustReportsEqual(before, after) {
  const stable = (report) =>
    JSON.stringify({
      status: report.status,
      coreVersion: report.coreVersion,
      family: report.family,
      setId: report.setId,
      setTag: report.setTag,
      packageCount: report.packageCount,
      publisherIdentity: report.publisherIdentity,
      sourceCommit: report.sourceCommit,
      workflowCommit: report.workflowCommit,
      buildConfigCommit: report.buildConfigCommit,
      checkpoint: report.checkpoint,
      packages: report.packages,
    });
  if (stable(before) !== stable(after)) {
    throw new DeployBlocked(
      "signed publisher evidence changed during verification",
    );
  }
}

function dryRunEvidence(plan, trust, commit, missingPackageTags) {
  return {
    kind: "takoform.form-package-publication-dry-run@v1",
    surface: RELEASE_SURFACE,
    repository: REPOSITORY_URL,
    commit,
    signedSet: {
      setId: trust.setId,
      tag: trust.setTag,
      sourceCommit: trust.sourceCommit,
      publisherIdentity: trust.publisherIdentity,
    },
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => form.locator.tag),
    packageTagsToCreate: missingPackageTags,
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
  return `usage: bun run deploy -- [--contract] | ${RELEASE_SURFACE} --trust-set <40-hex-source-commit> [--dry-run|--verify]`;
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
