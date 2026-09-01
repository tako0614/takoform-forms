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
  ABANDONED_PREPUBLICATION_SET_ID,
  ABANDONED_PREPUBLICATION_SET_TAG,
  credentialFreeEnvironment,
  derivePublicationPlan,
} from "./form-publication.mjs";

export const RELEASE_SURFACE = "form-packages-edge";
export const REPOSITORY_URL = "https://github.com/tako0614/takoform-forms.git";
export const REPOSITORY = "tako0614/takoform-forms";
export const OWNER_GATE = "bun run check";
export const TRUST_SET_TAG_PREFIX = "forms/sets/";
export const REVOCATION_TAG_PREFIX = "forms/revocations/v";
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
const semverPattern =
  /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/u;
const MAX_REVOCATION_SEQUENCE = 1024;

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export const DEPLOY_CONTRACT = Object.freeze({
  kind: "takos.deploy-contract@v2",
  surfaces: [
    {
      surface: RELEASE_SURFACE,
      target: `${REPOSITORY_URL}:main + forms/<release-id>/sha256-<digest> + forms/sets/<signed-source-commit> + forms/revocations/v<statement-version>`,
      covers: [
        "forms/candidates/current-family-index.json",
        "forms/candidates/edge.forms.takoform.com",
        "forms/releases",
        "forms/retained-packages.json",
        "forms/revocations",
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
          "The one clean canonical main commit is gated once. Released Core v1.1.0 verifies all 17 exact canonical package-index subjects, the exact publisher policy and trusted root, every Sigstore v0.3 bundle, the bounded signed API v1 checkpoint chain from genesis, every canonical statement digest, and every not-revoked decision. Publication separately verifies the exact local release inventory (17 current roots, two retained immutable roots, and three explicitly listed abandoned evidence-only roots) while only 19 current/retained roots receive publishable package tags. All new evidence reports one protected-main publisher/source/workflow/build commit; package subjects, revocation source, retained inventory, abandoned recovery manifest, and publisher verification code remain byte-exact from that signed commit through publication.",
        "post-conditions":
          "After one ordinary atomic non-force push, credential-free verification reads origin main, all 19 exact publishable release-root tags (17 current package tags plus two retained tags), the three untagged abandoned evidence-only roots from the recovery manifest, the create-only forms/sets/<source-commit> tag, and every immutable forms/revocations/v<statement-version> tag; fetches public commits into fresh storage; compares package and revocation bytes; and replays Core v1.1.0 package, publisher, signature, checkpoint, continuity, and revocation verification.",
        reversal:
          "Package tags, release paths, signed trust-set paths, set tags, revocation statements, checkpoints, and revocation tags are immutable and are never deleted, retagged, or overwritten. A bad publication cannot be rolled back in place; forward repair appends one new statement/checkpoint, signs a new source commit, and creates a new set, while changed package bytes also create a new Core-derived package identity.",
        "failure-handling":
          "The entrypoint prints bounded command diagnostics and stops before mutation whenever the signed trust closure, source identity, exact package/revocation closure, public predecessor, or tag state is uncertain. A failure during or after the single atomic push is reported as indeterminate; rerun settles only an exact anonymous readback match, and there is no blind retry, cleanup, deletion, update, or force path.",
        "independent-review":
          "The non-authoring TASK-0042 independent architecture review examined this publisher authority boundary and identified this contract omission. Before any publication, a person or agent that did not author the release must review the exact signed source commit, trust-set verification report, immutable tag plan, and atomic refspecs; the operator retains the named reviewer and exact commit outside the repository, and neither the signing workflow nor a green gate substitutes for that review.",
        "no-overwrite":
          "Immediately before mutation, the new set tag and new revocation tag must be absent locally and remotely. Existing Core-derived package tags are reused only when their tagged package path is byte-identical to the Core-verified signed source; prior revocation refs and bytes must equal the signed cumulative prefix. The atomic push has no force, delete, update, or retag path.",
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
      allowCurrentRemote: true,
    });
    requireSignedSourceClosure(dependencies, beforeTrust, firstCommit);
    preflightAbandonedIdentities(dependencies, before);
    const observedRemote = readRemoteMain(dependencies);
    if (observedRemote === firstCommit) {
      const evidence = runPublicVerification(
        before,
        beforeTrust,
        dependencies,
        { expectedCommit: firstCommit },
      );
      outputJSON(dependencies, {
        ...evidence,
        status: "PUBLISHED_SETTLED",
      });
      return 0;
    }
    requirePublicPredecessor(before, beforeTrust, dependencies);
    preflightPackageTagInventory(dependencies, before, beforeTrust);
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
    const evidence = runPublicVerification(after, afterTrust, dependencies, {
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
  {
    credentialFree = false,
    repositoryRoot = root,
    validateCurrentPackages = true,
  } = {},
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
  validateTrustReport(report, plan, trustSet, { validateCurrentPackages });
  return report;
}

function validateTrustReport(
  report,
  plan,
  trustSet,
  { validateCurrentPackages = true } = {},
) {
  if (
    trustSet === ABANDONED_PREPUBLICATION_SET_ID ||
    report?.disposition === "evidence-only"
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} is abandoned evidence-only and cannot be deployed`,
    );
  }
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
    report.packageCount !== 17 ||
    !Array.isArray(report.packages) ||
    report.packages.length !== 17 ||
    report.checkpoint?.status !== "verified" ||
    !Number.isSafeInteger(report.checkpoint?.pin?.sequence) ||
    report.checkpoint.pin.sequence < 0 ||
    report.checkpoint.pin.sequence > MAX_REVOCATION_SEQUENCE ||
    report.checkpoint.entryCount !== report.checkpoint.pin.sequence ||
    report.checkpoint.pin.checkpointApiVersion !==
      "trust.forms.takoform.com/v1" ||
    !digestPattern.test(report.checkpoint.pin.digest ?? "") ||
    !digestPattern.test(report.checkpoint.pin.entriesDigest ?? "") ||
    !Array.isArray(report.checkpointHistory) ||
    !Array.isArray(report.revocationTags) ||
    !Array.isArray(report.statements)
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} did not return the exact Core v1.1.0 publisher/package/checkpoint report`,
    );
  }
  if (validateCurrentPackages && plan.formCount !== 17) {
    throw new DeployBlocked(
      "publication plan must contain exactly 17 packages",
    );
  }
  const expectedTags = validateCurrentPackages
    ? plan.forms.map((form) => form.locator.tag)
    : null;
  const verifiedTags = report.packages.map((entry) => entry?.locator?.tag);
  if (
    validateCurrentPackages &&
    expectedTags.join("\n") !== verifiedTags.join("\n")
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} package identities differ from the publication plan`,
    );
  }
  const sequence = report.checkpoint.pin.sequence;
  const checkpointBundle = report.checkpoint.bundle;
  if (report.checkpointHistory.length !== sequence + 1) {
    throw new DeployBlocked(
      `trust set ${trustSet} does not contain the complete bounded checkpoint publisher history`,
    );
  }
  const seenCheckpointSets = new Set();
  for (let index = 0; index < report.checkpointHistory.length; index += 1) {
    const historical = report.checkpointHistory[index];
    const expectedVersion =
      index === 0 ? "0.0.0" : report.statements[index - 1]?.statementVersion;
    if (
      !commitPattern.test(historical?.setId ?? "") ||
      historical.setTag !== `${TRUST_SET_TAG_PREFIX}${historical.setId}` ||
      historical.checkpointVersion !== expectedVersion ||
      historical.pin?.checkpointApiVersion !== "trust.forms.takoform.com/v1" ||
      historical.pin?.sequence !== index ||
      !digestPattern.test(historical.pin?.digest ?? "") ||
      !digestPattern.test(historical.pin?.entriesDigest ?? "") ||
      seenCheckpointSets.has(historical.setId)
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} has invalid checkpoint publisher history at sequence ${index}`,
      );
    }
    seenCheckpointSets.add(historical.setId);
  }
  const currentHistory = report.checkpointHistory.at(-1);
  if (
    currentHistory.setId !== trustSet ||
    currentHistory.checkpointVersion !== report.checkpoint.checkpointVersion ||
    !pinsEqual(currentHistory.pin, report.checkpoint.pin)
  ) {
    throw new DeployBlocked(
      `trust set ${trustSet} checkpoint publisher history does not end at the current signed set`,
    );
  }
  if (sequence === 0) {
    if (
      report.checkpoint.checkpointVersion !== "0.0.0" ||
      report.checkpoint.pin.digest !== GENESIS_DIGEST ||
      report.checkpoint.pin.entriesDigest !== GENESIS_ENTRIES_DIGEST ||
      report.previousCheckpoint !== undefined ||
      (report.revocationTag ?? "") !== "" ||
      report.revocationTags.length !== 0 ||
      report.statements.length !== 0 ||
      !exactPublisherBundle(checkpointBundle, GENESIS_DIGEST, trustSet)
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} does not contain the exact signed Core API v1 genesis`,
      );
    }
  } else {
    const previous = report.previousCheckpoint;
    const historicalPrevious = report.checkpointHistory.at(-2);
    if (
      previous === null ||
      typeof previous !== "object" ||
      !commitPattern.test(previous.setId ?? "") ||
      previous.setTag !== `${TRUST_SET_TAG_PREFIX}${previous.setId}` ||
      previous.pin?.checkpointApiVersion !== "trust.forms.takoform.com/v1" ||
      previous.pin?.sequence !== sequence - 1 ||
      !digestPattern.test(previous.pin?.digest ?? "") ||
      !digestPattern.test(previous.pin?.entriesDigest ?? "") ||
      report.revocationTags.length !== sequence ||
      report.statements.length !== sequence ||
      !exactPublisherBundle(
        checkpointBundle,
        report.checkpoint.pin.digest,
        trustSet,
      )
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} does not contain one exact Core API v1 checkpoint advancement`,
      );
    }
    if (
      previous.setId !== historicalPrevious.setId ||
      previous.setTag !== historicalPrevious.setTag ||
      previous.checkpointVersion !== historicalPrevious.checkpointVersion ||
      !pinsEqual(previous.pin, historicalPrevious.pin)
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} immediate predecessor differs from its signed checkpoint history`,
      );
    }
    const seenVersions = new Set();
    for (let index = 0; index < report.statements.length; index += 1) {
      const statement = report.statements[index];
      const version = statement?.statementVersion;
      const tag = `${REVOCATION_TAG_PREFIX}${version}`;
      if (
        statement?.sequence !== index + 1 ||
        !semverPattern.test(version ?? "") ||
        version === "0.0.0" ||
        seenVersions.has(version) ||
        !digestPattern.test(statement.statementDigest ?? "") ||
        !digestPattern.test(statement.packageDigest ?? "") ||
        statement.sourcePath !== `forms/revocations/${version}.json` ||
        statement.tag !== tag ||
        report.revocationTags[index] !== tag
      ) {
        throw new DeployBlocked(
          `trust set ${trustSet} has invalid or non-consecutive revocation statement evidence at sequence ${index + 1}`,
        );
      }
      seenVersions.add(version);
    }
    const latest = report.statements.at(-1).statementVersion;
    const priorVersion =
      sequence === 1 ? "0.0.0" : report.statements.at(-2).statementVersion;
    if (
      report.checkpoint.checkpointVersion !== latest ||
      previous.checkpointVersion !== priorVersion ||
      report.revocationTag !== `${REVOCATION_TAG_PREFIX}${latest}`
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} revocation tag/version does not equal its checkpoint head`,
      );
    }
  }
  for (let index = 0; index < report.packages.length; index += 1) {
    const form = validateCurrentPackages ? plan.forms[index] : null;
    const verified = report.packages[index];
    if (
      !digestPattern.test(verified?.packageDigest ?? "") ||
      typeof verified?.locator?.tag !== "string" ||
      (validateCurrentPackages &&
        verified.packageDigest !== form.packageDigest) ||
      !exactPublisherBundle(verified.bundle, verified.packageDigest, trustSet)
    ) {
      throw new DeployBlocked(
        `trust set ${trustSet} has incomplete exact package evidence at index ${index}`,
      );
    }
  }
}

function pinsEqual(left, right) {
  return (
    left?.checkpointApiVersion === right?.checkpointApiVersion &&
    left?.sequence === right?.sequence &&
    left?.digest === right?.digest &&
    left?.entriesDigest === right?.entriesDigest
  );
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
  {
    credentialFree = false,
    expectedRemoteCommit,
    allowCurrentRemote = false,
  } = {},
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
  if (
    remoteCommit !== requiredRemote &&
    !(allowCurrentRemote && remoteCommit === commit)
  ) {
    throw new DeployBlocked(
      `origin main is ${remoteCommit || "<missing>"}, expected ${requiredRemote}`,
    );
  }
  return commit;
}

function readRemoteMain(dependencies, mutationStarted = false) {
  const remoteMain = requireSuccess(
    dependencies,
    "git",
    ["ls-remote", "origin", "refs/heads/main"],
    "cannot read origin main",
    mutationStarted,
    true,
  );
  const commit = parseRemoteRef(remoteMain, "refs/heads/main");
  if (!commitPattern.test(commit)) {
    throw new DeployBlocked(
      `origin main did not resolve an exact commit: ${commit || "<missing>"}`,
      mutationStarted,
    );
  }
  return commit;
}

function runPublicVerification(plan, trust, dependencies, options = {}) {
  return typeof dependencies.verifyPublicPublication === "function"
    ? dependencies.verifyPublicPublication(plan, trust, options)
    : verifyPublicPublication(plan, trust, dependencies, options);
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
      "forms/retained-packages.json",
      "forms/trust/abandoned-prepublication.json",
      "forms/revocations",
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
      "forms/trust/abandoned-prepublication.json",
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

function requirePublicPredecessor(plan, trust, dependencies) {
  const sequence = trust.checkpoint.pin.sequence;
  const expectedTags = trust.revocationTags.slice(0, -1);
  const expectedSetTags = trust.checkpointHistory
    .slice(0, -1)
    .map((checkpoint) => checkpoint.setTag);
  const remoteSetTags = readRemoteTrustSetTags(dependencies);
  assertExactTagNames(
    remoteSetTags,
    expectedSetTags,
    "public publisher-set predecessor",
  );
  const remoteTags = readRemoteRevocationTags(dependencies);
  assertExactTagNames(
    remoteTags,
    expectedTags,
    "public revocation predecessor",
  );
  if (sequence === 0) return;

  const previous = trust.previousCheckpoint;
  const setTagCommits = new Map();
  for (const historical of trust.checkpointHistory.slice(0, -1)) {
    setTagCommits.set(
      historical.setTag,
      readRemoteTagCommit(
        dependencies,
        historical.setTag,
        "public predecessor set",
      ),
    );
  }
  for (let index = 0; index < expectedTags.length; index += 1) {
    const checkpointSetTag = trust.checkpointHistory[index + 1].setTag;
    if (
      remoteTags.get(expectedTags[index]) !==
      setTagCommits.get(checkpointSetTag)
    ) {
      throw new DeployBlocked(
        `public checkpoint set ${checkpointSetTag} and revocation ${expectedTags[index]} do not identify one atomic publication commit`,
      );
    }
  }

  let temporary;
  try {
    temporary = mkdtempSync(
      path.join(tmpdir(), "takoform-public-predecessor-"),
    );
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
      "cannot fetch fresh anonymous public predecessor main",
      false,
      true,
    );
    const publicMain = requireSuccess(
      dependencies,
      "git",
      ["-C", temporary, "rev-parse", "HEAD"],
      "cannot resolve fresh public predecessor main",
      false,
      true,
    );
    if (publicMain !== trust.sourceCommit) {
      throw new DeployBlocked(
        `fresh public predecessor main is ${publicMain}, expected signed source ${trust.sourceCommit}`,
      );
    }
    requireSuccess(
      dependencies,
      "git",
      ["-C", temporary, "checkout", "--quiet", "--detach", publicMain],
      "cannot materialize fresh public predecessor main",
      false,
      true,
    );
    const tagRefspecs = [...expectedSetTags, ...expectedTags].map(
      (tag) => `refs/tags/${tag}:refs/tags/${tag}`,
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
        ...tagRefspecs,
      ],
      "cannot fetch immutable public predecessor refs",
      false,
      true,
    );
    for (const historical of trust.checkpointHistory.slice(0, -1)) {
      const fetchedSetCommit = requireSuccess(
        dependencies,
        "git",
        [
          "-C",
          temporary,
          "rev-parse",
          `refs/tags/${historical.setTag}^{commit}`,
        ],
        `cannot resolve fetched predecessor set ${historical.setTag}`,
        false,
        true,
      );
      if (fetchedSetCommit !== setTagCommits.get(historical.setTag)) {
        throw new DeployBlocked(
          `public predecessor set ${historical.setTag} changed during anonymous readback`,
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
          fetchedSetCommit,
          publicMain,
          "--",
          `forms/trust/sets/${historical.setId}`,
        ],
        `public predecessor set bytes changed after ${historical.setTag}`,
        false,
        true,
      );
    }
    for (let index = 0; index < expectedTags.length; index += 1) {
      const tag = expectedTags[index];
      const version = trust.statements[index].statementVersion;
      const fetched = requireSuccess(
        dependencies,
        "git",
        ["-C", temporary, "rev-parse", `refs/tags/${tag}^{commit}`],
        `cannot resolve fetched revocation tag ${tag}`,
        false,
        true,
      );
      if (fetched !== remoteTags.get(tag)) {
        throw new DeployBlocked(
          `public revocation tag ${tag} changed during anonymous readback`,
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
          fetched,
          publicMain,
          "--",
          `forms/revocations/${version}.json`,
          `forms/revocations/checkpoints/${version}.json`,
        ],
        `public revocation bytes at ${tag} were updated or deleted`,
        false,
        true,
      );
    }
    const predecessor = readTrustSet(dependencies, plan, previous.setId, {
      credentialFree: true,
      repositoryRoot: temporary,
      validateCurrentPackages: false,
    });
    if (
      predecessor.setTag !== previous.setTag ||
      predecessor.checkpoint.checkpointVersion !== previous.checkpointVersion ||
      JSON.stringify(predecessor.checkpoint.pin) !==
        JSON.stringify(previous.pin) ||
      predecessor.revocationTags.join("\n") !== expectedTags.join("\n") ||
      JSON.stringify(predecessor.checkpointHistory) !==
        JSON.stringify(trust.checkpointHistory.slice(0, -1))
    ) {
      throw new DeployBlocked(
        `fresh public predecessor ${previous.setTag} does not equal the checkpoint pin signed into ${trust.setTag}`,
      );
    }
  } finally {
    if (temporary) rmSync(temporary, { recursive: true, force: true });
  }
}

function readRemoteTrustSetTags(dependencies, mutationStarted = false) {
  return readRemoteTagInventory(
    dependencies,
    TRUST_SET_TAG_PREFIX,
    "publisher-set",
    mutationStarted,
  );
}

function readRemoteRevocationTags(dependencies, mutationStarted = false) {
  return readRemoteTagInventory(
    dependencies,
    REVOCATION_TAG_PREFIX,
    "revocation",
    mutationStarted,
  );
}

function readRemoteTagInventory(
  dependencies,
  prefix,
  label,
  mutationStarted = false,
) {
  const output = requireSuccess(
    dependencies,
    "git",
    ["ls-remote", "--tags", "origin", `refs/tags/${prefix}*`],
    `cannot read public ${label} tag inventory`,
    mutationStarted,
    true,
  );
  const found = new Map();
  for (const line of output.split(/\r?\n/u).filter(Boolean)) {
    const [commit, ref, extra] = line.trim().split(/\s+/u);
    if (
      extra !== undefined ||
      !commitPattern.test(commit ?? "") ||
      !ref?.startsWith(`refs/tags/${prefix}`) ||
      ref.endsWith("^{}")
    ) {
      throw new DeployBlocked(
        `public ${label} tag inventory contains an invalid ref: ${line}`,
        mutationStarted,
      );
    }
    const tag = ref.slice("refs/tags/".length);
    if (found.has(tag)) {
      throw new DeployBlocked(
        `public ${label} tag inventory repeats ${tag}`,
        mutationStarted,
      );
    }
    found.set(tag, commit);
    const limit =
      prefix === TRUST_SET_TAG_PREFIX
        ? MAX_REVOCATION_SEQUENCE + 1
        : MAX_REVOCATION_SEQUENCE;
    if (found.size > limit) {
      throw new DeployBlocked(
        `public ${label} tag inventory exceeds the bounded ${limit}-ref publisher history`,
        mutationStarted,
      );
    }
  }
  return found;
}

function readRemotePackageTagInventory(dependencies, mutationStarted = false) {
  const output = requireSuccess(
    dependencies,
    "git",
    ["ls-remote", "--tags", "origin", "refs/tags/forms/*"],
    "cannot read public Form Package release tag inventory",
    mutationStarted,
    true,
  );
  const found = new Map();
  for (const line of output.split(/\r?\n/u).filter(Boolean)) {
    const [commit, ref, extra] = line.trim().split(/\s+/u);
    if (ref?.endsWith("^{}")) continue;
    if (extra !== undefined || !commitPattern.test(commit ?? "")) {
      throw new DeployBlocked(
        `public Form Package release tag inventory contains an invalid ref: ${line}`,
        mutationStarted,
      );
    }
    const tag = ref?.slice("refs/tags/".length) ?? "";
    if (tag.startsWith("forms/sets/") || tag.startsWith("forms/revocations/")) {
      continue;
    }
    if (!/^forms\/[^/]+\/sha256-[0-9a-f]{64}$/u.test(tag)) {
      throw new DeployBlocked(
        `public Form Package release tag inventory contains an unexpected ref: ${line}`,
        mutationStarted,
      );
    }
    if (found.has(tag)) {
      throw new DeployBlocked(
        `public Form Package release tag inventory repeats ${tag}`,
        mutationStarted,
      );
    }
    found.set(tag, commit);
  }
  return found;
}

/**
 * Check the immutable package-tag surface before the owner gate and before
 * any mutation. Current tags may be absent (the atomic push creates those
 * roots), but every retained tag is an already-published identity and must
 * exist, point at a commit whose package bytes equal the signed source, and
 * resolve through Core to the exact publisher-owned locator. No unlisted
 * package tag is accepted as an accidental third history root.
 */
function preflightPackageTagInventory(dependencies, plan, trust) {
  const actual = readRemotePackageTagInventory(dependencies);
  const currentTags = new Set(plan.forms.map((form) => form.locator.tag));
  const retained = plan.retainedPackages ?? [];
  const retainedByTag = new Map(retained.map((entry) => [entry.tag, entry]));
  for (const tag of actual.keys()) {
    if (!currentTags.has(tag) && !retainedByTag.has(tag)) {
      throw new DeployBlocked(
        `public Form Package release inventory contains an unknown pre-existing tag ${tag}`,
      );
    }
  }
  for (const entry of retained) {
    const tagCommit = actual.get(entry.tag);
    if (!tagCommit) {
      throw new DeployBlocked(
        `retained package tag ${entry.tag} is missing from the preflight inventory`,
      );
    }
    requireSuccess(
      dependencies,
      "git",
      ["cat-file", "-e", `${tagCommit}^{commit}`],
      `cannot resolve retained package tag ${entry.tag} commit ${tagCommit} locally`,
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
        entry.sourcePath,
      ],
      `retained package tag ${entry.tag} does not contain the exact Core-verified package bytes`,
    );

    const localRoot = path.resolve(plan.repositoryRoot ?? root);
    const releaseRoot = path.join(localRoot, ...entry.sourcePath.split("/"));
    const locator = runCoreLocatorForFetched(dependencies, releaseRoot, false);
    const expectedLocator = {
      apiVersion: "packages.forms.takoform.com/v1alpha5",
      releaseId: entry.releaseId,
      artifactId: entry.artifactId,
      tag: entry.tag,
      sourcePath: entry.sourcePath,
    };
    if (
      locator.apiVersion !== expectedLocator.apiVersion ||
      locator.releaseId !== expectedLocator.releaseId ||
      locator.artifactId !== expectedLocator.artifactId ||
      locator.tag !== expectedLocator.tag ||
      locator.sourcePath !== expectedLocator.sourcePath ||
      locator.artifactId !== entry.packageDigest.replace(":", "-")
    ) {
      throw new DeployBlocked(
        `retained package tag ${entry.tag} Core locator differs from the exact inventory`,
      );
    }
  }
}

/**
 * Evidence-only recovery roots are retained in the release tree for audit and
 * readback, but their abandoned set/package tags must never enter the public
 * tag inventory. Prove those four immutable refs absent before the owner gate
 * and before any push. A non-zero remote/local result is only acceptable when
 * it is the ordinary "not found" status; uncertainty fails closed.
 */
function preflightAbandonedIdentities(dependencies, plan) {
  const entries = plan.evidenceOnlyPackages ?? [];
  const forbiddenTags = [
    ABANDONED_PREPUBLICATION_SET_TAG,
    ...entries.map((entry) => entry.tag),
  ];
  for (const tag of forbiddenTags) {
    const local = runDependency(dependencies, "git", [
      "show-ref",
      "--verify",
      "--quiet",
      `refs/tags/${tag}`,
    ]);
    if (local.exitCode === 0) {
      throw new DeployBlocked(
        `local abandoned evidence-only tag ${tag} already exists`,
      );
    }
    if (local.exitCode !== 1) {
      throw new DeployBlocked(
        `cannot prove local abandoned evidence-only tag ${tag} is absent${commandDetail(local) ? `:\n${commandDetail(local)}` : ""}`,
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
      `cannot inspect remote abandoned evidence-only tag ${tag}`,
    );
    if (remote !== "") {
      throw new DeployBlocked(
        `remote abandoned evidence-only tag ${tag} already exists`,
      );
    }
  }
}

function assertExactTagNames(actual, expected, label, mutationStarted = false) {
  if (
    actual.size !== expected.length ||
    expected.some((tag) => !actual.has(tag))
  ) {
    throw new DeployBlocked(
      `${label} refs are ${[...actual.keys()].sort().join(", ") || "<empty>"}, expected ${expected.join(", ") || "<empty>"}`,
      mutationStarted,
    );
  }
}

function readRemoteTagCommit(
  dependencies,
  tag,
  label,
  mutationStarted = false,
) {
  const output = requireSuccess(
    dependencies,
    "git",
    [
      "ls-remote",
      "--tags",
      "origin",
      `refs/tags/${tag}`,
      `refs/tags/${tag}^{}`,
    ],
    `cannot read ${label} tag ${tag}`,
    mutationStarted,
    true,
  );
  const direct = parseRemoteRef(output, `refs/tags/${tag}`);
  const peeled = parseRemoteRef(output, `refs/tags/${tag}^{}`);
  const commit = peeled || direct;
  if (!commitPattern.test(commit ?? "")) {
    throw new DeployBlocked(
      `${label} tag ${tag} did not resolve to an exact commit`,
      mutationStarted,
    );
  }
  return commit;
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

  if (trust.revocationTag) {
    const revocationTag = trust.revocationTag;
    const localRevocation = runDependency(dependencies, "git", [
      "show-ref",
      "--verify",
      "--quiet",
      `refs/tags/${revocationTag}`,
    ]);
    if (localRevocation.exitCode === 0) {
      throw new DeployBlocked(
        `local revocation tag ${revocationTag} already exists`,
      );
    }
    if (localRevocation.exitCode !== 1) {
      throw new DeployBlocked(
        `cannot prove local revocation tag ${revocationTag} is absent${commandDetail(localRevocation) ? `:\n${commandDetail(localRevocation)}` : ""}`,
      );
    }
    const remoteRevocation = requireSuccess(
      dependencies,
      "git",
      [
        "ls-remote",
        "--tags",
        "origin",
        `refs/tags/${revocationTag}`,
        `refs/tags/${revocationTag}^{}`,
      ],
      `cannot inspect origin revocation tag ${revocationTag}`,
    );
    if (remoteRevocation !== "") {
      throw new DeployBlocked(
        `remote revocation tag ${revocationTag} already exists; update/delete/retag is forbidden`,
      );
    }
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
  if (trust.revocationTag) {
    refs.push(`${commit}:refs/tags/${trust.revocationTag}`);
  }
  requireSuccess(
    dependencies,
    "git",
    ["push", "--atomic", "origin", ...refs],
    `push of main, ${missingPackageTags.length} new package tags, signed set ${trust.setTag}${trust.revocationTag ? `, and revocation ${trust.revocationTag}` : ""} did not complete cleanly`,
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

  const setTagCommits = readRemoteTrustSetTags(dependencies, mutationStarted);
  const expectedSetTags = trust.checkpointHistory.map(
    (historical) => historical.setTag,
  );
  assertExactTagNames(
    setTagCommits,
    expectedSetTags,
    "public publisher-set",
    mutationStarted,
  );
  if (setTagCommits.get(trust.setTag) !== localCommit) {
    throw new DeployBlocked(
      `public signed set tag ${trust.setTag} points to ${setTagCommits.get(trust.setTag) || "<missing>"}, expected ${localCommit}`,
      mutationStarted,
    );
  }

  const revocationTagCommits = readRemoteRevocationTags(
    dependencies,
    mutationStarted,
  );
  assertExactTagNames(
    revocationTagCommits,
    trust.revocationTags,
    "public revocation",
    mutationStarted,
  );
  if (
    trust.revocationTag &&
    revocationTagCommits.get(trust.revocationTag) !== localCommit
  ) {
    throw new DeployBlocked(
      `public revocation tag ${trust.revocationTag} points to ${revocationTagCommits.get(trust.revocationTag) || "<missing>"}, expected ${localCommit}`,
      mutationStarted,
    );
  }
  for (let index = 0; index < trust.revocationTags.length; index += 1) {
    const checkpointSetTag = trust.checkpointHistory[index + 1].setTag;
    if (
      revocationTagCommits.get(trust.revocationTags[index]) !==
      setTagCommits.get(checkpointSetTag)
    ) {
      throw new DeployBlocked(
        `public checkpoint set ${checkpointSetTag} and revocation ${trust.revocationTags[index]} do not identify one atomic publication commit`,
        mutationStarted,
      );
    }
  }

  const packageTagInventory = readRemotePackageTagInventory(
    dependencies,
    mutationStarted,
  );
  const expectedReleaseTags = [
    ...plan.forms.map((form) => form.locator.tag),
    ...(plan.retainedPackages ?? []).map((retained) => retained.tag),
  ];
  assertExactTagNames(
    packageTagInventory,
    expectedReleaseTags,
    "public Form Package release",
    mutationStarted,
  );

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
  const retainedTagCommits = new Map();
  for (const retained of plan.retainedPackages ?? []) {
    const tagOutput = requireSuccess(
      dependencies,
      "git",
      [
        "ls-remote",
        "--tags",
        "origin",
        `refs/tags/${retained.tag}`,
        `refs/tags/${retained.tag}^{}`,
      ],
      `cannot read public retained package tag ${retained.tag}`,
      mutationStarted,
      true,
    );
    const direct = parseRemoteRef(tagOutput, `refs/tags/${retained.tag}`);
    const peeled = parseRemoteRef(tagOutput, `refs/tags/${retained.tag}^{}`);
    const tagCommit = peeled || direct;
    if (!commitPattern.test(tagCommit ?? "")) {
      throw new DeployBlocked(
        `public retained package tag ${retained.tag} did not resolve to a commit`,
        mutationStarted,
      );
    }
    retainedTagCommits.set(retained.tag, tagCommit);
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
        ...(plan.retainedPackages ?? []).map(
          (retained) => `refs/tags/${retained.tag}:refs/tags/${retained.tag}`,
        ),
        ...expectedSetTags.map((tag) => `refs/tags/${tag}:refs/tags/${tag}`),
        ...trust.revocationTags.map(
          (tag) => `refs/tags/${tag}:refs/tags/${tag}`,
        ),
      ],
      "cannot fetch the public package tags",
      mutationStarted,
      true,
    );
    try {
      for (const historical of trust.checkpointHistory) {
        const fetchedSetCommit = requireSuccess(
          dependencies,
          "git",
          [
            "-C",
            temporary,
            "rev-parse",
            `refs/tags/${historical.setTag}^{commit}`,
          ],
          `cannot resolve fetched public publisher set ${historical.setTag}`,
          mutationStarted,
          true,
        );
        if (fetchedSetCommit !== setTagCommits.get(historical.setTag)) {
          throw new DeployBlocked(
            `public publisher set ${historical.setTag} changed during readback`,
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
            fetchedSetCommit,
            localCommit,
            "--",
            `forms/trust/sets/${historical.setId}`,
          ],
          `public publisher set bytes at ${historical.setTag} were updated or deleted`,
          mutationStarted,
          true,
        );
      }
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
      for (const retained of plan.retainedPackages ?? []) {
        const fetchedTagCommit = requireSuccess(
          dependencies,
          "git",
          ["-C", temporary, "rev-parse", `refs/tags/${retained.tag}^{commit}`],
          `cannot resolve fetched retained package tag ${retained.tag}`,
          mutationStarted,
          true,
        );
        if (fetchedTagCommit !== retainedTagCommits.get(retained.tag)) {
          throw new DeployBlocked(
            `public retained package tag ${retained.tag} changed during readback`,
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
            `refs/tags/${retained.tag}^{commit}`,
            localCommit,
            "--",
            retained.sourcePath,
          ],
          `public retained package tag ${retained.tag} bytes were updated or deleted`,
          mutationStarted,
          true,
        );
      }
      for (let index = 0; index < trust.revocationTags.length; index += 1) {
        const tag = trust.revocationTags[index];
        const version = trust.statements[index].statementVersion;
        const fetchedTagCommit = requireSuccess(
          dependencies,
          "git",
          ["-C", temporary, "rev-parse", `refs/tags/${tag}^{commit}`],
          `cannot resolve fetched public revocation tag ${tag}`,
          mutationStarted,
          true,
        );
        if (fetchedTagCommit !== revocationTagCommits.get(tag)) {
          throw new DeployBlocked(
            `public revocation tag ${tag} changed during readback`,
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
            fetchedTagCommit,
            localCommit,
            "--",
            `forms/revocations/${version}.json`,
            `forms/revocations/checkpoints/${version}.json`,
          ],
          `public revocation tag ${tag} bytes were updated or deleted`,
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
      checkpointHistory: trust.checkpointHistory,
      revocationTag: trust.revocationTag || null,
    },
    revocationTagCount: trust.revocationTags.length,
    revocationTags: trust.revocationTags.map((tag, index) => ({
      tag,
      commit: revocationTagCommits.get(tag),
      statementVersion: trust.statements[index].statementVersion,
      statementDigest: trust.statements[index].statementDigest,
    })),
    currentPackageCount: plan.formCount,
    releaseRootCount:
      plan.releaseRootCount ??
      plan.formCount +
        (plan.retainedPackages ?? []).length +
        (plan.evidenceOnlyPackages ?? []).length,
    releaseTagCount: plan.formCount + (plan.retainedPackages ?? []).length,
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => ({
      tag: form.locator.tag,
      commit: packageTagCommits.get(form.locator.tag),
      releaseId: form.locator.releaseId,
      artifactId: form.locator.artifactId,
      sourcePath: form.locator.sourcePath,
      packageDigest: form.packageDigest,
    })),
    retainedTags: (plan.retainedPackages ?? []).map((retained) => ({
      tag: retained.tag,
      commit: retainedTagCommits.get(retained.tag),
      releaseId: retained.releaseId,
      artifactId: retained.artifactId,
      sourcePath: retained.sourcePath,
      packageDigest: retained.packageDigest,
    })),
    evidenceOnlyPackages: (plan.evidenceOnlyPackages ?? []).map((evidence) => ({
      releaseId: evidence.releaseId,
      artifactId: evidence.artifactId,
      sourcePath: evidence.sourcePath,
      packageDigest: evidence.packageDigest,
      formRef: evidence.formRef,
    })),
    postConditions: [
      "PUBLIC_MAIN_READBACK",
      "SIGNED_SET_TAG_READBACK",
      "APPEND_ONLY_REVOCATION_TAG_CHAIN_READBACK",
      "ALL_22_RELEASE_ROOTS_AND_19_TAGGED_PACKAGE_BYTES_READBACK",
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
    verifyFetchedReleaseRoot(
      {
        kind: form.kind,
        locator: form.locator,
        packageDigest: form.packageDigest,
      },
      fetchedRoot,
      localRoot,
      dependencies,
      mutationStarted,
      expected,
    );
  }
  for (const evidence of plan.evidenceOnlyPackages ?? []) {
    verifyFetchedReleaseRoot(
      {
        kind: evidence.formRef.kind,
        locator: {
          apiVersion: "packages.forms.takoform.com/v1alpha5",
          releaseId: evidence.releaseId,
          artifactId: evidence.artifactId,
          tag: evidence.tag,
          sourcePath: evidence.sourcePath,
        },
        packageDigest: evidence.packageDigest,
        formRef: evidence.formRef,
      },
      fetchedRoot,
      localRoot,
      dependencies,
      mutationStarted,
      expected,
    );
  }
  for (const retained of plan.retainedPackages ?? []) {
    verifyFetchedReleaseRoot(
      {
        kind: retained.formRef.kind,
        locator: {
          apiVersion: "packages.forms.takoform.com/v1alpha5",
          releaseId: retained.releaseId,
          artifactId: retained.artifactId,
          tag: retained.tag,
          sourcePath: retained.sourcePath,
        },
        packageDigest: retained.packageDigest,
        formRef: retained.formRef,
      },
      fetchedRoot,
      localRoot,
      dependencies,
      mutationStarted,
      expected,
    );
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

function verifyFetchedReleaseRoot(
  entry,
  fetchedRoot,
  localRoot,
  dependencies,
  mutationStarted,
  expected,
) {
  const releaseRoot = path.join(
    fetchedRoot,
    ...entry.locator.sourcePath.split("/"),
  );
  const localReleaseRoot = path.join(
    localRoot,
    ...entry.locator.sourcePath.split("/"),
  );
  if (!existsSync(releaseRoot)) {
    throw new DeployBlocked(
      `${entry.locator.sourcePath}: public release path is missing`,
      mutationStarted,
    );
  }
  const publicFiles = inventoryRelativeFiles(releaseRoot);
  const localFiles = inventoryRelativeFiles(localReleaseRoot);
  if (localFiles.length !== publicFiles.length) {
    throw new DeployBlocked(
      `${entry.kind}: public release closure file count differs from local`,
      mutationStarted,
    );
  }
  for (const relative of localFiles) {
    const localPath = path.join(localReleaseRoot, relative);
    const publicPath = path.join(releaseRoot, relative);
    if (!existsSync(publicPath) || !bytesEqual(localPath, publicPath)) {
      throw new DeployBlocked(
        `${entry.kind}: public release bytes differ at ${relative}`,
        mutationStarted,
      );
    }
  }
  for (const relative of publicFiles) {
    expected.set(
      `${entry.locator.sourcePath}/${relative}`,
      publicPathFor(releaseRoot, relative),
    );
  }
  const locator = runCoreLocatorForFetched(
    dependencies,
    releaseRoot,
    mutationStarted,
  );
  if (
    locator.apiVersion !== entry.locator.apiVersion ||
    locator.releaseId !== entry.locator.releaseId ||
    locator.artifactId !== entry.locator.artifactId ||
    locator.tag !== entry.locator.tag ||
    locator.sourcePath !== entry.locator.sourcePath
  ) {
    throw new DeployBlocked(
      `${entry.kind}: public release locator differs from the exact inventory`,
      mutationStarted,
    );
  }
  if (entry.formRef) {
    const packageIndex = JSON.parse(
      readFileSync(path.join(releaseRoot, "package-index.json"), "utf8"),
    );
    if (
      JSON.stringify(packageIndex.formRef) !== JSON.stringify(entry.formRef)
    ) {
      throw new DeployBlocked(
        `${entry.kind}: public release FormRef differs from the exact inventory`,
        mutationStarted,
      );
    }
    if (locator.artifactId !== entry.packageDigest.replace(":", "-")) {
      throw new DeployBlocked(
        `${entry.kind}: public release package digest differs from the exact inventory`,
        mutationStarted,
      );
    }
  }
}

function publicPathFor(releaseRoot, relative) {
  return path.join(releaseRoot, relative);
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
  const retained = (plan) =>
    (plan.retainedPackages ?? [])
      .map(
        (entry) =>
          `${entry.tag}:${entry.sourcePath}:${entry.packageDigest}:${JSON.stringify(entry.formRef)}`,
      )
      .join("\n");
  if (retained(before) !== retained(after)) {
    throw new DeployBlocked(
      "retained publication inventory changed during the owner gate",
    );
  }
  const evidenceOnly = (plan) =>
    (plan.evidenceOnlyPackages ?? [])
      .map(
        (entry) =>
          `${entry.tag}:${entry.sourcePath}:${entry.packageDigest}:${JSON.stringify(entry.formRef)}`,
      )
      .join("\n");
  if (evidenceOnly(before) !== evidenceOnly(after)) {
    throw new DeployBlocked(
      "abandoned evidence-only publication inventory changed during the owner gate",
    );
  }
}

function assertTrustReportsEqual(before, after) {
  const stable = (report) =>
    JSON.stringify({
      status: report.status,
      coreVersion: report.coreVersion,
      family: report.family,
      setId: report.setId,
      setTag: report.setTag,
      disposition: report.disposition ?? null,
      evidenceOnlyPackages: report.evidenceOnlyPackages ?? [],
      packageCount: report.packageCount,
      publisherIdentity: report.publisherIdentity,
      sourceCommit: report.sourceCommit,
      workflowCommit: report.workflowCommit,
      buildConfigCommit: report.buildConfigCommit,
      checkpoint: report.checkpoint,
      checkpointHistory: report.checkpointHistory,
      previousCheckpoint: report.previousCheckpoint ?? null,
      revocationTag: report.revocationTag ?? null,
      revocationTags: report.revocationTags,
      statements: report.statements,
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
      previousSetId: trust.previousCheckpoint?.setId ?? null,
      revocationTag: trust.revocationTag || null,
    },
    currentPackageCount: plan.formCount,
    releaseRootCount:
      plan.releaseRootCount ??
      plan.formCount +
        (plan.retainedPackages ?? []).length +
        (plan.evidenceOnlyPackages ?? []).length,
    releaseTagCount: plan.formCount + (plan.retainedPackages ?? []).length,
    tagCount: plan.formCount,
    tags: plan.forms.map((form) => form.locator.tag),
    evidenceOnlyPackages: (plan.evidenceOnlyPackages ?? []).map((entry) => ({
      tag: entry.tag,
      sourcePath: entry.sourcePath,
      packageDigest: entry.packageDigest,
      formRef: entry.formRef,
    })),
    packageTagsToCreate: missingPackageTags,
    revocationTagToCreate: trust.revocationTag || null,
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
