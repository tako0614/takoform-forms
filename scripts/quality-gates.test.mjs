import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const packageJson = JSON.parse(
  readFileSync(resolve(root, "package.json"), "utf8"),
);
const readme = readFileSync(resolve(root, "forms/README.md"), "utf8");
const rootReadme = readFileSync(resolve(root, "README.md"), "utf8");
const goMod = readFileSync(resolve(root, "go.mod"), "utf8");
const deploySource = readFileSync(resolve(root, "scripts/deploy.mjs"), "utf8");
const publisherSource = readFileSync(
  resolve(root, "internal/publishertrust/publisher.go"),
  "utf8",
);
const signingWorkflow = readFileSync(
  resolve(root, ".github/workflows/form-package-signing.yml"),
  "utf8",
);
const publisherPolicy = JSON.parse(
  readFileSync(resolve(root, "forms/trust/publisher-policy.json"), "utf8"),
);
const trustedRoot = readFileSync(
  resolve(root, "forms/trust/trusted-root.json"),
);

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

  test("keeps released Core v1.1.0 trust verification in the complete gate", () => {
    expect(goMod).toMatch(
      /^\s*github\.com\/tako0614\/takoform\s+v1\.1\.0\s*$/mu,
    );
    expect(packageJson.scripts.check).toContain("bun run check:trust");
    expect(packageJson.scripts["check:trust"]).toBe(
      "go run ./cmd/publisher-trust check --repository .",
    );
    for (const command of [
      "prepare:trust",
      "prepare:revocation",
      "verify:trust",
      "install:trust",
      "recover:trust",
    ]) {
      expect(packageJson.scripts[command]).toContain("./cmd/publisher-trust");
    }
    expect(`${rootReadme}\n${readme}`).not.toContain("Core v1.0.1");
    expect(`${rootReadme}\n${readme}`).not.toContain("unsigned provenance");
  });

  test("pins one publisher authority without a private signing key", () => {
    expect(publisherPolicy).toEqual({
      apiVersion: "trust.forms.takoform.com/v1alpha1",
      kind: "PublisherPolicy",
      oidcIssuer: "https://token.actions.githubusercontent.com",
      ref: "refs/heads/main",
      sourceRepository: "https://github.com/tako0614/takoform-forms",
      workflow:
        "https://github.com/tako0614/takoform-forms/.github/workflows/form-package-signing.yml",
    });
    expect(createHash("sha256").update(trustedRoot).digest("hex")).toBe(
      "6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66",
    );
    expect(signingWorkflow).toContain("workflow_dispatch:");
    expect(signingWorkflow).toContain("contents: read");
    expect(signingWorkflow).toContain("id-token: write");
    expect(signingWorkflow).toContain("cosign sign-blob");
    expect(signingWorkflow).toContain('cosign-release: "v3.0.6"');
    expect(signingWorkflow).toContain("verify-evidence");
    expect(signingWorkflow).toContain("previous_set:");
    expect(signingWorkflow).toContain("statement_version:");
    expect(signingWorkflow).toContain("prepare-advancement");
    expect(signingWorkflow).toContain("genesis signing is first-set-only");
    expect(signingWorkflow).toContain("(.subjects | length == 17)");
    expect(signingWorkflow).not.toContain("statement.sigstore.json");
    expect(signingWorkflow).toContain("retention-days: 1");
    expect(signingWorkflow).not.toContain("contents: write");
    expect(signingWorkflow).not.toContain("COSIGN_PRIVATE_KEY");
    expect(signingWorkflow).not.toMatch(/\bgit\s+push\b/u);
    expect(signingWorkflow).not.toMatch(/\bgh\s+release\b/u);
  });

  test("requires Core signature and revocation evidence before public readback", () => {
    for (const call of [
      "trust.VerifyBundle(",
      "trust.VerifyRevocationCheckpoint(",
      ".CheckNotRevoked(",
      "sameProvenance(",
      "debug.ReadBuildInfo()",
      "os.O_EXCL",
    ]) {
      expect(publisherSource).toContain(call);
    }
    expect(deploySource).toContain('args[1] !== "--trust-set"');
    expect(deploySource).toContain("readTrustSet(");
    expect(deploySource).toContain(
      "CORE_V1_1_0_PACKAGE_TRUST_REVOCATION_VERIFICATION",
    );
    expect(`${publisherSource}\n${deploySource}`).not.toMatch(
      /EvaluateAdmission|CoreAdmissionAdapter|HostAdmission|Specification 1\.1|trust\.forms\.takoform\.com\/v2/u,
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
