# Edge Form documentation

## Human-authored Form specifications

The new Host API v2 Form specifications are authored in Japanese under
`spec/forms/<Kind>/<version>/index.md`. Their exact identifiers are
`https://edge.forms.takoform.com/forms/<Kind>/<version>/`, including the trailing
slash. At publication, each exact URL must serve its normative content directly;
a redirect or a locale alias does not transfer its identity.
The corresponding `/ja/` page is a reading view, not another Form identity.
`spec/guides/` provides the overview, Host usage, Provider guidance and Migration.
Guides are explanatory and their `/v2/` path denotes the Host API they discuss,
not a collective Form release version.

These sources are loaded independently of v1 package verification and rendered
into the same site. They must not be inserted into a v1 signed set, promoted by
the package generator, or treated as current Host/Provider capabilities. The
existing v1 definitions, package bytes, history and versioned routes remain
unchanged. An English reading view that contains the Japanese original must say
so; only navigation and a language notice are not a translated specification.

Before publishing a Form URL, review its complete normative body, exact links,
examples and all referenced Form versions. Check direct-200 URL behavior and
anonymous body readback on the actual site separately from local builds. Record
the exact source commit and retain the published bytes; any later semantic
change needs a new Form URL. The local documentation work does not perform
that publication or establish Host runtime conformance.

`spec/forms.freeze.json` is a publisher-local, append-only first-add anchor for
the exact Japanese Markdown bytes at each version-fixed URL. Its check requires
complete Git history. A frozen entry is only eligible for publication; it does
not claim the URL is already public. The published site also serves those exact
bytes at the derived `source.md` path beside each Form page. This companion
asset and the site's `_edge-forms-publication.json` inventory are readback
evidence for this publisher, not Takoform API endpoints, Host discovery, or
required distribution formats. The canonical Form URL itself remains the
direct-200 HTML reading view; its presentation can change without changing the
normative Markdown source.

With authored sources present, `/` and `/ja/` introduce the new Forms. The v1
overview remains at `/v1/` and `/ja/v1/`; all existing versioned Form routes keep
their original definitions. `/v2/` is also a direct entry into the new guide.

## Retained v1 package pages

The human-facing reference lives at **edge.forms.takoform.com**. It is owned
by this publisher, not the neutral Takoform API/Core site. The v1 section contains
an index and the current and retained version pages derived from its selected
package set. The selected source may contain newer, unpublished Form versions. Package publication,
Host support, admission and actual hosting are separate concerns.

English keeps the existing root routes; Japanese pages are under `/ja/`. The
language menu links to the corresponding page and preserves its heading fragment.
Each locale has the complete published sidebar. Guides and navigation are translated;
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
- Public current pages are tied to the selected installed signed set. Retained pages
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

The portable `bun run check` uses `check:edge-form-pages:source`: it Core-verifies
the exact selected package closure, checks the version-pinned reading guide,
renders real VitePress pages, and runs a Wrangler dry-run in a disposable
directory. This source check is explicitly **UNPUBLISHED**: it consumes no
trust set, emits no signed/public-readback assertion or unborn package-tag
links, and deletes all preview assets. It is neither a site build target nor a
deployment authorization. The signed site build uses the explicitly selected
already-published v1 set, not the newer unsigned v1 candidate roster. Its
historical reading guide is read byte-exact from pinned Git history. The normal
build includes only previously published v2 Form URLs; frozen but not yet
published Forms cannot appear through the routine site-update surface. The
explicit publication build below selects all frozen entries and preserves all
19 previously published v1 versioned routes. It requires no new v1 package
signature for v2 prose.

```console
bun install --frozen-lockfile
go mod download
bun run check
bun run check:edge-form-pages:browser:source
```

The source browser lane builds one temporary preview, checks the rendered routes,
and removes it on completion. It is explicitly **UNPUBLISHED** and does not
qualify production CSP headers or public availability.

By default, the signed build prints a fresh temporary v1-only output
directory; it never deletes a caller-selected nonempty directory. For a specific
set/destination:

```console
bun run build:edge-form-pages --trust-set <source-commit> --output <empty-directory>

# Offline candidate only: include all frozen authored v2 Form versions.
bun run build:edge-form-pages --trust-set <published-v1-set> --output <empty-directory> --publish-frozen
```

The portable gate is read-only and validates exact packages, deterministic HTML,
fixtures, history labels and the static build. The explicit browser lane needs
installed Chrome/Chromium (`TAKOFORM_BROWSER` overrides its path); it neither
downloads a browser nor uses a user profile. It checks every current and retained
page at 320/375/414/768px, navigation, JSON, semantics,
keyboard disclosure, the mobile sidebar, local search and both color schemes.
The default signed browser lane also checks the generated production CSP; the
explicit source-preview lane checks local rendering without claiming that proof.
Manual visual review still checks hierarchy, contrast and reading burden.

## Publish through this repository

Source preview is not a substitute publication artifact. A local frozen-Form
build is only a candidate: it does not prove public URL readback or authorize an
upload. Publishing the first v2 Form URLs is a separate, consumer-pinned identity
surface in this repository:

```console
bun run deploy -- edge-v2-forms --trust-set <published-v1-set> --environment production --commit <exact-public-main-commit> --dry-run
bun run deploy -- edge-v2-forms --trust-set <published-v1-set> --environment production --commit <exact-public-main-commit>
bun run deploy -- edge-v2-forms --trust-set <published-v1-set> --environment production --verify
```

Before its one full-asset Worker upload, this path proves the selected frozen
source and checks that every previously published Form URL and raw source still
exist unchanged. It requires each new canonical URL and companion source path
to be absent, retains the old published inventory, and rechecks provider history
and public source immediately before upload. After upload, the same public
readback compares the complete generated asset closure, including old/new Form
pages and raw normative source. If an upload or readback becomes uncertain,
inspect provider history and public URLs; never blindly retry or remove a
published URL. Cloudflare does not provide an atomic compare-and-swap for this
whole static-asset replacement: an independent concurrent writer can still race
the final preflight, so publication must be operationally serialized. The
publisher does not introduce an admission service or a cross-Host registry.

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
