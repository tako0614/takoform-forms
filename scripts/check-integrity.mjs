#!/usr/bin/env node

// Verify the extracted data without consulting the predecessor checkout. The
// baseline is a checked-in byte map; generation and Go validators provide the
// semantic checks layered on top of it.
import { createHash } from "node:crypto";
import { existsSync, lstatSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const baselinePath = path.join(root, "integrity", "source-baseline.json");
const baseline = JSON.parse(readFileSync(baselinePath, "utf8"));
const failures = [];
const abandonedPrepublicationRelative =
  "forms/trust/abandoned-prepublication.json";
const supplementalIntegrityPaths = [abandonedPrepublicationRelative];

if (baseline.format !== "takoform-forms.source-baseline@v1") {
  failures.push(
    `unexpected baseline format ${JSON.stringify(baseline.format)}`,
  );
}
if (baseline.source?.commit !== "220d37b284d8288e6e12d31375ecfdca6a5f15c5") {
  failures.push("baseline is not bound to the Provider tombstone commit");
}

function digest(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function isSafeRelative(value) {
  return (
    typeof value === "string" &&
    value.length > 0 &&
    !path.isAbsolute(value) &&
    !value.includes("\\") &&
    value
      .split("/")
      .every(
        (segment) => segment.length > 0 && segment !== "." && segment !== "..",
      )
  );
}

function pathUnderRoot(relative, label) {
  if (!isSafeRelative(relative)) {
    failures.push(`${label}: unsafe relative path ${JSON.stringify(relative)}`);
    return null;
  }
  return path.join(root, relative);
}

const expectedPaths = Object.keys(baseline.files ?? {}).sort();
const expectedIntegrityPaths = [
  ...new Set([...expectedPaths, ...supplementalIntegrityPaths]),
].sort();
for (const relative of expectedPaths) {
  const expected = baseline.files[relative];
  const absolute = pathUnderRoot(relative, "baseline path");
  if (!absolute) continue;
  if (!existsSync(absolute)) {
    failures.push(`${relative}: baseline file is missing`);
    continue;
  }
  const info = lstatSync(absolute);
  if (!info.isFile()) {
    failures.push(`${relative}: baseline path is not a regular file`);
    continue;
  }
  const bytes = readFileSync(absolute);
  if (bytes.length !== expected.bytes || digest(bytes) !== expected.sha256) {
    failures.push(`${relative}: source-baseline byte digest drift`);
  }
}

function walk(directory, relative = "") {
  const paths = [];
  for (const name of readdirSync(directory).sort()) {
    const child = path.join(directory, name);
    const childRelative = relative ? `${relative}/${name}` : name;
    const info = lstatSync(child);
    if (info.isDirectory()) paths.push(...walk(child, childRelative));
    else if (info.isFile() && name.endsWith(".json")) paths.push(childRelative);
    else if (info.isFile()) {
      failures.push(
        `${childRelative}: unexpected non-schema file in normative root`,
      );
    } else if (!info.isFile())
      failures.push(`${childRelative}: unsupported filesystem entry`);
  }
  return paths;
}

const trackedRoots = [
  "forms/candidates",
  "interfaces/candidates",
  "bindings/candidates",
  "conformance/takoform-v1/families",
];
const actualPaths = trackedRoots
  .flatMap((relative) => walk(path.join(root, relative), relative))
  .concat("forms/retained-packages.json", ...supplementalIntegrityPaths)
  .sort();
if (actualPaths.join("\n") !== expectedIntegrityPaths.join("\n")) {
  const expected = new Set(expectedIntegrityPaths);
  const actual = new Set(actualPaths);
  for (const relative of expectedIntegrityPaths)
    if (!actual.has(relative))
      failures.push(`${relative}: baseline path is not present`);
  for (const relative of actualPaths)
    if (!expected.has(relative))
      failures.push(`${relative}: unlisted normative file`);
}

const familyIndexPath = path.join(
  root,
  "forms/candidates/current-family-index.json",
);
const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
if (familyIndex.format !== "takoform.current-family-index@v1")
  failures.push("current-family index format drift");
if (
  !Array.isArray(familyIndex.families) ||
  familyIndex.families.length !== baseline.counts.families
) {
  failures.push(
    `family count drift: got ${familyIndex.families?.length}, want ${baseline.counts.families}`,
  );
}
let formCount = 0;
const familyGroups = new Set();
for (const family of familyIndex.families ?? []) {
  if (familyGroups.has(family.group))
    failures.push(`duplicate family ${family.group}`);
  familyGroups.add(family.group);
  formCount += family.formCount ?? 0;
  if (!/^\S+\.forms\.takoform\.com$/u.test(family.group))
    failures.push(`unsafe current family ${family.group}`);
  const expectedCandidatePath = `forms/candidates/${family.group}/candidate-set.json`;
  const candidateRelative = family.candidateSet;
  if (candidateRelative !== expectedCandidatePath) {
    failures.push(
      `${family.group}: candidate-set path is not the current family path`,
    );
  }
  const candidatePath = pathUnderRoot(
    candidateRelative,
    `${family.group} candidate-set`,
  );
  if (!candidatePath || !candidateRelative.startsWith("forms/candidates/")) {
    failures.push(
      `${family.group}: candidate-set path escapes the candidate root`,
    );
    continue;
  }
  if (!existsSync(candidatePath)) {
    failures.push(`${family.group}: candidate set is missing`);
    continue;
  }
  const candidateBytes = readFileSync(candidatePath);
  if (digest(candidateBytes) !== family.sha256)
    failures.push(`${family.group}: candidate-set digest does not match index`);
  const candidate = JSON.parse(candidateBytes);
  if (
    candidate.family !== family.group ||
    !Array.isArray(candidate.forms) ||
    candidate.forms.length !== family.formCount
  ) {
    failures.push(`${family.group}: candidate-set form count or group drift`);
  }
  for (const entry of candidate.forms ?? []) {
    const packageRelative = entry.path;
    const packageRoot = pathUnderRoot(
      packageRelative,
      `${family.group}/${entry.kind} package`,
    );
    if (
      !packageRoot ||
      !packageRelative.startsWith(`forms/candidates/${family.group}/`)
    ) {
      failures.push(
        `${family.group}/${entry.kind}: package path escapes the family candidate root`,
      );
      continue;
    }
    const definitionPath = path.join(packageRoot, "definition.json");
    const indexPath = path.join(packageRoot, "package-index.json");
    if (!existsSync(definitionPath) || !existsSync(indexPath)) {
      failures.push(
        `${family.group}/${entry.kind}: package files are incomplete`,
      );
      continue;
    }
    const definition = JSON.parse(readFileSync(definitionPath, "utf8"));
    const packageIndex = JSON.parse(readFileSync(indexPath, "utf8"));
    if (
      definition.apiVersion !== family.group ||
      definition.kind !== entry.kind ||
      definition.role !== entry.role
    ) {
      failures.push(`${family.group}/${entry.kind}: candidate identity drift`);
    }
    if (
      packageIndex.formRef?.schemaDigest !== entry.formRef?.schemaDigest ||
      packageIndex.formRef?.kind !== entry.kind
    ) {
      failures.push(`${family.group}/${entry.kind}: package FormRef drift`);
    }
  }
}
if (formCount !== baseline.counts.forms)
  failures.push(
    `Form count drift: got ${formCount}, want ${baseline.counts.forms}`,
  );
if (familyGroups.has("edge.forms.takoform.com/v1beta1"))
  failures.push("retained edge/v1beta1 family is not a current candidate");

const retainedInventoryPath = path.join(root, "forms/retained-packages.json");
if (!existsSync(retainedInventoryPath)) {
  failures.push("retained package inventory is missing");
} else {
  const retainedInventory = JSON.parse(
    readFileSync(retainedInventoryPath, "utf8"),
  );
  if (
    retainedInventory.format !== "takoform.retained-package-inventory@v1" ||
    retainedInventory.family !== "edge.forms.takoform.com" ||
    !Array.isArray(retainedInventory.packages) ||
    retainedInventory.packages.length !== 2
  ) {
    failures.push("retained package inventory format/count drift");
  }
  const retainedKeys = new Set();
  for (const entry of retainedInventory.packages ?? []) {
    const key = `${entry.formRef?.kind}@${entry.formRef?.definitionVersion}`;
    if (retainedKeys.has(key))
      failures.push(`duplicate retained package ${key}`);
    retainedKeys.add(key);
    const expectedArtifact = String(entry.packageDigest ?? "").replace(
      ":",
      "-",
    );
    const expectedTag = `forms/${entry.releaseId}/${entry.artifactId}`;
    const expectedSourcePath = `forms/releases/${entry.releaseId}/${entry.artifactId}`;
    if (
      !/^sha256:[0-9a-f]{64}$/u.test(entry.packageDigest ?? "") ||
      entry.artifactId !== expectedArtifact ||
      entry.tag !== expectedTag ||
      entry.sourcePath !== expectedSourcePath ||
      !isSafeRelative(entry.sourcePath)
    ) {
      failures.push(`retained package ${key}: locator drift`);
      continue;
    }
    const packageIndexPath = path.join(
      root,
      entry.sourcePath,
      "package-index.json",
    );
    if (!existsSync(packageIndexPath)) {
      failures.push(`retained package ${key}: package index is missing`);
      continue;
    }
    const packageIndex = JSON.parse(readFileSync(packageIndexPath, "utf8"));
    if (
      JSON.stringify(packageIndex.formRef) !== JSON.stringify(entry.formRef)
    ) {
      failures.push(`retained package ${key}: FormRef drift`);
    }
  }
  if (
    !retainedKeys.has("WorkerVersion@0.2.0") ||
    !retainedKeys.has("WorkerDeployment@0.1.0")
  ) {
    failures.push("retained package inventory identities drift");
  }
}

const abandonedPrepublicationPath = path.join(
  root,
  abandonedPrepublicationRelative,
);
const expectedAbandonedPrepublication = [
  {
    formRef: {
      apiVersion: "edge.forms.takoform.com",
      kind: "ObjectBucket",
      definitionVersion: "0.1.0",
      schemaDigest:
        "sha256:eeda7b2fe4450bdd2301a348c27d7ade81b0a94bf9708655875329d72f902c57",
    },
    packageDigest:
      "sha256:52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
    releaseId: "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2pmjvgky3uij2wg23foq",
    artifactId:
      "sha256-52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
  },
  {
    formRef: {
      apiVersion: "edge.forms.takoform.com",
      kind: "WorkerDeployment",
      definitionVersion: "0.2.0",
      schemaDigest:
        "sha256:247d64335cbff296efc0298aa6811f299714fe7187d29aec6f73ed734e978756",
    },
    packageDigest:
      "sha256:f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
    releaseId:
      "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu",
    artifactId:
      "sha256-f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
  },
  {
    formRef: {
      apiVersion: "edge.forms.takoform.com",
      kind: "WorkerVersion",
      definitionVersion: "0.3.0",
      schemaDigest:
        "sha256:e82dce714f8b623ca926379c855ee9e314c83262e5564828ccc37be2dbe05820",
    },
    packageDigest:
      "sha256:d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
    releaseId: "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa",
    artifactId:
      "sha256-d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
  },
];
if (!existsSync(abandonedPrepublicationPath)) {
  failures.push("abandoned prepublication manifest is missing");
} else {
  let abandoned;
  try {
    abandoned = JSON.parse(readFileSync(abandonedPrepublicationPath, "utf8"));
  } catch (error) {
    failures.push(
      `abandoned prepublication manifest is not valid JSON: ${error.message}`,
    );
  }
  if (abandoned) {
    if (
      Object.keys(abandoned).sort().join(",") !==
        "disposition,evidenceOnlyPackages,family,format,setId,setTag" ||
      abandoned.format !== "takoform.abandoned-prepublication@v1" ||
      abandoned.family !== "edge.forms.takoform.com" ||
      abandoned.setId !== "cdd30b711e2c6857b1b4d247b1471f5676904933" ||
      abandoned.setTag !==
        "forms/sets/cdd30b711e2c6857b1b4d247b1471f5676904933" ||
      abandoned.disposition !== "evidence-only" ||
      !Array.isArray(abandoned.evidenceOnlyPackages) ||
      abandoned.evidenceOnlyPackages.length !==
        expectedAbandonedPrepublication.length
    ) {
      failures.push("abandoned prepublication manifest format/count drift");
    } else {
      const seen = new Set();
      for (const entry of abandoned.evidenceOnlyPackages) {
        const key = `${entry.formRef?.kind}@${entry.formRef?.definitionVersion}`;
        const expected = expectedAbandonedPrepublication.find(
          (candidate) =>
            `${candidate.formRef.kind}@${candidate.formRef.definitionVersion}` ===
            key,
        );
        const expectedArtifact = String(entry.packageDigest ?? "").replace(
          ":",
          "-",
        );
        const expectedTag = `forms/${entry.releaseId}/${entry.artifactId}`;
        const expectedSourcePath = `forms/releases/${entry.releaseId}/${entry.artifactId}`;
        if (
          !expected ||
          seen.has(key) ||
          Object.keys(entry).sort().join(",") !==
            "artifactId,formRef,packageDigest,releaseId,sourcePath,tag" ||
          JSON.stringify(entry.formRef) !== JSON.stringify(expected.formRef) ||
          entry.packageDigest !== expected.packageDigest ||
          entry.releaseId !== expected.releaseId ||
          entry.artifactId !== expected.artifactId ||
          entry.tag !== expectedTag ||
          entry.sourcePath !== expectedSourcePath ||
          entry.artifactId !== expectedArtifact ||
          !isSafeRelative(entry.sourcePath)
        ) {
          failures.push(
            `abandoned prepublication package ${key}: identity drift`,
          );
          continue;
        }
        seen.add(key);
        const packageIndexPath = path.join(
          root,
          entry.sourcePath,
          "package-index.json",
        );
        if (!existsSync(packageIndexPath)) {
          failures.push(
            `abandoned prepublication package ${key}: package index is missing`,
          );
          continue;
        }
        const packageIndex = JSON.parse(readFileSync(packageIndexPath, "utf8"));
        if (
          JSON.stringify(packageIndex.formRef) !== JSON.stringify(entry.formRef)
        ) {
          failures.push(
            `abandoned prepublication package ${key}: FormRef drift`,
          );
        }
      }
      if (seen.size !== expectedAbandonedPrepublication.length) {
        failures.push("abandoned prepublication manifest identities drift");
      }
    }
  }
}

const interfaceSet = JSON.parse(
  readFileSync(
    path.join(root, "interfaces/candidates/v1alpha1/candidate-set.json"),
    "utf8",
  ),
);
const bindingSet = JSON.parse(
  readFileSync(
    path.join(root, "bindings/candidates/v1alpha2/candidate-set.json"),
    "utf8",
  ),
);
if (interfaceSet.interfaces?.length !== baseline.counts.interfaces)
  failures.push("Interface candidate count drift");
if (bindingSet.bindings?.length !== baseline.counts.bindings)
  failures.push("Binding candidate count drift");
const interfaceCandidatePath = pathUnderRoot(
  familyIndex.interfaceCandidateSet?.path,
  "Interface candidate-set",
);
const bindingCandidatePath = pathUnderRoot(
  familyIndex.bindingCandidateSet?.path,
  "Binding candidate-set",
);
if (
  familyIndex.interfaceCandidateSet?.path !==
  "interfaces/candidates/v1alpha1/candidate-set.json"
) {
  failures.push("Interface candidate-set path drift");
}
if (
  familyIndex.bindingCandidateSet?.path !==
  "bindings/candidates/v1alpha2/candidate-set.json"
) {
  failures.push("Binding candidate-set path drift");
}
if (
  !interfaceCandidatePath ||
  !interfaceCandidatePath
    .toString()
    .includes(`${path.sep}interfaces${path.sep}candidates${path.sep}`)
) {
  failures.push("Interface candidate-set path escapes the interface root");
} else if (
  familyIndex.interfaceCandidateSet?.sha256 !==
  digest(readFileSync(interfaceCandidatePath))
) {
  failures.push(
    "Interface candidate-set digest does not match current-family index",
  );
}
if (
  !bindingCandidatePath ||
  !bindingCandidatePath
    .toString()
    .includes(`${path.sep}bindings${path.sep}candidates${path.sep}`)
) {
  failures.push("Binding candidate-set path escapes the binding root");
} else if (
  familyIndex.bindingCandidateSet?.sha256 !==
  digest(readFileSync(bindingCandidatePath))
) {
  failures.push(
    "Binding candidate-set digest does not match current-family index",
  );
}

if (failures.length)
  throw new Error(`source integrity check failed:\n${failures.join("\n")}`);
console.log(
  `source baseline matches ${expectedPaths.length} canonical files; ${formCount} Forms, ${interfaceSet.interfaces.length} Interfaces, ${bindingSet.bindings.length} Bindings`,
);
