package edgeformcatalog

import (
	"fmt"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

const (
	// VectorIndexCandidateBindingName and Version identify an unpublished
	// development Binding. The numeric version is required by the released
	// Core schema; publication, registration, and Host support remain out of
	// scope for this candidate.
	VectorIndexCandidateBindingName    = "module-worker.edge-vector"
	VectorIndexCandidateBindingVersion = "1.0.0"

	// VectorWorkerVersionCandidateVersion advances the WorkerVersion identity
	// only in this forward candidate. The registered WorkerVersion remains
	// 0.3.0 and is never mutated by this renderer.
	VectorWorkerVersionCandidateVersion    = "0.4.0-dev.1"
	VectorWorkerDeploymentCandidateVersion = "0.3.0-dev.1"

	// This is the digest of the rendered candidate edge.vector Interface. It is
	// checked when the candidate is assembled so a changed Interface cannot be
	// silently projected through the Binding; it is not a publication or Host
	// support assertion.
	vectorIndexCandidateInterfaceSchemaDigest = "sha256:6df8b7680b0ff278cb8fcb6f56c602ec5d6bb9b4ec115a6172e70e6b54d6cada"
)

// RenderVectorIndexCandidate renders the unpublished Form, Interface,
// Binding and forward Worker Forms without modifying the registered catalog.
func RenderVectorIndexCandidate() (VectorIndexCandidate, error) {
	index, err := renderVectorIndexCandidatePair()
	if err != nil {
		return VectorIndexCandidate{}, err
	}
	binding, workerVersion, err := renderVectorWorkerArtifacts(index)
	if err != nil {
		return VectorIndexCandidate{}, err
	}
	index.Binding = binding
	index.WorkerVersion = workerVersion
	index.WorkerDeployment, err = renderVectorWorkerDeployment(workerVersion)
	if err != nil {
		return VectorIndexCandidate{}, err
	}
	return index, nil
}

// A Deployment reads its Version's desired state and therefore pins that
// exact Definition. The current Deployment cannot admit the forward Version;
// render a separate unpublished Definition instead of relaxing its old pin.
func renderVectorWorkerDeployment(workerVersion RenderedForm) (RenderedForm, error) {
	base, ok := ByKind("WorkerDeployment")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no WorkerDeployment Form")
	}
	digest, err := formpackage.DigestCanonicalJSON([]byte(workerVersion.DefinitionJSON))
	if err != nil {
		return RenderedForm{}, err
	}
	base.DefinitionVersion = VectorWorkerDeploymentCandidateVersion
	resolver := &vectorWorkerDeploymentResolver{
		base: newTargetContractResolver(),
		versionRef: model.TargetFormRef{
			APIVersion: Family.APIVersion(), Kind: workerVersion.Kind,
			DefinitionVersion: workerVersion.Definition.DefinitionVersion,
			SchemaDigest:      digest,
		},
	}
	deployment, err := renderForm(base, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("vector candidate WorkerDeployment: %w", err)
	}
	return deployment, nil
}

type vectorWorkerDeploymentResolver struct {
	base       *targetContractResolver
	versionRef model.TargetFormRef
}

func (r *vectorWorkerDeploymentResolver) ResolveResourceTarget(
	target model.ResourceTarget,
) (model.ResolvedResourceTarget, error) {
	if target.Group == r.versionRef.APIVersion && target.Kind == r.versionRef.Kind && target.Contract.ExactForm {
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			TargetFormRefs:      []model.TargetFormRef{r.versionRef},
		}, nil
	}
	return r.base.ResolveResourceTarget(target)
}

// renderVectorWorkerArtifacts builds the candidate Binding and WorkerVersion
// from an already rendered VectorIndex pair. Keeping this seam separate makes
// it impossible for WorkerVersion rendering to re-author or re-register the
// edge.vector Interface.
func renderVectorWorkerArtifacts(index VectorIndexCandidate) (RenderedContract, RenderedForm, error) {
	if index.Interface.Name != VectorIndexCandidateInterfaceName ||
		index.Interface.Version != VectorIndexCandidateInterfaceVersion {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"vector index candidate Interface identity = %s@%s, want %s@%s",
			index.Interface.Name, index.Interface.Version,
			VectorIndexCandidateInterfaceName, VectorIndexCandidateInterfaceVersion,
		)
	}
	if index.Interface.SchemaDigest != vectorIndexCandidateInterfaceSchemaDigest {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"vector index candidate Interface digest = %q, want %q",
			index.Interface.SchemaDigest, vectorIndexCandidateInterfaceSchemaDigest,
		)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(index.Interface.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate Interface: %w", err)
	}

	bindingDefinition := vectorIndexCandidateBinding(index.Interface)
	binding, err := renderContract(
		bindingDefinition.Name, bindingDefinition.Version, bindingDefinition,
	)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate Binding: %w", err)
	}
	if err := formpackage.ValidateBindingDefinition([]byte(binding.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate Binding: %w", err)
	}

	workerForm, err := vectorWorkerVersionForm()
	if err != nil {
		return RenderedContract{}, RenderedForm{}, err
	}
	if err := workerForm.Validate(); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate WorkerVersion authoring: %w", err)
	}
	resolver := newVectorWorkerCandidateResolver(index.Interface)
	workerVersion, err := renderForm(workerForm, resolver)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate WorkerVersion: %w", err)
	}

	// renderForm resolves only registered Binding references. Render the
	// existing seven exactly as the current catalog does, then append this
	// candidate's exact digest-bound BindingRef after that render. This keeps
	// current binding resolution untouched while making the forward extension
	// explicit in the WorkerVersion Definition.
	workerVersion.Definition.AcceptedBindings = append(
		append([]formpackage.BindingRef(nil), workerVersion.Definition.AcceptedBindings...),
		formpackage.BindingRef{
			APIVersion:   BindingAPIVersion,
			Name:         binding.Name,
			Version:      binding.Version,
			SchemaDigest: binding.SchemaDigest,
		},
	)
	workerVersion.DefinitionJSON, err = marshalIndented(workerVersion.Definition)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate WorkerVersion JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(workerVersion.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("vector index candidate WorkerVersion Core validation: %w", err)
	}
	return binding, workerVersion, nil
}

func vectorIndexCandidateBinding(iface RenderedContract) BindingDefinition {
	return BindingDefinition{
		APIVersion: BindingAPIVersion,
		Kind:       "BindingDefinition",
		Name:       VectorIndexCandidateBindingName,
		Version:    VectorIndexCandidateBindingVersion,
		Title:      "Module Worker vector index binding",
		Description: "Projects the exact edge.vector@0.1.0 Interface into an ES Module Worker " +
			"under one JavaScript binding name. The runtime surface is env.NAME with exactly these " +
			"methods: env.NAME.upsert(input), env.NAME.get(input), env.NAME.delete(input), and " +
			"env.NAME.query(input). Each method takes EXACTLY one argument: one closed JSON-object " +
			"input matching the corresponding Interface operation schema. Extra positional arguments, " +
			"missing arguments, unknown object members, and inputs that do not match the operation " +
			"schema are rejected before dispatch with Error.name=invalid_spec. Resource-dependent " +
			"constraints, such as the configured index dimension and filter keys, are validated by " +
			"the target Interface before effects and reject with the same error name. Every " +
			"method returns a Promise that resolves to the exact Interface output document: get resolves " +
			"to {vectors}, upsert and delete resolve to {ids,count}, and query resolves to {matches,count}; " +
			"no wrapper or provider-specific result is added. Other failures reject with an Error whose " +
			"name is the exact corresponding edge.vector Interface error code (invalid_spec, quota, or " +
			"unavailable); transport unavailable is named unavailable. The binding " +
			"exposes no vendor or native methods, native IDs, endpoints, credentials, or backend " +
			"metadata.",
		SourceRole: string(model.RoleRevision),
		TargetInterface: formpackage.InterfaceRef{
			APIVersion:   InterfaceAPIVersion,
			Name:         iface.Name,
			Version:      iface.Version,
			SchemaDigest: iface.SchemaDigest,
		},
		AllowedTargetForms: []AllowedTargetForm{{
			APIVersion: Family.APIVersion(),
			Kind:       VectorIndexCandidateFormKind,
		}},
		BindingNameGrammar: model.PatternBindingName,
		RuntimeProjection: RuntimeProjection{Operations: []string{
			"upsert", "get", "delete", "query",
		}},
		Lifecycle: BindingLifecycle{TargetDeletion: "refuse_while_bound"},
	}
}

func vectorWorkerVersionForm() (model.Form, error) {
	base, ok := ByKind("WorkerVersion")
	if !ok {
		return model.Form{}, fmt.Errorf("current catalog has no WorkerVersion Form")
	}
	if base.Role != model.RoleRevision || base.Kind != "WorkerVersion" || base.Slug != "worker-version" {
		return model.Form{}, fmt.Errorf(
			"current WorkerVersion identity drifted to %s/%s/%s",
			base.Kind, base.Slug, base.Role,
		)
	}
	if len(base.AcceptedBindings) != 7 {
		return model.Form{}, fmt.Errorf(
			"current WorkerVersion accepted %d bindings, want the existing seven",
			len(base.AcceptedBindings),
		)
	}

	// Copy every top-level slice before appending. ByKind returns a value whose
	// slices share backing arrays with the registered catalog; appending directly
	// would risk mutating that catalog when capacity happens to remain.
	candidate := base
	candidate.Fields = append([]model.Field(nil), base.Fields...)
	candidate.Outputs = append([]model.Field(nil), base.Outputs...)
	candidate.ProvidedInterfaces = append([]model.InterfaceRefSource(nil), base.ProvidedInterfaces...)
	candidate.AcceptedBindings = append([]model.BindingRefSource(nil), base.AcceptedBindings...)
	candidate.StructuralConstraints = append([]model.Constraint(nil), base.StructuralConstraints...)
	candidate.ResolvedUIDConstraints = append([]model.Constraint(nil), base.ResolvedUIDConstraints...)
	candidate.DefinitionVersion = VectorWorkerVersionCandidateVersion
	candidate.Fields = append(candidate.Fields, model.Field{
		HCL:         "vector_bindings",
		Wire:        "vectorBindings",
		Kind:        model.KindBindingList,
		TargetKind:  VectorIndexCandidateFormKind,
		BindingType: VectorIndexCandidateBindingName,
		Target:      requiresInterface(VectorIndexCandidateInterfaceName, VectorIndexCandidateInterfaceVersion),
		Default:     []any{},
		Doc: "Typed module-worker.edge-vector bindings projecting the exact edge.vector API under " +
			"JavaScript identifier names. Each env.NAME method accepts one closed operation input " +
			"object and returns the exact operation output document; omitting it declares no vector binding.",
		Example: []any{bindingInstance("VECTORS", VectorIndexCandidateFormKind, VectorIndexCandidateFormSlug)},
	})
	return candidate, nil
}

// vectorWorkerCandidateResolver delegates every registered target to the
// current catalog resolver and injects only the exact unpublished edge.vector
// Interface for the new VectorIndex binding field.
type vectorWorkerCandidateResolver struct {
	base         *targetContractResolver
	vectorRef    model.RequiredInterface
	vectorTarget model.ResourceTarget
}

func newVectorWorkerCandidateResolver(iface RenderedContract) *vectorWorkerCandidateResolver {
	return &vectorWorkerCandidateResolver{
		base: newTargetContractResolver(),
		vectorRef: model.RequiredInterface{
			APIVersion:   InterfaceAPIVersion,
			Name:         iface.Name,
			Version:      iface.Version,
			SchemaDigest: iface.SchemaDigest,
		},
		vectorTarget: model.ResourceTarget{
			Group: Family.APIVersion(), Kind: VectorIndexCandidateFormKind,
			Contract: model.TargetContract{Interface: &model.InterfaceRefSource{
				Name: VectorIndexCandidateInterfaceName, Version: VectorIndexCandidateInterfaceVersion,
			}},
		},
	}
}

func (r *vectorWorkerCandidateResolver) ResolveResourceTarget(
	target model.ResourceTarget,
) (model.ResolvedResourceTarget, error) {
	if target.Group == r.vectorTarget.Group &&
		target.Kind == r.vectorTarget.Kind &&
		target.Contract.Interface != nil &&
		target.Contract.Interface.Name == r.vectorRef.Name &&
		target.Contract.Interface.Version == r.vectorRef.Version {
		ref := r.vectorRef
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			RequiredInterface:   &ref,
		}, nil
	}
	return r.base.ResolveResourceTarget(target)
}
