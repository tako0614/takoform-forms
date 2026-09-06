// Initial hostname authority is deliberately separate from static version uploads.
import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { credentialFreeEnvironment } from "./form-publication.mjs";

export const DOMAIN_SURFACE = "edge-form-domain";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const hostname = "edge.forms.takoform.com";
const worker = "takoform-edge-form-pages";
const repository = "https://github.com/tako0614/takoform-forms.git";
const sha = /^[a-f0-9]{40}$/u;

export const DOMAIN_CONTRACT = {
  surface: DOMAIN_SURFACE,
  target: `Cloudflare custom domain ${hostname} for ${worker}`,
  covers: ["scripts/edge-form-domain.mjs", "scripts/deploy.mjs"],
  requiresScripts: ["deploy"],
  requiresTools: ["git", "bun", "wrangler"],
  requiresEnv: [],
  triggers: ["authority"],
  obligations: {
    provenance:
      "One explicit clean source commit equal to public main; pinned Wrangler authenticates in memory. Account and production environment are explicit.",
    "post-conditions":
      "Read exact account, active zone, Worker and custom-domain binding from Cloudflare. Then use edge-form-pages --verify for public HTTPS bytes and assets; binding alone is not site completion.",
    reversal:
      "Only create an unoccupied hostname. Keep all existing domains/DNS; no overwrite or delete path. To reverse a newly created binding, the operator removes only the returned new binding ID after checking its ownership.",
    "failure-handling":
      "Read-only changeset refuses conflicts. One PUT with all overwrite controls false. An uncertain PUT is followed by exact domain readback, never blindly retried.",
    "independent-review":
      "Independent reviewer checks the exact source, target and conflict-refusing changeset/PUT behavior before initial authority mutation. No routine upload calls this surface.",
  },
};

export function assertSiteSource(commit, directory = root) {
  if (!sha.test(commit ?? ""))
    throw new Error("--commit must select an exact 40-hex source commit");
  const run = (args, publicRead = false) => {
    const result = spawnSync("git", args, {
      cwd: directory,
      encoding: "utf8",
      env: publicRead ? credentialFreeEnvironment() : process.env,
    });
    if (result.status !== 0)
      throw new Error(`source check failed: git ${args[0]}`);
    return result.stdout.trim();
  };
  if (run(["status", "--porcelain=v1", "--untracked-files=all"]))
    throw new Error("site publication requires a clean worktree");
  if (run(["rev-parse", "HEAD"]) !== commit)
    throw new Error("selected source commit is not HEAD");
  if (run(["remote", "get-url", "origin"]) !== repository)
    throw new Error("site requires canonical origin");
  const remote = run(["ls-remote", repository, "refs/heads/main"], true).split(
    /\s+/u,
  )[0];
  if (remote !== commit)
    throw new Error("selected source must equal exact public main");
  return commit;
}

export function parseDomainInvocation(args) {
  const result = { execute: false };
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--execute" && !result.execute) result.execute = true;
    else if (["--environment", "--account-id", "--commit"].includes(args[i])) {
      const key = args[i].slice(2);
      if (
        result[key] !== undefined ||
        !args[i + 1] ||
        args[i + 1].startsWith("--")
      )
        throw new Error("invalid domain invocation");
      result[key] = args[++i];
    } else throw new Error("invalid domain invocation");
  }
  if (
    result.environment !== "production" ||
    !/^[a-f0-9]{32}$/u.test(result["account-id"] ?? "") ||
    !sha.test(result.commit ?? "")
  )
    throw new Error(
      "domain requires --environment production --account-id <32-hex> --commit <40-hex> [--execute]",
    );
  return result;
}

export async function bindEdgeDomain(
  options,
  { api, assertSource = assertSiteSource } = {},
) {
  assertSource(options.commit);
  const account = options["account-id"];
  const zones = await api(
    "GET",
    `/zones?name=takoform.com&account.id=${account}`,
  );
  if (
    zones.length !== 1 ||
    zones[0].name !== "takoform.com" ||
    zones[0].status !== "active" ||
    zones[0].account?.id !== account
  )
    throw new Error("exact active takoform.com zone/account not verified");
  const zone = zones[0].id;
  const domainsURL = `/accounts/${account}/workers/domains/records?hostname=${hostname}`;
  const existing = await api("GET", domainsURL);
  const exact = existing.filter((entry) => entry.hostname === hostname);
  if (
    exact.length &&
    !(
      exact.length === 1 &&
      exact[0].service === worker &&
      exact[0].environment === "production" &&
      exact[0].zone_id === zone
    )
  )
    throw new Error(
      "hostname is already owned by a different target; no takeover permitted",
    );
  const workerURL = `/accounts/${account}/workers/scripts/${worker}`;
  await api("GET", `${workerURL}/settings`);
  const history = await api("GET", `${workerURL}/deployments`);
  const versions = history.deployments?.[0]?.versions;
  if (versions?.length !== 1 || versions[0].percentage !== 100)
    throw new Error("domain target must have one 100% deployed version");
  const version = await api(
    "GET",
    `${workerURL}/versions/${versions[0].version_id}`,
  );
  if (version.metadata?.annotations?.["workers/tag"] !== options.commit)
    throw new Error(
      "uploaded Worker does not identify the selected source commit",
    );
  if (exact.length)
    return {
      status: "ALREADY_BOUND",
      hostname,
      worker,
      domainId: exact[0].id,
      changed: false,
    };
  const origins = [{ hostname, zone_id: zone }];
  const changes = await api(
    "POST",
    `${workerURL}/domains/changeset?replace_state=false`,
    origins,
  );
  if (
    !["added", "updated", "removed", "conflicting"].every((key) =>
      Array.isArray(changes[key]),
    ) ||
    changes.updated.length ||
    changes.removed.length ||
    changes.conflicting.length ||
    changes.added.length !== 1 ||
    changes.added[0].hostname !== hostname
  )
    throw new Error(
      "domain changeset is not one conflict-free hostname addition",
    );
  if (!options.execute)
    return {
      status: "READY",
      hostname,
      worker,
      zoneId: zone,
      changed: false,
      changes,
    };
  assertSource(options.commit);
  let mutationError;
  try {
    await api("PUT", `${workerURL}/domains/records`, {
      origins,
      override_scope: false,
      override_existing_origin: false,
      override_existing_dns_record: false,
    });
  } catch (error) {
    mutationError = error;
  }
  let records;
  try {
    records = await api("GET", domainsURL);
  } catch {
    throw new Error(
      "domain mutation is indeterminate; inspect provider domain history before retrying",
    );
  }
  const observed = records.filter(
    (entry) =>
      entry.hostname === hostname &&
      entry.service === worker &&
      entry.environment === "production" &&
      entry.zone_id === zone,
  );
  if (observed.length !== 1)
    throw new Error(
      `domain mutation is indeterminate; inspect provider binding before retrying${mutationError ? " (PUT failed)" : ""}`,
    );
  return {
    status: "DOMAIN_BOUND",
    hostname,
    worker,
    domainId: observed[0].id,
    changed: true,
    settledAfterError: !!mutationError,
  };
}

export async function runDomainCLI(args) {
  try {
    const options = parseDomainInvocation(args);
    const auth = spawnSync(
      path.join(root, "node_modules/.bin/wrangler"),
      ["auth", "token", "--json"],
      { cwd: root, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] },
    );
    if (auth.status !== 0)
      throw new Error("Wrangler authentication unavailable");
    const { token } = JSON.parse(auth.stdout);
    if (typeof token !== "string" || !token)
      throw new Error("Wrangler did not supply a bearer token");
    const api = async (method, route, body) => {
      const response = await fetch(
        `https://api.cloudflare.com/client/v4${route}`,
        {
          method,
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          ...(body === undefined ? {} : { body: JSON.stringify(body) }),
          signal: AbortSignal.timeout(30000),
        },
      );
      const payload = await response.json();
      if (!response.ok || !payload.success)
        throw new Error(
          `Cloudflare ${method} ${route} failed (${response.status}; codes ${(payload.errors ?? []).map((e) => e.code).join(",")})`,
        );
      return payload.result;
    };
    console.log(
      JSON.stringify(await bindEdgeDomain(options, { api }), null, 2),
    );
    return 0;
  } catch (error) {
    console.error(`edge-form-domain: ${error.message}`);
    return 1;
  }
}
