package edgeformcatalog

import (
	"fmt"
	"strings"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

const (
	containerImageDigestPattern = `^(?:[a-z0-9]+(?:[.-][a-z0-9]+)*(?::(?:[1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]))?/)?[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?@sha256:[0-9a-f]{64}$`
	containerHealthPathPattern  = `^/(?:[A-Za-z0-9._~!$&'()*+,;=:@-]|%[0-9A-Fa-f]{2})+(?:/(?:[A-Za-z0-9._~!$&'()*+,;=:@-]|%[0-9A-Fa-f]{2})+)*$|^/$`
	containerRevisionPattern    = `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`

	containerEnvironmentMaxEntries     = 64
	containerEnvironmentMaxValueLength = 4096
	containerSensitiveNameMaxItems     = 64
	containerHealthPathMaxLength       = 2048
	containerRevisionMaxLength         = 128
	containerEndpointHostnameMaxLength = 253
	containerEndpointURLMaxLength      = 262
	containerEndpointURLPattern        = `^https://[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+/$`
)

const (
	ContainerServiceKind        = "ContainerService"
	ContainerServiceVersion     = "0.1.0"
	ContainerServiceSchemaHash  = "sha256:114d452395562573f46d9a879efa889ab42a3e43348d7db244e22df7d6e330e2"
	ContainerServicePackageHash = "sha256:0fb3c53940180e3f661268e079f9dbc6667c4d1fbbc74b4561ebb5ffa2740d33"
	ContainerEndpointKind       = "ContainerEndpoint"
	ContainerEndpointVersion    = "0.1.0"
)

// SourceOnlyForms are normal model declarations rendered outside the current
// 17-Form publication projection. Their generated package bytes remain local
// source artifacts and are not inserted into the family index or release set.
func SourceOnlyForms() []model.Form {
	return []model.Form{containerServiceForm(), containerEndpointForm()}
}

// RenderSourceOnlyForms uses the same Form model and renderer as the current
// catalog. The endpoint resolver pins the frozen service identity rather than
// silently accepting a locally changed target contract.
func RenderSourceOnlyForms() ([]RenderedForm, error) {
	forms := SourceOnlyForms()
	out := make([]RenderedForm, 0, len(forms))
	service, err := renderSourceOnlyForm(forms[0], newTargetContractResolver())
	if err != nil {
		return nil, fmt.Errorf("render source-only ContainerService: %w", err)
	}
	serviceSchemaDigest, err := formpackage.DigestCanonicalJSON([]byte(service.DefinitionJSON))
	if err != nil {
		return nil, fmt.Errorf("digest source-only ContainerService: %w", err)
	}
	if serviceSchemaDigest != ContainerServiceSchemaHash {
		return nil, fmt.Errorf("source-only ContainerService schema digest %s does not match frozen target %s", serviceSchemaDigest, ContainerServiceSchemaHash)
	}
	out = append(out, service)
	endpoint, err := renderSourceOnlyForm(forms[1], &containerEndpointTargetResolver{base: newTargetContractResolver()})
	if err != nil {
		return nil, fmt.Errorf("render source-only ContainerEndpoint: %w", err)
	}
	out = append(out, endpoint)
	return out, nil
}

func renderSourceOnlyForm(form model.Form, resolver model.TargetContractResolver) (RenderedForm, error) {
	if err := form.Validate(); err != nil {
		return RenderedForm{}, fmt.Errorf("%s authoring: %w", form.Kind, err)
	}
	return renderForm(form, resolver)
}

func containerServiceForm() model.Form {
	return model.Form{
		Family: model.Family{Group: Family.Group},
		Kind:   ContainerServiceKind, Slug: "container-service", Role: model.RoleIdentity,
		RequiresHostAPI: stableHostLane, DefinitionVersion: ContainerServiceVersion,
		ExcludeImport: true,
		Title:         "Container Service",
		Description: "One logical private HTTP Container service identified by an OCI image pinned to a sha256 " +
			"manifest digest. The Host provisions and operates exact execution revisions using Host Offering-owned " +
			"capacity and placement; those are never customer desired fields. This Form-only local candidate does " +
			"not publish caller-facing HTTP invocation; container.http and its Worker Binding remain a separate " +
			"unresolved contract gap. The service exposes the declared HTTP port and health path. An update starts " +
			"the exact requested workload revision, verifies health before moving serving traffic, and preserves the " +
			"previous serving execution revision of the same Resource incarnation if startup or health fails. A lost operation " +
			"acknowledgement is resolved by readback of the same resource UID, desired generation, and operation " +
			"identity before retry; a mismatched identity is never adopted. Delete is fenced by the exact observed " +
			"incarnation. Durable restart recovery reconciles persisted desired/observed generations and operation " +
			"identity; it is not process-control through the request API. `requiredSensitiveVars` contains names only, " +
			"never secret values. This initial local executable slice refuses any nonempty requiredSensitiveVars " +
			"because the current Host API cannot supply secret material. `environment` is classified as ordinary " +
			"nonsecret desired-state configuration and is persisted/read back as part of ResourceSpec. Callers MUST NOT " +
			"put secrets in this field; the schema does not automatically detect or reject secret values. A separate " +
			"sealed, generation-bound secret transport is not available. `outboundInternet` is explicitly false when " +
			"omitted and becomes true only when requested.",
		Fields: []model.Field{
			{HCL: "image", Wire: "image", Kind: model.KindString, Required: true,
				Pattern: containerImageDigestPattern, MaxLength: 512,
				Doc:     "OCI image reference including its immutable sha256 manifest digest; mutable tags without a digest are refused.",
				Example: containerImageExample(), AltExample: "ghcr.io/example/app:v2@sha256:" + strings.Repeat("b", 64), CounterExample: "ghcr.io/example/app:latest"},
			{HCL: "http_port", Wire: "httpPort", Kind: model.KindInteger, Required: true,
				Min: model.I64(1), Max: model.I64(65535),
				Doc:     "The private HTTP listener port inside the image; public port mapping and placement are Host-owned.",
				Example: 8080, AltExample: 9000, CounterExample: 0},
			{HCL: "health_path", Wire: "healthPath", Kind: model.KindString, Required: true,
				Pattern: containerHealthPathPattern, MaxLength: containerHealthPathMaxLength,
				Doc:     "Absolute path used by the Host to establish readiness before an update is made serving.",
				Example: "/health", AltExample: "/ready", CounterExample: "health"},
			{HCL: "environment", Wire: "environment", Kind: model.KindStringMap,
				Default: map[string]any{}, MaxProperties: containerEnvironmentMaxEntries,
				ItemPattern: `^[\s\S]*$`, MaxLength: containerEnvironmentMaxValueLength,
				Doc:     "At most 64 ordinary bounded desired-state strings persisted/read back as ResourceSpec. Callers MUST NOT put secrets here; automatic secret-value detection/rejection is not provided.",
				Example: map[string]any{"APP.MODE": "production", "APP_MODE": "standard"}, CounterExample: containerEnvironmentOverflow()},
			{HCL: "workload_revision", Wire: "workloadRevision", Kind: model.KindString, Required: true,
				Pattern: containerRevisionPattern, MaxLength: containerRevisionMaxLength,
				Doc:     "Author-declared bounded revision identity for this desired workload; it is part of the exact operation fence, not a Host UID or generation.",
				Example: "revision-1", AltExample: "revision-2", CounterExample: strings.Repeat("r", containerRevisionMaxLength+1)},
			{HCL: "outbound_internet", Wire: "outboundInternet", Kind: model.KindBoolean, Default: false,
				Doc: "Whether this workload may initiate outbound Internet connections. Omission means false.", Example: false, AltExample: true},
			{HCL: "required_sensitive_vars", Wire: "requiredSensitiveVars", Kind: model.KindStringSet,
				Default: []any{}, ItemPattern: model.PortableMapKeyPattern, MaxItems: containerSensitiveNameMaxItems,
				Doc:     "Names of required sensitive process variables only; values are supplied outside portable desired state. This initial executable local slice refuses a nonempty set.",
				Example: []any{}, CounterExample: []any{"not/a/portable-name"}},
		},
		DeclaredNegativeCases: []model.NegativeCase{
			{Name: "sensitive-requirement-values-not-names", Desired: containerServiceDesired(map[string]any{"APP_MODE": "production"}, int64(8080), "/health", "revision-1", false, []any{"API_TOKEN=plaintext"})},
			{Name: "http-port-out-of-range", Desired: containerServiceDesired(map[string]any{"APP_MODE": "production"}, int64(65536), "/health", "revision-1", false, []any{})},
			{Name: "environment-value-too-long", Desired: containerServiceDesired(map[string]any{"APP_MODE": strings.Repeat("x", containerEnvironmentMaxValueLength+1)}, int64(8080), "/health", "revision-1", false, []any{})},
			{Name: "registry-port-out-of-range", Desired: containerServiceDesired(map[string]any{"APP_MODE": "production"}, int64(8080), "/health", "revision-1", false, []any{}, "registry.example.com:99999/app@sha256:"+strings.Repeat("a", 64))},
			{Name: "health-path-empty-segment", Desired: containerServiceDesired(map[string]any{"APP_MODE": "production"}, int64(8080), "//health", "revision-1", false, []any{})},
		},
	}
}

func containerServiceDesired(environment map[string]any, port int64, health, revision string, outbound bool, sensitive []any, images ...string) map[string]any {
	image := containerImageExample()
	if len(images) > 0 {
		image = images[0]
	}
	return map[string]any{
		"image": image, "httpPort": port, "healthPath": health, "environment": environment,
		"workloadRevision": revision, "outboundInternet": outbound, "requiredSensitiveVars": sensitive,
	}
}

func containerImageExample() string {
	return "ghcr.io/example/app@sha256:" + strings.Repeat("a", 64)
}

func containerEnvironmentOverflow() map[string]any {
	values := make(map[string]any, containerEnvironmentMaxEntries+1)
	for index := 0; index <= containerEnvironmentMaxEntries; index++ {
		values[fmt.Sprintf("VAR_%02d", index)] = "bounded"
	}
	return values
}

func containerEndpointForm() model.Form {
	return model.Form{
		Family: model.Family{Group: Family.Group}, Kind: ContainerEndpointKind, Slug: "container-endpoint",
		Role: model.RoleAttachment, RequiresHostAPI: stableHostLane, DefinitionVersion: ContainerEndpointVersion,
		ExcludeImport: true, Title: "Container Endpoint",
		Description: "Attaches one stable HTTPS root URL to one exact ContainerService Resource incarnation. " +
			"The `service` reference pins edge.forms.takoform.com/ContainerService@0.1.0 and is immutable for " +
			"this endpoint; at most one live endpoint may attach to a service. The host-assigned canonical hostname " +
			"and root URL both remain immutable for the endpoint's UID. Requests route to the referenced service's " +
			"health-selected serving generation, so workload-generation cutover does not change the URL. If a " +
			"service is deleted and recreated under the same name with a new UID, the endpoint MUST NOT retarget to " +
			"that replacement. Delete the endpoint to detach it; deleting a service while this attachment is live " +
			"is refused as dependency_in_use, and detaching never deletes the service. HTTPS uses its standard root " +
			"port 443; no plaintext address is provided. This endpoint is not an authentication, authorization, or " +
			"secret-delivery contract and declares no Worker Interface or Binding.",
		Fields: []model.Field{{
			HCL: "service", Wire: "service", Kind: model.KindResourceRef, Required: true, Immutable: true,
			ResourceTarget: &model.ResourceTarget{Group: Family.APIVersion(), Kind: ContainerServiceKind, Contract: model.TargetContract{ExactForm: true}},
			Exclusive:      &model.ExclusiveHold{},
			Doc: "Exact ContainerService resource this endpoint attaches to. The group, kind, definition version, " +
				"and schema digest are pinned by this Form. It is immutable: use a new endpoint UID to change the " +
				"service. The relation remains fenced to the resolved service UID; a same-name replacement does not " +
				"become its target. At most one live ContainerEndpoint may hold this service.",
			Example:        map[string]any{"apiVersion": Family.APIVersion(), "kind": ContainerServiceKind, "name": "container-service"},
			CounterExample: map[string]any{"apiVersion": "other.forms.takoform.com", "kind": "DifferentService", "name": "container-service"},
		}},
		Outputs: []model.Field{
			{HCL: "hostname", Wire: "hostname", Kind: model.KindString, HostAssigned: true,
				Pattern: model.PatternCanonicalHostname, MaxLength: containerEndpointHostnameMaxLength,
				Doc: "Canonical dotted DNS hostname assigned by the host. It is immutable for the lifetime of this endpoint UID, including service generation cutover, and is not reconstructed from the resource name."},
			{HCL: "url", Wire: "url", Kind: model.KindString, HostAssigned: true,
				Pattern: containerEndpointURLPattern, MaxLength: containerEndpointURLMaxLength,
				Doc: "Absolute HTTPS root URL exactly `https://` plus the assigned canonical hostname plus `/`. The scheme is HTTPS with the standard port 443 and no explicit port; hostname and URL are immutable for this endpoint UID and the URL routes to the referenced service's health-selected generation."},
		},
	}
}

type containerEndpointTargetResolver struct{ base model.TargetContractResolver }

func (resolver *containerEndpointTargetResolver) ResolveResourceTarget(target model.ResourceTarget) (model.ResolvedResourceTarget, error) {
	if target.Group == Family.APIVersion() && target.Kind == ContainerServiceKind && target.Contract.ExactForm {
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			TargetFormRefs: []model.TargetFormRef{{
				APIVersion: Family.APIVersion(), Kind: ContainerServiceKind,
				DefinitionVersion: ContainerServiceVersion, SchemaDigest: ContainerServiceSchemaHash,
			}},
		}, nil
	}
	return resolver.base.ResolveResourceTarget(target)
}
