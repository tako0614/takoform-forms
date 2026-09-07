# Edge Form documentation

The human-facing reference lives at **edge.forms.takoform.com**. It is owned
by this publisher, not the neutral Takoform API/Core site. It contains an index,
17 current Form pages and two retained version pages. Package publication,
Host support, admission and actual hosting are separate concerns.

English keeps the existing root routes; Japanese pages are under `/ja/`. The
language menu links to the corresponding page and preserves its heading fragment.
Each locale has the complete 20-page sidebar. Guides and navigation are translated;
contract descriptions and field descriptions are marked as original English.
Schema, example and package-reference bytes do not change with language.

## Read the reference

Start with the job, then choose the contract:

- **Deliver a Worker:** ModuleWorker is the application identity. WorkerBundle
  identifies code bytes; WorkerVersion combines code, handlers and configuration;
  WorkerDeployment selects traffic. Add an Endpoint, CustomDomain or CronTrigger
  for the corresponding incoming event.
- **Give code a capability:** create the appropriate KV, object, SQLite, queue,
  actor or workflow resource and use a typed WorkerVersion binding. A QueueConsumer
  is an inbound attachment, not a producer binding.
- **Change a database schema:** SQLiteMigrationSet identifies ordered SQL history;
  SQLiteMigrationApplication applies its unapplied suffix to a database. Deleting
  the application does not roll the schema back.

Each page has use guidance, the exact contract description, required/optional
fields and constraints, a canonical desired-state fixture, the complete nested
schema, lifecycle capabilities, Interface references and package identity.
The four-field FormRef identifies the contract; the package digest identifies the
full package. Neither is a Host deployment ID.

The public index provides a task chooser, a Queue/Workflow/Actor comparison and
HTTP/queue/migration composition paths. Individual pages explain a concrete use
case and the purpose of each related Form: required input, serving prerequisite,
consumer, optional connection or alternative. These are publisher-authored
explanations reviewed against exact released contracts, not a machine-readable
dependency graph or an inferred deployment plan.

Examples are **canonical conformance fixtures, not copy-and-deploy tutorials**.
They may refer to unavailable example artifacts or prerequisite resources; the
custom-domain example deliberately uses `.invalid`. Empty desired objects are
intentional for contracts whose semantics come from identity and Interfaces.
A runnable tutorial needs a selected Host, committed artifacts, credentials and
real prerequisite resources; the publisher does not invent these.

## Source and quality rules

- `scripts/edge-form-pages.mjs` verifies the exact package closures under
  `forms/releases/`, not candidate files, guessed schemas or copied Core code.
- `scripts/edge-form-pages-vitepress.mjs` generates Markdown from that verified
  data in a fresh temporary directory. `site/.vitepress/config.mts` builds it
  with VitePress's standard theme, local search, sidebar, outline and previous/
  next links. There is no separate handwritten HTML renderer or custom palette.
  Site identity is text-only, without a custom logo or favicon mark.
  Every page uses the same sidebar. Current and retained Form groups start
  expanded, and related-site links are available there as well.
- `site/reading-guide.json` supplies non-normative reading guidance. Each entry
  is pinned to a definition version so a new version requires explicit review.
  Its English and Japanese text share that version pin; both are required.
  Purpose, use case, constraints and relationship explanations must be present
  in both languages. Related kinds must resolve to current pages without duplicate
  or self references. New wording must not assign runtime policy to an artifact
  bundle or confuse producer bindings with inbound attachments.
- Canonical examples follow the Definition's `conformanceFixtures` declaration
  and must be listed in the package index. Required fields, defaults and limits
  come from `desiredSchema`. The full schema remains inspectable.
- Current pages are tied to the selected installed signed set. Retained pages
  come from `forms/retained-packages.json` and keep their original versioned URLs.
  They explicitly do not claim membership in the current signed set. Do not
  replace a retained page's example or schema with the current definition.
- Public-byte claims require anonymous tag/byte verification during deployment.
  That observation is not proof that a tag has never moved historically, nor
  proof of signature admission, availability, Host support or runtime behavior.
- Every page needs one clear title, a unique description, valid structure,
  working internal links and visible keyboard focus. Do not call a fixture
  runnable when its resources or artifacts do not exist.
- The site serves local VitePress JavaScript and a local search index. Its CSP
  permits same-origin scripts and exact hashes of the generated inline bootstrap
  scripts, not arbitrary inline or third-party scripts. Local styles/fonts and
  inline styles support the theme and syntax highlighting. `Cache-Control:
  no-transform` prevents proxy-injected analytics from changing verified HTML.
  This does not change zone-wide settings. All generated assets are included in
  the build digest and the existing deployment readback. The root HTTP response
  must also carry the generated CSP, `nosniff` and `no-transform`; matching HTML
  alone is not a successful deployment readback.

## Build and verify

```console
bun install --frozen-lockfile
go mod download
bun run check
bun run build:edge-form-pages
bun run check:edge-form-pages:browser
```

The build prints a fresh temporary output directory; it never deletes a
caller-selected nonempty directory. For a specific set/destination:

```console
bun run build:edge-form-pages --trust-set <source-commit> --output <empty-directory>
```

The portable gate is read-only and validates exact packages, deterministic HTML,
fixtures, history labels and the static build. The explicit browser lane needs
installed Chrome/Chromium (`TAKOFORM_BROWSER` overrides its path); it neither
downloads a browser nor uses a user profile. It checks every current and retained
page at 320/375/414/768px under the generated CSP, navigation, JSON, semantics,
keyboard disclosure, the mobile sidebar, local search and both color schemes.
Manual visual review still checks hierarchy, contrast and reading burden.

## Publish through this repository

Inspect `bun run deploy -- --contract`. Keep the authenticated account explicitly
selected with `CLOUDFLARE_ACCOUNT_ID` outside source. Select a clean exact commit
equal to public `main`; detached source checkouts are allowed when explicitly
selected by `--commit`. No package tags or signed-set files change here.

Routine static update:

```console
bun run deploy -- edge-form-pages --trust-set <set-source-commit> --environment production --commit <site-commit>
```

This runs one exact-set scoped gate, uploads once, confirms the version in
provider history and checks all public page/asset bytes. The provider predecessor
must exist for a routine update and is rechecked immediately before upload. The output contains the
source commit, artifact digest, version/deployment and predecessor rollback
identity. `--dry-run` does not upload. `--verify` only performs readback and does
not require a commit argument.

### First publication only

1. Use `edge-form-pages-bootstrap` with the same arguments. It requires the
   Worker to be absent and uploads a static-only Worker with no routes or DNS.
   `UPLOADED_AWAITING_DOMAIN` is intentionally not a public-site success claim.
2. Independently review the domain authority change. Run the following without
   `--execute` to inspect its changeset, then with `--execute` once:

   ```console
   bun run deploy -- edge-form-domain --environment production --account-id <account-id> --commit <site-commit> --execute
   ```

   It verifies the active zone/account, the exact tagged Worker version and an
   unoccupied hostname. All DNS/origin/scope overwrite controls stay false.
   Existing ownership or a conflicting DNS record stops the command; no takeover,
   DNS deletion, package release or automatic retry is available.
3. After DNS/TLS propagation, use `edge-form-pages --trust-set <set-source-commit>
   --environment production --verify`. Binding success alone is insufficient:
   every page, CSS/font file and the negative route must read back correctly.

After an upload error the command reads provider history once, reports it and
leaves the result indeterminate for manual reconciliation; it does not upload a
second time. An uncertain domain PUT is settled by exact ownership readback when
possible. Inspect provider history or run read-only verification before deciding
what to do next. Routine rollback uses the captured previous Worker
version. Initial binding reversal is an operator action against only the returned
new domain ID, after rechecking ownership; it must not touch unrelated DNS.
Keep credentials and deployment evidence outside this repository.
