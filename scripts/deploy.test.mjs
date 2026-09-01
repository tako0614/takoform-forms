import { describe, expect, test } from "bun:test";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import {
  credentialFreeInvocation,
  DEPLOY_CONTRACT,
  RELEASE_SURFACE,
  REPOSITORY_URL,
  runDeploy,
  verifyPublicPublication,
  parseDeployInvocation,
} from "./deploy.mjs";

const COMMIT = "0123456789abcdef0123456789abcdef01234567";
const SOURCE_COMMIT = "89abcdef0123456789abcdef0123456789abcdef";
const EXISTING_TAG_COMMIT = "fedcba9876543210fedcba9876543210fedcba98";
const PREVIOUS_SET = "13579bdf02468ace13579bdf02468ace13579bdf";
const GENESIS_SET = "2468ace13579bdf02468ace13579bdf02468ace1";
const REVOCATION_TAG = "forms/revocations/v1.0.0";
const SECOND_REVOCATION_TAG = "forms/revocations/v1.1.0";

describe("Edge Form Package deploy surface", () => {
  test("exposes the exact contract and accepts only the documented CLI", () => {
    expect(DEPLOY_CONTRACT.kind).toBe("takos.deploy-contract@v2");
    expect(DEPLOY_CONTRACT.surfaces).toHaveLength(1);
    expect(DEPLOY_CONTRACT.surfaces[0].surface).toBe(RELEASE_SURFACE);
    expect(DEPLOY_CONTRACT.surfaces[0].target).toContain(REPOSITORY_URL);
    expect(DEPLOY_CONTRACT.surfaces[0].triggers).toEqual([
      "authority",
      "published-identity",
    ]);
    expect(
      JSON.stringify(DEPLOY_CONTRACT.surfaces[0].obligations),
    ).not.toContain("unsigned");
    expect(Object.keys(DEPLOY_CONTRACT.surfaces[0].obligations).sort()).toEqual(
      [
        "failure-handling",
        "independent-review",
        "no-overwrite",
        "post-conditions",
        "provenance",
        "reversal",
      ],
    );
    expect(
      DEPLOY_CONTRACT.surfaces[0].obligations["independent-review"],
    ).toContain("TASK-0042 independent architecture review");
    expect(
      DEPLOY_CONTRACT.surfaces[0].obligations["independent-review"],
    ).toContain("exact signed source commit");
    expect(parseDeployInvocation(["--contract"])).toEqual({ mode: "contract" });
    expect(
      parseDeployInvocation([RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT]),
    ).toEqual({
      mode: "publish",
      trustSet: SOURCE_COMMIT,
    });
    expect(
      parseDeployInvocation([
        RELEASE_SURFACE,
        "--trust-set",
        SOURCE_COMMIT,
        "--dry-run",
      ]),
    ).toEqual({
      mode: "dry-run",
      trustSet: SOURCE_COMMIT,
    });
    expect(
      parseDeployInvocation([
        RELEASE_SURFACE,
        "--trust-set",
        SOURCE_COMMIT,
        "--verify",
      ]),
    ).toEqual({
      mode: "verify",
      trustSet: SOURCE_COMMIT,
    });
    for (const args of [
      [],
      ["other"],
      [RELEASE_SURFACE],
      [RELEASE_SURFACE, "--verify"],
      [RELEASE_SURFACE, "--trust-set", "not-a-commit"],
      [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--force"],
    ]) {
      expect(() => parseDeployInvocation(args)).toThrow(/usage:/);
    }
  });

  test("refuses unsigned publication and byte-only anonymous verification before Git access", () => {
    const plan = makePlan();
    for (const mode of ["--dry-run", "--verify"]) {
      const fixture = makeCommandDependencies(plan, {
        trustError: "signed trust set is missing",
      });
      expect(
        runDeploy(
          [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, mode],
          fixture.dependencies,
        ),
      ).toBe(1);
      expect(fixture.stderr).toContain("signed trust set is missing");
      expect(fixture.calls).toHaveLength(0);
    }
  });

  test("dry-run proves preconditions and reuses only byte-identical package tags", () => {
    const plan = makePlan();
    const clean = makeCommandDependencies(plan);
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        clean.dependencies,
      ),
    ).toBe(0);
    expect(JSON.parse(clean.stdout).status).toBe("DRY_RUN_VERIFIED");
    expect(
      clean.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);
    const wrongMain = makeCommandDependencies(plan, {
      remoteMainCommit: COMMIT,
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        wrongMain.dependencies,
      ),
    ).toBe(1);
    expect(wrongMain.stderr).toContain(
      `public signed set tag forms/sets/${SOURCE_COMMIT} points to <missing>, expected ${COMMIT}`,
    );
    expect(
      wrongMain.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);

    const reusable = makeCommandDependencies(plan, {
      existingRemoteTag: plan.forms[0].locator.tag,
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        reusable.dependencies,
      ),
    ).toBe(0);
    expect(JSON.parse(reusable.stdout).packageTagsToCreate).not.toContain(
      plan.forms[0].locator.tag,
    );

    const divergent = makeCommandDependencies(plan, {
      existingRemoteTag: plan.forms[0].locator.tag,
      existingRemoteTagDiverges: true,
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        divergent.dependencies,
      ),
    ).toBe(1);
    expect(divergent.stderr).toContain(
      `existing remote package tag ${plan.forms[0].locator.tag} does not contain the Core-verified package bytes`,
    );
    expect(
      divergent.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);

    const verifierDrift = makeCommandDependencies(plan, {
      signedVerifierDiverges: true,
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        verifierDrift.dependencies,
      ),
    ).toBe(1);
    expect(verifierDrift.stderr).toContain(
      "publisher trust verifier or authority inputs changed after the signed source",
    );
  });

  test("preflights the exact retained package inventory before the owner gate or any push", () => {
    const plan = makePlan();
    const retained = plan.retainedPackages;

    const missing = makeCommandDependencies(plan, {
      retainedTagMode: "missing",
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        missing.dependencies,
      ),
    ).toBe(1);
    expect(missing.stderr).toContain(
      `retained package tag ${retained[0].tag} is missing from the preflight inventory`,
    );
    expect(
      missing.calls.some(
        (call) => call.command === "bun" && call.args.join(" ") === "run check",
      ),
    ).toBe(false);
    expect(
      missing.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);

    const divergent = makeCommandDependencies(plan, {
      retainedTagMode: "divergent",
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        divergent.dependencies,
      ),
    ).toBe(1);
    expect(divergent.stderr).toContain(
      `retained package tag ${retained[0].tag} does not contain the exact Core-verified package bytes`,
    );
    expect(
      divergent.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);

    const unknown = makeCommandDependencies(plan, {
      retainedTagMode: "extra",
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT, "--dry-run"],
        unknown.dependencies,
      ),
    ).toBe(1);
    expect(unknown.stderr).toContain(
      "public Form Package release inventory contains an unknown pre-existing tag",
    );
    expect(
      unknown.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);
  });

  test("uses one atomic first push with direct tag refspecs and no local tag creation", () => {
    const plan = makePlan();
    const fixture = makeCommandDependencies(plan, {
      pushExitCode: 1,
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        fixture.dependencies,
      ),
    ).toBe(1);
    const pushes = fixture.calls.filter(
      (call) => call.command === "git" && call.args[0] === "push",
    );
    expect(pushes).toHaveLength(1);
    expect(pushes[0].args[1]).toBe("--atomic");
    expect(pushes[0].args).toContain("refs/heads/main:refs/heads/main");
    for (const form of plan.forms) {
      expect(pushes[0].args).toContain(
        `${SOURCE_COMMIT}:refs/tags/${form.locator.tag}`,
      );
    }
    expect(pushes[0].args).toContain(
      `${COMMIT}:refs/tags/forms/sets/${SOURCE_COMMIT}`,
    );
    expect(
      fixture.calls.some(
        (call) => call.command === "git" && call.args[0] === "tag",
      ),
    ).toBe(false);
    expect(fixture.stderr).toContain("publication is indeterminate");
  });

  test("advancement atomically creates one immutable revocation tag while preserving all package identities", () => {
    const plan = makePlan();
    const trust = makeAdvancementTrustReport(plan);
    const fixture = makeCommandDependencies(plan, {
      trustReport: trust,
      previousTrustReport: trustAtCommit(makeTrustReport(plan), PREVIOUS_SET),
      remoteTags: new Map([
        [`forms/sets/${PREVIOUS_SET}`, EXISTING_TAG_COMMIT],
      ]),
      pushExitCode: 1,
    });

    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        fixture.dependencies,
      ),
    ).toBe(1);
    const pushes = fixture.calls.filter(
      (call) => call.command === "git" && call.args[0] === "push",
    );
    expect(pushes).toHaveLength(1);
    expect(pushes[0].args).toContain(`${COMMIT}:refs/tags/${REVOCATION_TAG}`);
    expect(
      pushes[0].args.filter((value) =>
        value.includes("refs/tags/forms/revocations/"),
      ),
    ).toEqual([`${COMMIT}:refs/tags/${REVOCATION_TAG}`]);
    expect(pushes[0].args.filter((value) => value.startsWith("+:"))).toEqual(
      [],
    );
    for (const form of plan.forms) {
      expect(pushes[0].args).toContain(
        `${SOURCE_COMMIT}:refs/tags/${form.locator.tag}`,
      );
    }
  });

  test("settles an exact lost acknowledgement by anonymous readback without a second push", () => {
    const plan = makePlan();
    const trust = makeAdvancementTrustReport(plan);
    const remoteTags = new Map([
      [`forms/sets/${PREVIOUS_SET}`, EXISTING_TAG_COMMIT],
      [trust.setTag, COMMIT],
      [REVOCATION_TAG, COMMIT],
      ...plan.forms.map((form) => [form.locator.tag, EXISTING_TAG_COMMIT]),
    ]);
    const fixture = makeCommandDependencies(plan, {
      trustReport: trust,
      previousTrustReport: trustAtCommit(makeTrustReport(plan), PREVIOUS_SET),
      remoteMainCommit: COMMIT,
      remoteTags,
      verifyPublicPublication: () => ({
        kind: "takoform.form-package-publication-verification@v1",
        status: "VERIFIED",
        commit: COMMIT,
      }),
    });

    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        fixture.dependencies,
      ),
    ).toBe(0);
    expect(JSON.parse(fixture.stdout).status).toBe("PUBLISHED_SETTLED");
    expect(
      fixture.calls.some(
        (call) => call.command === "git" && call.args[0] === "push",
      ),
    ).toBe(false);
  });

  test("refuses revocation-tag insertion, deletion, or retagging before mutation", () => {
    const plan = makePlan();
    const first = makeAdvancementTrustReport(plan);
    const inserted = makeCommandDependencies(plan, {
      trustReport: first,
      previousTrustReport: trustAtCommit(makeTrustReport(plan), PREVIOUS_SET),
      remoteTags: new Map([
        [`forms/sets/${PREVIOUS_SET}`, EXISTING_TAG_COMMIT],
        [REVOCATION_TAG, COMMIT],
      ]),
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        inserted.dependencies,
      ),
    ).toBe(1);
    expect(inserted.stderr).toContain(
      `public revocation predecessor refs are ${REVOCATION_TAG}, expected <empty>`,
    );

    const second = makeSecondAdvancementTrustReport(plan);
    const previous = makePreviousAdvancementTrustReport(plan);
    const deleted = makeCommandDependencies(plan, {
      trustReport: second,
      previousTrustReport: previous,
      remoteTags: new Map([
        [
          `forms/sets/${GENESIS_SET}`,
          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        ],
        [`forms/sets/${PREVIOUS_SET}`, EXISTING_TAG_COMMIT],
      ]),
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        deleted.dependencies,
      ),
    ).toBe(1);
    expect(deleted.stderr).toContain(
      `public revocation predecessor refs are <empty>, expected ${REVOCATION_TAG}`,
    );

    const retagged = makeCommandDependencies(plan, {
      trustReport: second,
      previousTrustReport: previous,
      remoteTags: new Map([
        [
          `forms/sets/${GENESIS_SET}`,
          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        ],
        [`forms/sets/${PREVIOUS_SET}`, EXISTING_TAG_COMMIT],
        [REVOCATION_TAG, COMMIT],
      ]),
    });
    expect(
      runDeploy(
        [RELEASE_SURFACE, "--trust-set", SOURCE_COMMIT],
        retagged.dependencies,
      ),
    ).toBe(1);
    expect(retagged.stderr).toContain(
      `public checkpoint set forms/sets/${PREVIOUS_SET} and revocation ${REVOCATION_TAG} do not identify one atomic publication commit`,
    );

    for (const fixture of [inserted, deleted, retagged]) {
      expect(
        fixture.calls.some(
          (call) => call.command === "git" && call.args[0] === "push",
        ),
      ).toBe(false);
    }
  });

  test("anonymous advancement verification checks every set, revocation, package tag, and byte closure", () => {
    const root = mkdtempSync(
      path.join(tmpdir(), "takoform-public-verify-test-"),
    );
    const plan = makePlan(root);
    const trust = makeAdvancementTrustReport(plan);
    writePublicFixture(plan);
    const calls = [];
    let extraReleaseTag = false;
    let missingRetainedTag = false;
    const dependencies = {
      run() {
        throw new Error("credentialed command used by --verify");
      },
      runReadOnly(command, args) {
        calls.push({ command, args });
        if (
          command === "git" &&
          args[0] === "ls-remote" &&
          args[1] === "origin" &&
          args[2] === "refs/heads/main"
        ) {
          return ok(`${COMMIT}\trefs/heads/main\n`);
        }
        if (
          command === "git" &&
          args[0] === "ls-remote" &&
          args[1] === "--tags"
        ) {
          const refs = args.slice(3);
          if (refs.some((ref) => ref.endsWith("*"))) {
            if (refs.includes("refs/tags/forms/*")) {
              const packageLines = [
                ...plan.forms.map(
                  (form) =>
                    `${EXISTING_TAG_COMMIT}\trefs/tags/${form.locator.tag}`,
                ),
                ...(plan.retainedPackages ?? [])
                  .map((retained) => {
                    if (
                      missingRetainedTag &&
                      retained === plan.retainedPackages[0]
                    )
                      return null;
                    return `${EXISTING_TAG_COMMIT}\trefs/tags/${retained.tag}`;
                  })
                  .filter(Boolean),
              ];
              if (extraReleaseTag)
                packageLines.push(
                  `${EXISTING_TAG_COMMIT}\trefs/tags/forms/unlisted/sha256-${"e".repeat(64)}`,
                );
              return ok(`${packageLines.join("\n")}\n`);
            }
            if (refs.includes("refs/tags/forms/sets/*")) {
              return ok(
                `${EXISTING_TAG_COMMIT}\trefs/tags/forms/sets/${PREVIOUS_SET}\n${COMMIT}\trefs/tags/${trust.setTag}\n`,
              );
            }
            return ok(`${COMMIT}\trefs/tags/${REVOCATION_TAG}\n`);
          }
          const commit =
            refs.includes(`refs/tags/${trust.setTag}`) ||
            refs.includes(`refs/tags/${REVOCATION_TAG}`)
              ? COMMIT
              : EXISTING_TAG_COMMIT;
          return ok(refs.map((ref) => `${commit}\t${ref}\n`).join(""));
        }
        if (command === "git" && args[0] === "clone") {
          const destination = args.at(-1);
          copyContents(root, destination);
          return ok();
        }
        if (command === "git" && args[0] === "-C" && args[2] === "rev-parse") {
          return ok(
            `${args.at(-1).includes(trust.setTag) || args.at(-1).includes(REVOCATION_TAG) || !args.at(-1).startsWith("refs/tags/") ? COMMIT : EXISTING_TAG_COMMIT}\n`,
          );
        }
        if (command === "git" && args[0] === "-C" && args[2] === "checkout") {
          return ok();
        }
        if (
          command === "git" &&
          args[0] === "-C" &&
          ["fetch", "diff"].includes(args[2])
        ) {
          return ok();
        }
        if (command === "go") {
          if (args.includes("./cmd/publisher-trust")) {
            return ok(`${JSON.stringify(trust)}\n`);
          }
          const packageRoot = args.at(-1);
          const form = plan.forms.find((candidate) =>
            packageRoot.endsWith(candidate.locator.sourcePath),
          );
          if (form) return ok(`${JSON.stringify(form.locator)}\n`);
          const retained = (plan.retainedPackages ?? []).find((candidate) =>
            packageRoot.endsWith(candidate.sourcePath),
          );
          return retained
            ? ok(
                `${JSON.stringify({
                  apiVersion: "packages.forms.takoform.com/v1alpha5",
                  releaseId: retained.releaseId,
                  artifactId: retained.artifactId,
                  tag: retained.tag,
                  sourcePath: retained.sourcePath,
                })}\n`,
              )
            : fail("unknown package");
        }
        return fail(
          `unexpected read-only command: ${command} ${args.join(" ")}`,
        );
      },
    };

    const evidence = verifyPublicPublication(plan, trust, dependencies, {
      expectedCommit: COMMIT,
    });
    expect(evidence.status).toBe("VERIFIED");
    expect(evidence.currentPackageCount).toBe(17);
    expect(evidence.releaseRootCount).toBe(19);
    expect(evidence.releaseTagCount).toBe(19);
    expect(evidence.tagCount).toBe(17);
    expect(evidence.revocationTagCount).toBe(1);
    expect(evidence.revocationTags).toEqual([
      {
        tag: REVOCATION_TAG,
        commit: COMMIT,
        statementVersion: "1.0.0",
        statementDigest: trust.statements[0].statementDigest,
      },
    ]);
    expect(evidence.tags.map((tag) => tag.tag)).toEqual(
      plan.forms.map((form) => form.locator.tag),
    );
    expect(
      evidence.tags.every((tag) => tag.commit === EXISTING_TAG_COMMIT),
    ).toBe(true);
    expect(evidence.retainedTags).toHaveLength(2);
    expect(
      evidence.retainedTags.every((tag) => tag.commit === EXISTING_TAG_COMMIT),
    ).toBe(true);
    extraReleaseTag = true;
    expect(() =>
      verifyPublicPublication(plan, trust, dependencies, {
        expectedCommit: COMMIT,
      }),
    ).toThrow(/public Form Package release refs are/);
    extraReleaseTag = false;
    missingRetainedTag = true;
    expect(() =>
      verifyPublicPublication(plan, trust, dependencies, {
        expectedCommit: COMMIT,
      }),
    ).toThrow(/public Form Package release refs are/);
    expect(
      calls.every((call) => call.command !== "git" || call.args[0] !== "push"),
    ).toBe(true);
  });

  test("credential-free Git and Go commands discard inherited authentication configuration", () => {
    const sourceEnvironment = {
      PATH: "/usr/bin",
      GH_TOKEN: "secret",
      GIT_ASKPASS: "/tmp/askpass",
      GIT_CONFIG_COUNT: "1",
      GIT_CONFIG_KEY_0: "http.extraHeader",
      GIT_CONFIG_VALUE_0: "Authorization: bearer secret",
      GIT_CONFIG: "/tmp/private-git-config",
      GIT_CONFIG_PARAMETERS: "'http.proxy'='https://user:secret@proxy.example'",
      GIT_CONFIG_SYSTEM: "/tmp/private-system-git-config",
      GIT_DIR: "/tmp/other.git",
      GIT_WORK_TREE: "/tmp/other-tree",
      GOAUTH: "netrc",
      GOENV: "/tmp/private-goenv",
      GOPRIVATE: "private.example",
      GONOPROXY: "private.example",
      GONOSUMDB: "private.example",
      GOPROXY: "https://token:secret@private.example",
      NETRC: "/tmp/private-netrc",
    };
    const invocation = credentialFreeInvocation(
      "git",
      ["ls-remote", REPOSITORY_URL, "refs/heads/main"],
      sourceEnvironment,
    );

    expect(invocation.command).toBe("git");
    expect(invocation.args.slice(0, 4)).toEqual([
      "-c",
      "credential.helper=",
      "-c",
      "http.extraHeader=",
    ]);
    expect(invocation.args.slice(4)).toEqual([
      "ls-remote",
      REPOSITORY_URL,
      "refs/heads/main",
    ]);
    expect(invocation.env.PATH).toBe("/usr/bin");
    expect(invocation.env.GH_TOKEN).toBeUndefined();
    expect(invocation.env.GIT_ASKPASS).toBeUndefined();
    expect(invocation.env.GIT_CONFIG_COUNT).toBeUndefined();
    expect(invocation.env.GIT_CONFIG_KEY_0).toBeUndefined();
    expect(invocation.env.GIT_CONFIG_VALUE_0).toBeUndefined();
    expect(invocation.env.GIT_CONFIG).toBeUndefined();
    expect(invocation.env.GIT_CONFIG_PARAMETERS).toBeUndefined();
    expect(invocation.env.GIT_CONFIG_SYSTEM).toBeUndefined();
    expect(invocation.env.GIT_DIR).toBeUndefined();
    expect(invocation.env.GIT_WORK_TREE).toBeUndefined();
    expect(invocation.env.GIT_TERMINAL_PROMPT).toBe("0");
    expect(invocation.env.GIT_CONFIG_NOSYSTEM).toBe("1");
    expect(invocation.env.GIT_CONFIG_GLOBAL).toBe("/dev/null");
    expect(invocation.env.GIT_OPTIONAL_LOCKS).toBe("0");

    const goInvocation = credentialFreeInvocation(
      "go",
      ["run", "./cmd/form-package", "verify", "./package"],
      sourceEnvironment,
    );
    expect(goInvocation.args).toEqual([
      "run",
      "./cmd/form-package",
      "verify",
      "./package",
    ]);
    expect(goInvocation.env.GOAUTH).toBe("off");
    expect(goInvocation.env.GOENV).toBe("off");
    expect(goInvocation.env.GOFLAGS).toBe("-mod=readonly");
    expect(goInvocation.env.GOPRIVATE).toBe("");
    expect(goInvocation.env.GONOPROXY).toBe("");
    expect(goInvocation.env.GONOSUMDB).toBe("");
    expect(goInvocation.env.GOPROXY).toBe("https://proxy.golang.org");
    expect(goInvocation.env.GOSUMDB).toBe("sum.golang.org");
    expect(goInvocation.env.GOTOOLCHAIN).toBe("local");
    expect(goInvocation.env.GOWORK).toBe("off");
    expect(goInvocation.env.NETRC).toBe("/dev/null");
  });
});

function makePlan(
  repositoryRoot = mkdtempSync(path.join(tmpdir(), "takoform-deploy-plan-")),
) {
  const forms = Array.from({ length: 17 }, (_, index) => {
    const hex = `${(index + 1).toString(16).padStart(2, "0")}`.repeat(32);
    const releaseId = `k-${String.fromCharCode(97 + index)}`;
    const artifactId = `sha256-${hex}`;
    return {
      kind: `Fixture${index}`,
      packageDigest: `sha256:${hex}`,
      locator: {
        apiVersion: "packages.forms.takoform.com/v1alpha5",
        releaseId,
        artifactId,
        tag: `forms/${releaseId}/${artifactId}`,
        sourcePath: `forms/releases/${releaseId}/${artifactId}`,
      },
    };
  });
  const retainedPackages = [
    {
      formRef: {
        apiVersion: "edge.forms.takoform.com",
        kind: "WorkerVersion",
        definitionVersion: "0.2.0",
        schemaDigest: `sha256:${"a".repeat(64)}`,
      },
      packageDigest: `sha256:${"b".repeat(64)}`,
      releaseId: "retained-worker-version",
      artifactId: `sha256-${"b".repeat(64)}`,
      tag: `forms/retained-worker-version/sha256-${"b".repeat(64)}`,
      sourcePath: `forms/releases/retained-worker-version/sha256-${"b".repeat(64)}`,
    },
    {
      formRef: {
        apiVersion: "edge.forms.takoform.com",
        kind: "WorkerDeployment",
        definitionVersion: "0.1.0",
        schemaDigest: `sha256:${"c".repeat(64)}`,
      },
      packageDigest: `sha256:${"d".repeat(64)}`,
      releaseId: "retained-worker-deployment",
      artifactId: `sha256-${"d".repeat(64)}`,
      tag: `forms/retained-worker-deployment/sha256-${"d".repeat(64)}`,
      sourcePath: `forms/releases/retained-worker-deployment/sha256-${"d".repeat(64)}`,
    },
  ];
  return {
    repositoryRoot,
    family: "edge.forms.takoform.com",
    formCount: forms.length,
    currentPackageCount: forms.length,
    retainedPackageCount: retainedPackages.length,
    releaseRootCount: forms.length + retainedPackages.length,
    forms,
    retainedPackages,
  };
}

function makeCommandDependencies(
  plan,
  {
    existingRemoteTag = "",
    existingRemoteTagDiverges = false,
    pushExitCode = 0,
    remoteMainCommit = SOURCE_COMMIT,
    remoteTags = new Map(),
    retainedTagMode = "all",
    signedVerifierDiverges = false,
    trustReport = makeTrustReport(plan),
    previousTrustReport = undefined,
    trustError = "",
    verifyPublicPublication = undefined,
  } = {},
) {
  if (retainedTagMode !== "missing") {
    for (const retained of plan.retainedPackages ?? []) {
      if (!remoteTags.has(retained.tag)) {
        remoteTags.set(retained.tag, EXISTING_TAG_COMMIT);
      }
    }
  }
  if (retainedTagMode === "divergent" && plan.retainedPackages?.[0]) {
    remoteTags.set(plan.retainedPackages[0].tag, COMMIT);
  }
  if (retainedTagMode === "extra") {
    remoteTags.set(
      `forms/unlisted/sha256-${"e".repeat(64)}`,
      EXISTING_TAG_COMMIT,
    );
  }
  const calls = [];
  let stdout = "";
  let stderr = "";
  const run = (command, args) => {
    calls.push({ command, args });
    if (command === "bun" && args[0] === "run") return ok();
    if (command === "go" && args.includes("./cmd/form-package")) {
      const packageRoot = args.at(-1);
      const retained = (plan.retainedPackages ?? []).find((entry) =>
        packageRoot.endsWith(entry.sourcePath),
      );
      return retained
        ? ok(
            `${JSON.stringify({
              apiVersion: "packages.forms.takoform.com/v1alpha5",
              releaseId: retained.releaseId,
              artifactId: retained.artifactId,
              tag: retained.tag,
              sourcePath: retained.sourcePath,
            })}\n`,
          )
        : fail("unknown package");
    }
    if (command !== "git") return ok();
    if (args[0] === "status") return ok();
    if (args[0] === "symbolic-ref") return ok("main\n");
    if (args[0] === "remote" && args.at(-1) === "origin")
      return ok(`${REPOSITORY_URL}\n`);
    if (args[0] === "rev-parse") return ok(`${COMMIT}\n`);
    if (args[0] === "-C" && args[2] === "rev-parse") {
      if (args[3] === "HEAD") return ok(`${remoteMainCommit}\n`);
      const tag = args[3]
        ?.replace(/^refs\/tags\//u, "")
        .replace(/\^\{commit\}$/u, "");
      const commit = remoteTags.get(tag);
      return commit ? ok(`${commit}\n`) : fail();
    }
    if (
      args[0] === "ls-remote" &&
      args[1] === "origin" &&
      args[2] === "refs/heads/main"
    ) {
      return ok(`${remoteMainCommit}\trefs/heads/main\n`);
    }
    if (args[0] === "show-ref") return fail();
    if (args[0] === "ls-remote" && args[1] === "--tags") {
      const requested = args
        .slice(3)
        .filter((value) => value.startsWith("refs/tags/"));
      const lines = [];
      for (const ref of requested) {
        if (ref.endsWith("*")) {
          const prefix = ref.slice("refs/tags/".length, -1);
          for (const [tag, commit] of remoteTags) {
            if (tag.startsWith(prefix)) {
              lines.push(`${commit}\trefs/tags/${tag}`);
            }
          }
          continue;
        }
        const tag = ref.replace(/^refs\/tags\//u, "").replace(/\^\{\}$/u, "");
        const commit = remoteTags.get(tag);
        if (commit) lines.push(`${commit}\t${ref}`);
      }
      if (lines.length > 0) return ok(`${lines.join("\n")}\n`);
      return existingRemoteTag &&
        args.includes(`refs/tags/${existingRemoteTag}`)
        ? ok(`${EXISTING_TAG_COMMIT}\trefs/tags/${existingRemoteTag}\n`)
        : ok();
    }
    if (
      args[0] === "diff" &&
      args[2] === EXISTING_TAG_COMMIT &&
      args[3] === SOURCE_COMMIT &&
      existingRemoteTagDiverges &&
      args.includes(
        plan.forms.find((form) => form.locator.tag === existingRemoteTag)
          ?.locator.sourcePath,
      )
    ) {
      return fail("package bytes differ");
    }
    if (
      args[0] === "diff" &&
      args[1] === "--quiet" &&
      retainedTagMode === "divergent" &&
      args[2] === COMMIT &&
      args[3] === SOURCE_COMMIT &&
      args.includes(plan.retainedPackages?.[0]?.sourcePath)
    ) {
      return fail("package bytes differ");
    }
    if (
      args[0] === "diff" &&
      args[1] === "--quiet" &&
      args.includes("cmd/publisher-trust") &&
      signedVerifierDiverges
    ) {
      return fail("publisher verifier differs");
    }
    if (args[0] === "push")
      return { exitCode: pushExitCode, stdout: "", stderr: "push fixture" };
    return ok();
  };
  return {
    calls,
    get stdout() {
      return stdout;
    },
    get stderr() {
      return stderr;
    },
    dependencies: {
      readPlan: () => plan,
      readTrustSet: ({ trustSet }) => {
        if (trustError) throw new Error(trustError);
        if (previousTrustReport && trustSet === PREVIOUS_SET)
          return previousTrustReport;
        return trustReport;
      },
      ...(verifyPublicPublication ? { verifyPublicPublication } : {}),
      run(command, args) {
        return run(command, args);
      },
      runReadOnly(command, args) {
        return run(command, args);
      },
      stdout(value) {
        stdout += value;
      },
      stderr(value) {
        stderr += value;
      },
    },
  };
}

function makeAdvancementTrustReport(plan) {
  const report = makeTrustReport(plan);
  const previous = report.checkpoint;
  const statementDigest = `sha256:${"c".repeat(64)}`;
  const checkpointDigest = `sha256:${"d".repeat(64)}`;
  report.checkpoint = {
    status: "verified",
    checkpointVersion: "1.0.0",
    entryCount: 1,
    pin: {
      checkpointApiVersion: "trust.forms.takoform.com/v1",
      sequence: 1,
      digest: checkpointDigest,
      entriesDigest: `sha256:${"e".repeat(64)}`,
    },
    bundle: {
      ...previous.bundle,
      subjectDigest: checkpointDigest,
    },
  };
  report.previousCheckpoint = {
    setId: PREVIOUS_SET,
    setTag: `forms/sets/${PREVIOUS_SET}`,
    checkpointVersion: "0.0.0",
    pin: previous.pin,
  };
  report.checkpointHistory = [
    {
      setId: PREVIOUS_SET,
      setTag: `forms/sets/${PREVIOUS_SET}`,
      checkpointVersion: "0.0.0",
      pin: previous.pin,
    },
    {
      setId: SOURCE_COMMIT,
      setTag: `forms/sets/${SOURCE_COMMIT}`,
      checkpointVersion: "1.0.0",
      pin: report.checkpoint.pin,
    },
  ];
  report.revocationTag = REVOCATION_TAG;
  report.revocationTags = [REVOCATION_TAG];
  report.statements = [
    {
      sequence: 1,
      statementVersion: "1.0.0",
      statementDigest,
      packageDigest: `sha256:${"f".repeat(64)}`,
      formRef: plan.forms[0].formRef ?? {
        apiVersion: "edge.forms.takoform.com",
        kind: plan.forms[0].kind,
        definitionVersion: "0.1.0",
        schemaDigest: `sha256:${"1".repeat(64)}`,
      },
      sourcePath: "forms/revocations/1.0.0.json",
      tag: REVOCATION_TAG,
    },
  ];
  return report;
}

function makeSecondAdvancementTrustReport(plan) {
  const report = makeAdvancementTrustReport(plan);
  const previousCheckpoint = report.checkpoint;
  report.checkpoint = {
    status: "verified",
    checkpointVersion: "1.1.0",
    entryCount: 2,
    pin: {
      checkpointApiVersion: "trust.forms.takoform.com/v1",
      sequence: 2,
      digest: `sha256:${"2".repeat(64)}`,
      entriesDigest: `sha256:${"3".repeat(64)}`,
    },
    bundle: {
      ...previousCheckpoint.bundle,
      subjectDigest: `sha256:${"2".repeat(64)}`,
    },
  };
  report.previousCheckpoint = {
    setId: PREVIOUS_SET,
    setTag: `forms/sets/${PREVIOUS_SET}`,
    checkpointVersion: "1.0.0",
    pin: previousCheckpoint.pin,
  };
  report.checkpointHistory = [
    {
      setId: GENESIS_SET,
      setTag: `forms/sets/${GENESIS_SET}`,
      checkpointVersion: "0.0.0",
      pin: report.checkpointHistory[0].pin,
    },
    {
      setId: PREVIOUS_SET,
      setTag: `forms/sets/${PREVIOUS_SET}`,
      checkpointVersion: "1.0.0",
      pin: previousCheckpoint.pin,
    },
    {
      setId: SOURCE_COMMIT,
      setTag: `forms/sets/${SOURCE_COMMIT}`,
      checkpointVersion: "1.1.0",
      pin: report.checkpoint.pin,
    },
  ];
  report.revocationTag = SECOND_REVOCATION_TAG;
  report.revocationTags = [REVOCATION_TAG, SECOND_REVOCATION_TAG];
  report.statements.push({
    sequence: 2,
    statementVersion: "1.1.0",
    statementDigest: `sha256:${"4".repeat(64)}`,
    packageDigest: `sha256:${"5".repeat(64)}`,
    formRef: report.statements[0].formRef,
    sourcePath: "forms/revocations/1.1.0.json",
    tag: SECOND_REVOCATION_TAG,
  });
  return report;
}

function makePreviousAdvancementTrustReport(plan) {
  const report = trustAtCommit(makeAdvancementTrustReport(plan), PREVIOUS_SET);
  report.previousCheckpoint.setId = GENESIS_SET;
  report.previousCheckpoint.setTag = `forms/sets/${GENESIS_SET}`;
  report.checkpointHistory[0].setId = GENESIS_SET;
  report.checkpointHistory[0].setTag = `forms/sets/${GENESIS_SET}`;
  return report;
}

function trustAtCommit(report, commit) {
  return JSON.parse(JSON.stringify(report).replaceAll(SOURCE_COMMIT, commit));
}

function makeTrustReport(plan) {
  const trustedRootDigest =
    "sha256:6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66";
  const publisherIdentity =
    "https://github.com/tako0614/takoform-forms/.github/workflows/form-package-signing.yml@refs/heads/main";
  const publisherBundle = (subjectDigest, bundleByte) => ({
    status: "verified",
    subjectDigest,
    bundleDigest: `sha256:${bundleByte.repeat(64)}`,
    trustedRootDigest,
    oidcIssuer: "https://token.actions.githubusercontent.com",
    sourceRepository: "https://github.com/tako0614/takoform-forms",
    workflow:
      "https://github.com/tako0614/takoform-forms/.github/workflows/form-package-signing.yml",
    ref: "refs/heads/main",
    publisherIdentity,
    sourceCommit: SOURCE_COMMIT,
    workflowCommit: SOURCE_COMMIT,
    buildConfigCommit: SOURCE_COMMIT,
    transparencyLogVerified: true,
    transparencyLogThreshold: 1,
  });
  return {
    status: "verified",
    coreVersion: "v1.1.0",
    family: plan.family,
    setId: SOURCE_COMMIT,
    setTag: `forms/sets/${SOURCE_COMMIT}`,
    packageCount: plan.formCount,
    publisherIdentity,
    sourceCommit: SOURCE_COMMIT,
    workflowCommit: SOURCE_COMMIT,
    buildConfigCommit: SOURCE_COMMIT,
    checkpoint: {
      status: "verified",
      checkpointVersion: "0.0.0",
      entryCount: 0,
      pin: {
        checkpointApiVersion: "trust.forms.takoform.com/v1",
        sequence: 0,
        digest:
          "sha256:35c5c4cdc6cd6c4beaec8ba273091be10ae02c0d6f49861f97062fd59f9e8f66",
        entriesDigest:
          "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945",
      },
      bundle: publisherBundle(
        "sha256:35c5c4cdc6cd6c4beaec8ba273091be10ae02c0d6f49861f97062fd59f9e8f66",
        "a",
      ),
    },
    checkpointHistory: [
      {
        setId: SOURCE_COMMIT,
        setTag: `forms/sets/${SOURCE_COMMIT}`,
        checkpointVersion: "0.0.0",
        pin: {
          checkpointApiVersion: "trust.forms.takoform.com/v1",
          sequence: 0,
          digest:
            "sha256:35c5c4cdc6cd6c4beaec8ba273091be10ae02c0d6f49861f97062fd59f9e8f66",
          entriesDigest:
            "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945",
        },
      },
    ],
    revocationTags: [],
    statements: [],
    packages: plan.forms.map((form) => ({
      kind: form.kind,
      packageDigest: form.packageDigest,
      locator: form.locator,
      bundle: publisherBundle(form.packageDigest, "b"),
    })),
  };
}

function writePublicFixture(plan) {
  for (const form of plan.forms) {
    const directory = path.join(
      plan.repositoryRoot,
      ...form.locator.sourcePath.split("/"),
    );
    mkdirSync(directory, { recursive: true });
    writeFileSync(
      path.join(directory, "package-index.json"),
      `${JSON.stringify({ kind: form.kind })}\n`,
    );
  }
  for (const retained of plan.retainedPackages ?? []) {
    const directory = path.join(
      plan.repositoryRoot,
      ...retained.sourcePath.split("/"),
    );
    mkdirSync(directory, { recursive: true });
    writeFileSync(
      path.join(directory, "package-index.json"),
      `${JSON.stringify({ formRef: retained.formRef })}\n`,
    );
  }
}

function copyContents(source, destination) {
  mkdirSync(destination, { recursive: true });
  for (const name of readdirSync(source)) {
    cpSync(path.join(source, name), path.join(destination, name), {
      recursive: true,
    });
  }
}

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function fail(stderr = "") {
  return { exitCode: 1, stdout: "", stderr };
}
