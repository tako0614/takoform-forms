// Publisher-local parsed-value checks. Raw JSON, replay and Host authorization
// enforcement remain the common API / implementing Host's responsibility.
export const CONTAINER_URL =
  "https://edge.forms.takoform.com/forms/Container/0.1.0/";
export const WORKER_VERSION_URL =
  "https://edge.forms.takoform.com/forms/WorkerVersion/0.6.0/";
export const INTERFACE_NAME = "container.http";
export const BINDING_NAME = "module-worker.container";
export const BODY_LIMIT = 104857600;
export const DRAIN_MS = 30000;

const envName = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/u;
const bindingName = /^[A-Za-z_$][A-Za-z0-9_$]{0,63}$/u;
const digest = /^sha256:[a-f0-9]{64}$/u;
const revision = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/u;
const token = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/u;
const methods = new Set([
  "GET",
  "HEAD",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS",
]);

export class ContainerContractError extends Error {
  constructor(code = "invalid_spec") {
    super(code);
    this.name = "ContainerContractError";
    this.code = code;
  }
}

function requireThat(condition, code) {
  if (!condition) throw new ContainerContractError(code);
}

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function closed(value, allowed, required = []) {
  requireThat(object(value));
  requireThat(Object.keys(value).every((key) => allowed.includes(key)));
  requireThat(required.every((key) => Object.hasOwn(value, key)));
}

function unicode(value, maximum) {
  return (
    typeof value === "string" &&
    !/[\uD800-\uDFFF]/u.test(value) &&
    [...value].length <= maximum
  );
}

function image(value) {
  if (!unicode(value, 512)) return false;
  const parts = value.split("@");
  if (parts.length !== 2 || !digest.test(parts[1])) return false;
  const [registry, ...repository] = parts[0].split("/");
  if (!repository.length) return false;
  const registryParts = registry.split(":");
  if (registryParts.length > 2) return false;
  const [hostname, port] = registryParts;
  if (
    !hostname
      .split(".")
      .every((label) =>
        /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/u.test(label),
      ) ||
    hostname.length > 253
  )
    return false;
  if (
    port !== undefined &&
    (!/^[1-9][0-9]{0,4}$/u.test(port) || Number(port) > 65535)
  )
    return false;
  const last = repository.pop().split(":");
  if (last.length > 2) return false;
  if (
    last[1] !== undefined &&
    !/^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/u.test(last[1])
  )
    return false;
  repository.push(last[0]);
  return repository.every((segment) =>
    /^[a-z0-9]+(?:[._-][a-z0-9]+)*$/u.test(segment),
  );
}

function healthPath(value) {
  if (!unicode(value, 2048) || !value.startsWith("/")) return false;
  if (value === "/") return true;
  return value
    .slice(1)
    .split("/")
    .every((part) => {
      if (!/^(?:[A-Za-z0-9._~!$&'()*+,;=:@-]|%[0-9A-Fa-f]{2})+$/u.test(part))
        return false;
      try {
        const decoded = decodeURIComponent(part);
        return (
          decoded !== "." &&
          decoded !== ".." &&
          !/[/\\\u0000-\u001F\u007F]/u.test(decoded)
        );
      } catch {
        return false;
      }
    });
}

export function normalizeContainerSpec(spec, privateInputs = {}) {
  closed(
    spec,
    [
      "image",
      "httpPort",
      "healthPath",
      "workloadRevision",
      "environment",
      "requiredSensitiveVars",
      "outboundInternet",
    ],
    ["image", "httpPort", "healthPath", "workloadRevision"],
  );
  requireThat(image(spec.image));
  requireThat(
    Number.isInteger(spec.httpPort) &&
      spec.httpPort >= 1 &&
      spec.httpPort <= 65535,
  );
  requireThat(healthPath(spec.healthPath));
  requireThat(
    typeof spec.workloadRevision === "string" &&
      revision.test(spec.workloadRevision),
  );
  const environment = Object.hasOwn(spec, "environment")
    ? spec.environment
    : {};
  requireThat(object(environment) && Object.keys(environment).length <= 64);
  for (const [name, value] of Object.entries(environment)) {
    requireThat(
      envName.test(name) && !/^(?:CLOUDFLARE_|TAKOFORM_)/u.test(name),
    );
    requireThat(unicode(value, 4096) && !value.includes("\0"));
  }
  const sensitive = Object.hasOwn(spec, "requiredSensitiveVars")
    ? spec.requiredSensitiveVars
    : [];
  requireThat(Array.isArray(sensitive) && sensitive.length === 0);
  closed(privateInputs, []);
  const outbound = Object.hasOwn(spec, "outboundInternet")
    ? spec.outboundInternet
    : false;
  requireThat(typeof outbound === "boolean");
  return {
    image: spec.image,
    httpPort: spec.httpPort,
    healthPath: spec.healthPath,
    workloadRevision: spec.workloadRevision,
    environment: Object.fromEntries(
      Object.entries(environment).sort(([a], [b]) => a.localeCompare(b)),
    ),
    requiredSensitiveVars: [],
    outboundInternet: outbound,
  };
}

export function validateContainerBindings(
  bindings,
  { source, occupiedNames = [], resolve },
) {
  requireThat(
    source?.form === WORKER_VERSION_URL &&
      typeof source.space === "string" &&
      source.space !== "",
  );
  requireThat(typeof source.host === "string" && source.host !== "");
  requireThat(typeof resolve === "function");
  requireThat(Array.isArray(bindings) && bindings.length <= 64);
  const names = new Set(occupiedNames);
  for (const entry of bindings) {
    closed(entry, ["name", "resource"], ["name", "resource"]);
    requireThat(
      typeof entry.name === "string" &&
        bindingName.test(entry.name) &&
        !names.has(entry.name),
    );
    names.add(entry.name);
    closed(entry.resource, ["resourceUid"], ["resourceUid"]);
    requireThat(
      typeof entry.resource.resourceUid === "string" &&
        entry.resource.resourceUid !== "",
    );
    const target = resolve(entry.resource.resourceUid);
    requireThat(
      target?.uid === entry.resource.resourceUid &&
        target.form === CONTAINER_URL &&
        target.space === source.space &&
        target.host === source.host &&
        target.authorized === true &&
        target.deleting !== true,
      "dependency_conflict",
    );
  }
}

export function validateContainerObserved(observed, generation) {
  requireThat(Number.isSafeInteger(generation) && generation >= 1);
  closed(observed, ["ready", "servingGeneration", "imageDigest"]);
  if (Object.keys(observed).length === 0) return;
  requireThat(typeof observed.ready === "boolean");
  const hasGeneration = Object.hasOwn(observed, "servingGeneration");
  requireThat(hasGeneration === Object.hasOwn(observed, "imageDigest"));
  requireThat(!observed.ready || hasGeneration);
  if (hasGeneration) {
    requireThat(
      Number.isSafeInteger(observed.servingGeneration) &&
        observed.servingGeneration >= 1 &&
        observed.servingGeneration <= generation &&
        typeof observed.imageDigest === "string" &&
        digest.test(observed.imageDigest),
    );
  }
}

// Checks inspect only the Request head. Never pull a body to invent its length.
export function validateContainerRequest(request) {
  const bad = (condition) => requireThat(condition, "invalid_request");
  bad(request instanceof Request && !request.bodyUsed);
  const url = new URL(request.url);
  const target = url.pathname + url.search;
  bad(
    ["http:", "https:"].includes(url.protocol) &&
      !url.username &&
      !url.password &&
      !url.hash,
  );
  bad(
    methods.has(request.method) &&
      !/[\\\u0000-\u0020\u007F]/u.test(target) &&
      [...target].length <= 8192,
  );
  bad(!["GET", "HEAD"].includes(request.method) || request.body === null);
  bad(!request.headers.has("upgrade"));
  let count = 0,
    bytes = 0;
  for (const [name, value] of request.headers) {
    if (name === "set-cookie") continue;
    bad(token.test(name) && /^[\t\u0020-\u007E\u0080-\u00FF]*$/u.test(value));
    count++;
    bytes += name.length + value.length + 4;
  }
  for (const value of request.headers.getSetCookie()) {
    count++;
    bytes += "set-cookie".length + value.length + 4;
  }
  bad(count <= 64 && bytes <= 16384);
  const connection = request.headers.get("connection");
  if (connection !== null) {
    const names = connection.split(",").map((name) => name.trim());
    bad(
      names.every((name) => token.test(name)) &&
        !names.some((name) => name.toLowerCase() === "content-length"),
    );
  }
  const length = request.headers.get("content-length");
  if (length !== null) {
    bad(
      /^(?:0|[1-9][0-9]*)$/u.test(length) &&
        Number.isSafeInteger(Number(length)),
    );
    requireThat(Number(length) <= BODY_LIMIT, "request_too_large");
  }
  requireThat(!request.signal.aborted, "request_aborted");
  return {
    method: request.method,
    path: target,
    contentLength: length === null ? null : Number(length),
  };
}
