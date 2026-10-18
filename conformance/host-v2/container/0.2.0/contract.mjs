// Publisher-local parsed-value checks for the Container 0.2.0 source.
// Raw JSON, replay comparison, capability transport, registry access and Host
// custody remain the common API / implementing Host's responsibility.
import {
  ContainerContractError,
  normalizeContainerSpec as normalizeBaselineSpec,
} from "../contract.mjs";

export { ContainerContractError };
export const CONTAINER_URL =
  "https://edge.forms.takoform.com/forms/Container/0.2.0/";
export const MAX_SECRET_NAMES = 64;
export const MAX_SECRET_VALUE_SCALARS = 16384;
export const MAX_SECRET_TOTAL_BYTES = 65536;
export const MAX_IMAGE_DOCUMENT_BYTES = 4194304;

const secretName = /^[A-Z][A-Z0-9_]{0,63}$/u;
const reservedName = /^(?:CLOUDFLARE_|TAKOFORM_)/u;
const digest = /^sha256:[a-f0-9]{64}$/u;
const manifestTypes = new Set([
  "application/vnd.oci.image.manifest.v1+json",
  "application/vnd.docker.distribution.manifest.v2+json",
]);
const indexTypes = new Set([
  "application/vnd.oci.image.index.v1+json",
  "application/vnd.docker.distribution.manifest.list.v2+json",
]);

function requireThat(condition, code = "invalid_spec") {
  if (!condition) throw new ContainerContractError(code);
}

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/** Validates the public spec; requiredSensitiveVars is a set, returned sorted. */
export function normalizeContainerSpec(spec) {
  requireThat(object(spec));
  const declared = Object.hasOwn(spec, "requiredSensitiveVars")
    ? spec.requiredSensitiveVars
    : [];
  // The 0.1.0 checker still validates every other key, including unknown keys.
  const base = normalizeBaselineSpec({ ...spec, requiredSensitiveVars: [] });
  requireThat(Array.isArray(declared) && declared.length <= MAX_SECRET_NAMES);
  const names = new Set();
  for (const name of declared) {
    requireThat(
      typeof name === "string" &&
        secretName.test(name) &&
        !reservedName.test(name) &&
        !names.has(name) &&
        !Object.hasOwn(base.environment, name),
    );
    names.add(name);
  }
  return { ...base, requiredSensitiveVars: [...names].sort() };
}

function secretValue(value) {
  return (
    value !== "" &&
    !value.includes("\0") &&
    !/[\uD800-\uDFFF]/u.test(value) &&
    [...value].length <= MAX_SECRET_VALUE_SCALARS
  );
}

function completeMap(privateInputs, names) {
  // The common envelope is Record<string, string>; other shapes are not specs.
  requireThat(object(privateInputs), "invalid_request");
  const keys = Object.keys(privateInputs);
  requireThat(
    keys.every((key) => typeof privateInputs[key] === "string"),
    "invalid_request",
  );
  requireThat(
    keys.length === names.length && names.every((name) => keys.includes(name)),
  );
  let bytes = 0;
  for (const name of names) {
    requireThat(secretValue(privateInputs[name]));
    bytes += new TextEncoder().encode(privateInputs[name]).length;
  }
  requireThat(bytes <= MAX_SECRET_TOTAL_BYTES);
}

/**
 * Classifies a CREATE or PUT's private inputs. `previousNames` is the name set
 * of the configured secret map: the latest accepted generation's, even if that
 * generation failed. Returns what the new generation receives; values are
 * never returned. `valuesChange` is a fixture aid, not a Form term.
 */
export function classifyPrivateInputs({
  operation,
  spec,
  privateInputs,
  previousNames,
  hostPrivateInputs,
}) {
  requireThat(operation === "create" || operation === "update");
  const names = normalizeContainerSpec(spec).requiredSensitiveVars;
  const supplied = privateInputs !== undefined;
  requireThat(typeof hostPrivateInputs === "boolean");
  if (!hostPrivateInputs && (supplied || names.length > 0))
    throw new ContainerContractError("capability_required");
  if (operation === "create") {
    if (!supplied) {
      requireThat(names.length === 0);
      return { secrets: "none", names, removed: [], valuesChange: false };
    }
    completeMap(privateInputs, names);
    return {
      secrets: names.length ? "set" : "none",
      names,
      removed: [],
      valuesChange: names.length > 0,
    };
  }
  requireThat(Array.isArray(previousNames));
  const previous = [...new Set(previousNames)].sort();
  const removed = previous.filter((name) => !names.includes(name));
  if (supplied) {
    completeMap(privateInputs, names);
    return {
      secrets: "replace",
      names,
      removed,
      valuesChange: names.length > 0 || previous.length > 0,
    };
  }
  // Omission preserves values (common API); it can only drop declared names.
  requireThat(names.every((name) => previous.includes(name)));
  return {
    secrets: removed.length ? "inherit-subset" : "inherit",
    names,
    removed,
    valuesChange: removed.length > 0,
  };
}

/**
 * Selects the executable manifest for a source digest. `source` is the fetched
 * document's media type and parsed JSON; `selectedConfigPlatform` is the
 * platform read from the selected manifest's config.
 */
export function selectExecutableManifest({
  sourceDigest,
  source,
  selectedConfigPlatform,
}) {
  const invalid = (condition) => requireThat(condition, "image_invalid");
  requireThat(digest.test(sourceDigest));
  invalid(object(source) && object(source.document));
  let executableDigest = sourceDigest;
  if (indexTypes.has(source.mediaType)) {
    const descriptors = source.document.manifests;
    invalid(Array.isArray(descriptors));
    for (const descriptor of descriptors)
      invalid(object(descriptor) && digest.test(descriptor.digest));
    const matches = descriptors.filter(
      (descriptor) =>
        descriptor.platform?.os === "linux" &&
        descriptor.platform?.architecture === "amd64",
    );
    invalid(matches.length === 1 && manifestTypes.has(matches[0].mediaType));
    executableDigest = matches[0].digest;
  } else {
    invalid(manifestTypes.has(source.mediaType));
  }
  invalid(
    selectedConfigPlatform?.os === "linux" &&
      selectedConfigPlatform?.architecture === "amd64",
  );
  return { sourceDigest, executableDigest };
}

/** Selected image bytes: config plus layer descriptor sizes of the executed manifest. */
export function selectedImageBytes(manifest, limit) {
  if (!Number.isSafeInteger(limit) || limit < 0)
    throw new TypeError("limit must be a non-negative safe integer");
  const invalid = (condition) => requireThat(condition, "image_invalid");
  invalid(object(manifest) && object(manifest.config));
  invalid(Array.isArray(manifest.layers));
  const descriptors = [manifest.config, ...manifest.layers];
  invalid(
    descriptors.every(
      (entry) =>
        object(entry) &&
        digest.test(entry.digest) &&
        Number.isSafeInteger(entry.size) &&
        entry.size >= 0,
    ),
  );
  const total = descriptors.reduce((sum, entry) => sum + entry.size, 0);
  requireThat(Number.isSafeInteger(total), "image_invalid");
  requireThat(total <= limit, "image_too_large");
  return total;
}
