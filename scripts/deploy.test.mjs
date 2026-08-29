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
      `origin main is ${COMMIT}, expected ${SOURCE_COMMIT}`,
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

  test("verify reads only through the credential-free dependency route and checks all tags and bytes", () => {
    const root = mkdtempSync(
      path.join(tmpdir(), "takoform-public-verify-test-"),
    );
    const plan = makePlan(root);
    const trust = makeTrustReport(plan);
    writePublicFixture(plan);
    const calls = [];
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
          const commit = refs.includes(`refs/tags/${trust.setTag}`)
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
            `${args.at(-1).startsWith("refs/tags/") ? EXISTING_TAG_COMMIT : COMMIT}\n`,
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
          return form
            ? ok(`${JSON.stringify(form.locator)}\n`)
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
    expect(evidence.tagCount).toBe(16);
    expect(evidence.tags.map((tag) => tag.tag)).toEqual(
      plan.forms.map((form) => form.locator.tag),
    );
    expect(
      evidence.tags.every((tag) => tag.commit === EXISTING_TAG_COMMIT),
    ).toBe(true);
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
    expect(invocation.env.GIT_TERMINAL_PROMPT).toBe("0");
    expect(invocation.env.GIT_CONFIG_NOSYSTEM).toBe("1");
    expect(invocation.env.GIT_CONFIG_GLOBAL).toBe("/dev/null");

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
  const forms = Array.from({ length: 16 }, (_, index) => {
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
  return {
    repositoryRoot,
    family: "edge.forms.takoform.com",
    formCount: forms.length,
    forms,
  };
}

function makeCommandDependencies(
  plan,
  {
    existingRemoteTag = "",
    existingRemoteTagDiverges = false,
    pushExitCode = 0,
    remoteMainCommit = SOURCE_COMMIT,
    signedVerifierDiverges = false,
    trustError = "",
  } = {},
) {
  const calls = [];
  let stdout = "";
  let stderr = "";
  const run = (command, args) => {
    calls.push({ command, args });
    if (command === "bun" && args[0] === "run") return ok();
    if (command !== "git") return ok();
    if (args[0] === "status") return ok();
    if (args[0] === "symbolic-ref") return ok("main\n");
    if (args[0] === "remote" && args.at(-1) === "origin")
      return ok(`${REPOSITORY_URL}\n`);
    if (args[0] === "rev-parse") return ok(`${COMMIT}\n`);
    if (
      args[0] === "ls-remote" &&
      args[1] === "origin" &&
      args[2] === "refs/heads/main"
    ) {
      return ok(`${remoteMainCommit}\trefs/heads/main\n`);
    }
    if (args[0] === "show-ref") return fail();
    if (args[0] === "ls-remote" && args[1] === "--tags") {
      return existingRemoteTag &&
        args.includes(`refs/tags/${existingRemoteTag}`)
        ? ok(`${EXISTING_TAG_COMMIT}\trefs/tags/${existingRemoteTag}\n`)
        : ok();
    }
    if (
      args[0] === "diff" &&
      args[2] === EXISTING_TAG_COMMIT &&
      args[3] === SOURCE_COMMIT &&
      existingRemoteTagDiverges
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
      readTrustSet: () => {
        if (trustError) throw new Error(trustError);
        return makeTrustReport(plan);
      },
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
