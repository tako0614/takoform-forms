// Publisher-local checks for the callers of Container 0.2.0: which
// WorkerVersion may bind which exact Container URL, and what
// WorkerDeployment 0.7.0 may select. Host authorization, Resource state and
// the single-active ownership record remain the implementing Host's.
import { ContainerContractError } from "../contract.mjs";

const forms = "https://edge.forms.takoform.com/forms/";
export const CONTAINER_0_1_0 = `${forms}Container/0.1.0/`;
export const CONTAINER_0_2_0 = `${forms}Container/0.2.0/`;
export const WORKER_VERSION_URLS = ["0.5.0", "0.6.0", "0.7.0", "0.8.0"].map(
  (version) => `${forms}WorkerVersion/${version}/`,
);
export const WORKER_DEPLOYMENT_URLS = ["0.4.0", "0.5.0", "0.6.0", "0.7.0"].map(
  (version) => `${forms}WorkerDeployment/${version}/`,
);
/** Container URLs each WorkerVersion's containerBindings may reference. */
export const CONTAINER_TARGETS = {
  [`${forms}WorkerVersion/0.6.0/`]: [CONTAINER_0_1_0],
  [`${forms}WorkerVersion/0.7.0/`]: [CONTAINER_0_1_0],
  [`${forms}WorkerVersion/0.8.0/`]: [CONTAINER_0_1_0, CONTAINER_0_2_0],
};
/** WorkerVersion URLs WorkerDeployment 0.7.0 may select. */
export const DEPLOYMENT_0_7_0_VERSIONS = WORKER_VERSION_URLS;

const bindingName = /^[A-Za-z_$][A-Za-z0-9_$]{0,63}$/u;

function requireThat(condition, code = "invalid_spec") {
  if (!condition) throw new ContainerContractError(code);
}

function closed(value, keys) {
  return (
    value !== null &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    Object.keys(value).length === keys.length &&
    keys.every((key) => Object.hasOwn(value, key))
  );
}

/**
 * Resolves a WorkerVersion's containerBindings and seals each target's exact
 * Container URL. `resolve(uid)` returns the Host's record of that Resource.
 */
export function sealContainerBindings(bindings, { source, resolve }) {
  const targets = CONTAINER_TARGETS[source?.form];
  requireThat(targets && typeof resolve === "function");
  requireThat(Array.isArray(bindings) && bindings.length <= 64);
  const names = new Set();
  return bindings.map((entry) => {
    requireThat(closed(entry, ["name", "resource"]));
    requireThat(
      typeof entry.name === "string" &&
        bindingName.test(entry.name) &&
        !names.has(entry.name),
    );
    names.add(entry.name);
    requireThat(closed(entry.resource, ["resourceUid"]));
    const uid = entry.resource.resourceUid;
    requireThat(typeof uid === "string" && uid !== "");
    const target = resolve(uid);
    requireThat(
      target?.uid === uid &&
        targets.includes(target.form) &&
        target.host === source.host &&
        target.space === source.space &&
        target.authorized === true &&
        target.deleting !== true,
      "dependency_conflict",
    );
    return { name: entry.name, resourceUid: uid, form: target.form };
  });
}

/** Whether WorkerDeployment 0.7.0 may select a Version of this exact Form. */
export function deploymentAcceptsVersion(form) {
  return DEPLOYMENT_0_7_0_VERSIONS.includes(form);
}

/**
 * Whether a new WorkerDeployment 0.7.0 may become active for `worker` while
 * `active` Deployments exist. WorkerDeployment 0.4.0 to 0.7.0 share one fence.
 */
export function deploymentFenceAllows({ worker, active }) {
  return !active.some(
    (deployment) =>
      deployment.worker === worker &&
      WORKER_DEPLOYMENT_URLS.includes(deployment.form),
  );
}
