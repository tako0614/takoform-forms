# Documentation map

This repository is the provider-neutral source and package publisher for the
Edge Form family.

- [Read the Edge Form reference](https://edge.forms.takoform.com/): choose a
  contract, inspect fields and exact examples, and follow related Forms.
- [Site and reader guide](site.md): the resource model, examples versus runnable
  deployments, current/retained history, quality checks and publication.

- [Root README](../README.md): what the project is, its four building blocks,
  commands, and scope.
- [Form inventory](../forms/README.md): the complete Edge Form list, the two
  version axes, and package publication flow.
- [Conformance corpus](../conformance/README.md): desired-state and negative
  fixtures used by the validators.
- [Revocation advancement runbook](revocation-advancement.md): append-only
  source, signing, installation, immutable publication, settlement, and safe
  partial-install recovery.

## Source and generated files

Go catalogs under `internal/` are the authoring source. The JSON trees under
`forms/candidates/`, `interfaces/candidates/`, and `bindings/candidates/` are
generated candidate output. `forms/releases/` contains content-addressed
copies checked against those candidates before publication.

The current composition selects one version per Interface/Binding name. The
generator refuses two versions that would share its name-based output path;
an old exact contract is acquired from its immutable Git source, not replaced
by whichever version is current.

`check:integrity` preserves the extraction baseline against published source
`e7f8a39311dd011b8467e97e7f300cabb9a6b06c` and refuses changed bytes for a
published definition's same version, including the retained Form identities.
`check:generation` separately verifies exact current candidate bytes. Checks
need Git with `--no-lazy-fetch` support (2.45 or newer) and that exact source's
objects locally; missing history is an error, with no automatic fetch or fallback.
CI already checks out full history. The fixed legacy corpus and retained
inventory still require their existing byte guards until their forward writer
and append-only proof are implemented. Before later releases are edited, their
published identities must also enter the historical protection set; the initial
snapshot alone is not a complete future-publication policy.

Form Packages and publisher trust evidence are verified with the pinned public
Core v1.1.0. The root and inventory pages describe the Core-derived locator,
external OIDC signing handoff, create-only trust set, and publication
condition; all publishers use the same API v1 contracts.

## Local forward candidates

- [Workflow execution candidate](proposals/workflow-execution-contract.md):
  plain-JavaScript class, retry and replay semantics with a separate exact
  Form/Interface/Binding closure. This is not published or advertised as Host
  support; current package definitions remain unchanged.

## Unaccepted proposals

- [Portable actor execution](proposals/actor-runtime.md): the missing class
  execution and WebSocket contract, with decisions needed before a forward
  definition can be authored. This is not a published specification, package,
  version allocation, or claim of Host support.
- [Concrete actor contract proposal](proposals/actor-execution-contract.md):
  class/SQL/alarm/socket signatures, event and transport lifetime, and the
  exact forward-reference graph. Error vocabulary and candidate limits remain
  adoption work; the current published contracts remain unchanged.

## Useful checks

```console
bun run check:generation
bun run check:publication
bun run check:trust
bun run check
```
