# Container Form source-readiness

The normal Go Form model and the existing `current-form-families` writer now
emit two additional local source packages under
`forms/source-candidates/edge.forms.takoform.com/`. The current family
projection, `forms/candidates/current-family-index.json`, and its 17 Forms are
unchanged. These two packages are marked `UNPUBLISHED`, kept outside the
publisher candidate tree, and are not part of signing, release planning, or
Host-support evidence.

The generated Service package matches the frozen local candidate bytes:

- `ContainerService@0.1.0`: schema digest
  `sha256:114d452395562573f46d9a879efa889ab42a3e43348d7db244e22df7d6e330e2`,
  package digest
  `sha256:0fb3c53940180e3f661268e079f9dbc6667c4d1fbbc74b4561ebb5ffa2740d33`.
- `ContainerEndpoint@0.1.0`: schema digest
  `sha256:c32e716d9185026fce7ae105d14035d75fa64ec7322483122126b95db902b336`,
  package digest
  `sha256:c5ee452369ddafc1ba15d76adcdcc1611ff558d2e385a3f1f6379a4cce86b288`.

The endpoint pins the exact Service FormRef and declares stable HTTPS ingress;
the source projection must not be described as CRUD-only. The packages declare
no Interface or Binding. Both Forms explicitly omit `import`; existing Forms
retain the model's historical default. Service's 16 negative fixtures and
Endpoint's three negative fixtures are checked as part of normal generation.

This is authoring-source readiness only. It does not establish publication,
signatures, public package tags, Host implementation/support, account
availability, installation, activation, or a public invocation Binding. The
private HTTP Binding remains blocked by Core v1's frozen Interface
consistency vocabulary and is not inferred from these Form packages.
