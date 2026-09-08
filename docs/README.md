# Documentation map

This repository is the provider-neutral source and package publisher for the
Edge Form family.

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

Form Packages and publisher trust evidence are verified with the pinned public
Core v1.1.0. The root and inventory pages describe the Core-derived locator,
external OIDC signing handoff, create-only trust set, and publication
condition; official and external publishers use the same API v1 contracts.

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
