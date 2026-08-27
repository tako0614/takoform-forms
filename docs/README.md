# Documentation map

This repository is the provider-neutral source and package publisher for the
Edge Form family.

- [Root README](../README.md): what the project is, its four building blocks,
  commands, and scope.
- [Form inventory](../forms/README.md): the complete Edge Form list, the two
  version axes, and package publication flow.
- [Conformance corpus](../conformance/README.md): desired-state and negative
  fixtures used by the validators.

## Source and generated files

Go catalogs under `internal/` are the authoring source. The JSON trees under
`forms/candidates/`, `interfaces/candidates/`, and `bindings/candidates/` are
generated candidate output. `forms/releases/` contains content-addressed
copies checked against those candidates before publication.

Form Packages are verified with the pinned public Core v1.0.1. The root and
inventory pages describe the Core-derived locator and publication condition;
official and external publishers use the same package format.

## Useful checks

```console
bun run check:generation
bun run check:publication
bun run check
```
