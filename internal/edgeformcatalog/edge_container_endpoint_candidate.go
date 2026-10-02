package edgeformcatalog

import (
	"fmt"
	"strings"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

const (
	EdgeContainerEndpointCandidateFormKind    = "ContainerEndpoint"
	EdgeContainerEndpointCandidateFormVersion = "0.1.0"
	EdgeContainerEndpointCandidateFormSlug    = "container-endpoint"

	EdgeContainerServiceCandidateFormKind      = "ContainerService"
	EdgeContainerServiceCandidateFormVersion   = "0.1.0"
	EdgeContainerServiceCandidateSchemaDigest  = "sha256:114d452395562573f46d9a879efa889ab42a3e43348d7db244e22df7d6e330e2"
	EdgeContainerServiceCandidatePackageDigest = "sha256:0fb3c53940180e3f661268e079f9dbc6667c4d1fbbc74b4561ebb5ffa2740d33"
)

const (
	containerEndpointHostnameMaxLength = 253
	containerEndpointURLMaxLength      = 262 // https:// + 253-character hostname + /
	containerEndpointURLPattern        = `^https://[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+/$`
)

// ContainerEndpointLifecycleFixture is a local-only semantic regression case.
// It is not a Host support record or a portable Form Package conformance claim.
type ContainerEndpointLifecycleFixture struct {
	Name     string `json:"name"`
	Negative bool   `json:"negative"`
	Scenario string `json:"scenario"`
	Expected string `json:"expected"`
}

// EdgeContainerEndpointFormCandidate is an unpublished Form-only package. Its
// single service reference consumes the exact ContainerService identity and
// package emitted by the frozen local service candidate.
type EdgeContainerEndpointFormCandidate struct {
	PublicationStatus      string                              `json:"publicationStatus"`
	ContractClosure        string                              `json:"contractClosure"`
	Form                   RenderedForm                        `json:"form"`
	Package                RenderedFormPackage                 `json:"package"`
	LocalLifecycleFixtures []ContainerEndpointLifecycleFixture `json:"localLifecycleFixtures"`
}

// RenderEdgeContainerEndpointFormCandidate renders the optional unpublished
// endpoint attachment and its exact package closure without registering it in
// any current catalog or release surface.
func RenderEdgeContainerEndpointFormCandidate() (EdgeContainerEndpointFormCandidate, error) {
	formModel := containerEndpointCandidateForm()
	if err := formModel.Validate(); err != nil {
		return EdgeContainerEndpointFormCandidate{}, fmt.Errorf("ContainerEndpoint candidate authoring: %w", err)
	}

	resolver := &containerEndpointTargetResolver{base: newTargetContractResolver()}
	form, err := renderForm(formModel, resolver)
	if err != nil {
		return EdgeContainerEndpointFormCandidate{}, fmt.Errorf("ContainerEndpoint candidate Form: %w", err)
	}
	// The shared attachment model carries historical import capability. This
	// candidate specifies no adoption identity or import fencing, so it claims
	// only create/read/delete/observe. The immutable service field derives no
	// update capability.
	form.Definition.LifecycleCapabilities = []string{"create", "read", "delete", "observe"}
	form.DefinitionJSON, err = marshalIndented(form.Definition)
	if err != nil {
		return EdgeContainerEndpointFormCandidate{}, fmt.Errorf("ContainerEndpoint candidate Definition JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(form.DefinitionJSON)); err != nil {
		return EdgeContainerEndpointFormCandidate{}, fmt.Errorf("ContainerEndpoint candidate Core validation: %w", err)
	}
	packageClosure, err := renderContainerFormPackage(form)
	if err != nil {
		return EdgeContainerEndpointFormCandidate{}, fmt.Errorf("ContainerEndpoint candidate package closure: %w", err)
	}
	return EdgeContainerEndpointFormCandidate{
		PublicationStatus: "UNPUBLISHED",
		ContractClosure:   "form-only; no Interface or Binding is declared",
		Form:              form,
		Package:           packageClosure,
		LocalLifecycleFixtures: []ContainerEndpointLifecycleFixture{
			{
				Name: "healthy-service-generation-routes", Scenario: "The referenced ContainerService has a health-selected serving generation.",
				Expected: "The endpoint's stable HTTPS root URL routes to that service's health-selected serving generation.",
			},
			{
				Name: "same-name-new-service-uid-does-not-retarget", Negative: true,
				Scenario: "The referenced service is deleted and a new ContainerService incarnation is created with the same group, kind, and name.",
				Expected: "The existing endpoint remains fenced to the exact original service UID and is not silently retargeted to the replacement.",
			},
			{
				Name: "service-delete-with-live-endpoint-is-refused", Negative: true,
				Scenario: "A service deletion is requested while its exclusive ContainerEndpoint attachment remains live.",
				Expected: "The service delete is refused as dependency_in_use; deleting the endpoint removes only the attachment.",
			},
			{
				Name: "missing-host-assigned-output-is-refused", Negative: true,
				Scenario: "A response omits hostname or url from the required host-assigned outputs.",
				Expected: "The missing output is rejected; both hostname and URL are required and remain immutable for this endpoint UID.",
			},
			{
				Name: "wrong-target-or-schema-digest-is-unresolved", Negative: true,
				Scenario: "The exact ContainerService FormRef is absent from the Core Snapshot or its schema digest differs.",
				Expected: "Core produces no partial Snapshot and reports the exact target reference unresolved.",
			},
		},
	}, nil
}

func containerEndpointCandidateForm() model.Form {
	return model.Form{
		Family:            model.Family{Group: Family.Group},
		Kind:              EdgeContainerEndpointCandidateFormKind,
		Slug:              EdgeContainerEndpointCandidateFormSlug,
		Role:              model.RoleAttachment,
		RequiresHostAPI:   stableHostLane,
		DefinitionVersion: EdgeContainerEndpointCandidateFormVersion,
		Title:             "Container Endpoint",
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
			ResourceTarget: &model.ResourceTarget{
				Group: Family.APIVersion(), Kind: EdgeContainerServiceCandidateFormKind,
				Contract: model.TargetContract{ExactForm: true},
			},
			Exclusive: &model.ExclusiveHold{},
			Doc: "Exact ContainerService resource this endpoint attaches to. The group, kind, definition version, " +
				"and schema digest are pinned by this Form. It is immutable: use a new endpoint UID to change the " +
				"service. The relation remains fenced to the resolved service UID; a same-name replacement does not " +
				"become its target. At most one live ContainerEndpoint may hold this service.",
			Example: map[string]any{
				"apiVersion": Family.APIVersion(), "kind": EdgeContainerServiceCandidateFormKind, "name": "container-service",
			},
			CounterExample: map[string]any{
				"apiVersion": "other.forms.takoform.com", "kind": "DifferentService", "name": "container-service",
			},
		}},
		Outputs: []model.Field{
			{HCL: "hostname", Wire: "hostname", Kind: model.KindString,
				HostAssigned: true, Pattern: model.PatternCanonicalHostname,
				MaxLength: containerEndpointHostnameMaxLength,
				Doc: "Canonical dotted DNS hostname assigned by the host. It is immutable for the lifetime of this " +
					"endpoint UID, including service generation cutover, and is not reconstructed from the resource name."},
			{HCL: "url", Wire: "url", Kind: model.KindString,
				HostAssigned: true, Pattern: containerEndpointURLPattern,
				MaxLength: containerEndpointURLMaxLength,
				Doc: "Absolute HTTPS root URL exactly `https://` plus the assigned canonical hostname plus `/`. " +
					"The scheme is HTTPS with the standard port 443 and no explicit port; hostname and URL are immutable " +
					"for this endpoint UID and the URL routes to the referenced service's health-selected generation."},
		},
	}
}

type containerEndpointTargetResolver struct {
	base model.TargetContractResolver
}

func (resolver *containerEndpointTargetResolver) ResolveResourceTarget(target model.ResourceTarget) (model.ResolvedResourceTarget, error) {
	if target.Group == Family.APIVersion() && target.Kind == EdgeContainerServiceCandidateFormKind && target.Contract.ExactForm {
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			TargetFormRefs: []model.TargetFormRef{{
				APIVersion: Family.APIVersion(), Kind: EdgeContainerServiceCandidateFormKind,
				DefinitionVersion: EdgeContainerServiceCandidateFormVersion,
				SchemaDigest:      EdgeContainerServiceCandidateSchemaDigest,
			}},
		}, nil
	}
	return resolver.base.ResolveResourceTarget(target)
}

func containerEndpointPackageTargetRef() formpackage.FormRef {
	return formpackage.FormRef{
		APIVersion: Family.APIVersion(), Kind: EdgeContainerServiceCandidateFormKind,
		DefinitionVersion: EdgeContainerServiceCandidateFormVersion,
		SchemaDigest:      EdgeContainerServiceCandidateSchemaDigest,
	}
}

func assertContainerEndpointCandidateStatus(candidate EdgeContainerEndpointFormCandidate) error {
	if candidate.PublicationStatus != "UNPUBLISHED" {
		return fmt.Errorf("ContainerEndpoint publicationStatus = %q, want UNPUBLISHED", candidate.PublicationStatus)
	}
	if !strings.HasPrefix(candidate.ContractClosure, "form-only;") {
		return fmt.Errorf("ContainerEndpoint contractClosure %q is not Form-only", candidate.ContractClosure)
	}
	return nil
}
