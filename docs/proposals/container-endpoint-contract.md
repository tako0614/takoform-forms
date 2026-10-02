# Local ContainerEndpoint Form candidate (unpublished)

This proposal describes the optional `edge.forms.takoform.com/ContainerEndpoint`
`0.1.0` attachment candidate emitted by
`internal/edgeformcatalog/edge_container_endpoint_candidate.go`. Its only
portable artifact is one Core-v1.1.0-verifiable Form Package. The command
`go run ./cmd/edge-container-endpoint-candidate` prints the exact package
closure with an `UNPUBLISHED` status marker. This candidate is not part of the
registered catalog, current signed set, tag history, or published 17 Forms.

## Exact service attachment

The required `service` reference resolves only to the exact frozen
`edge.forms.takoform.com/ContainerService@0.1.0` FormRef:

| Identity member | Value |
| --- | --- |
| `apiVersion` | `edge.forms.takoform.com` |
| `kind` | `ContainerService` |
| `definitionVersion` | `0.1.0` |
| `schemaDigest` | `sha256:114d452395562573f46d9a879efa889ab42a3e43348d7db244e22df7d6e330e2` |
| `packageDigest` | `sha256:0fb3c53940180e3f661268e079f9dbc6667c4d1fbbc74b4561ebb5ffa2740d33` |

`service` is immutable. Core's exact-target relation annotation pins the
service Definition identity, while the Host must retain the resolved target
Resource UID for the endpoint lifetime: a same-name delete/recreate with a new
UID is not a retarget. One live ContainerEndpoint may hold a service; the
exclusive relation makes a second endpoint for it invalid. A live attachment
blocks service deletion with `dependency_in_use`. Deleting the endpoint removes
only the attachment and does not delete the service.

## Address contract

The Host assigns a canonical lowercase hostname and its absolute HTTPS root
URL, exactly `https://<hostname>/`. HTTPS uses its standard port 443; the URL
has no explicit port, path beyond `/`, query, or fragment. Both values are
required outputs and immutable for the endpoint UID. Requests route to the
referenced service's health-selected serving generation, so workload cutover
does not change the address or retarget the endpoint.

This is an address-and-routing attachment only. It is not an authentication,
authorization, secret-delivery, private WorkerBinding, or caller-facing
`container.http` Interface contract. It emits no provided Interface or
accepted Binding. The Host's public admission and implementation support are
not claimed by this package.

## Local verification scope

Focused tests validate the endpoint Definition/index and complete package
closure with released Core v1.1.0, then compile a two-package Core Snapshot
containing the exact frozen ContainerService package and this Endpoint package.
Red cases reject a wrong desired target, a wrong target schema digest, missing
host-assigned outputs, and lifecycle scenarios that substitute a same-name
service UID. Snapshot failure returns no usable partial result. These are
local authoring and integration fixtures, not Host conformance, support, or
publication evidence.
