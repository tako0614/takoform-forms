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

## Useful checks

```console
bun run check:generation
bun run check:publication
bun run check:trust
bun run check
```
