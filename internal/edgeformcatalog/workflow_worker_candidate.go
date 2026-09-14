package edgeformcatalog

import (
	"fmt"
	"slices"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

const (
	// These are unpublished forward identities. The registered workflow
	// Binding, WorkerVersion, and WorkerDeployment remain unchanged.
	WorkflowCandidateBindingName             = "module-worker.workflow"
	WorkflowCandidateBindingVersion          = "2.0.0"
	WorkflowWorkerVersionCandidateVersion    = "0.4.0-workflow.1"
	WorkflowWorkerDeploymentCandidateVersion = "0.3.0-workflow.1"
)

// RenderWorkflowCandidate renders the forward workflow Interface pair and
// projects it into a new Binding, WorkerVersion, and WorkerDeployment. It does
// not register any of those artifacts in the current catalog.
func RenderWorkflowCandidate() (WorkflowCandidate, error) {
	candidate, err := renderWorkflowCandidatePair()
	if err != nil {
		return WorkflowCandidate{}, err
	}
	binding, workerVersion, err := renderWorkflowWorkerArtifacts(candidate)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	candidate.Binding = binding
	candidate.WorkerVersion = workerVersion
	candidate.WorkerDeployment, err = renderWorkflowWorkerDeployment(workerVersion)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	return candidate, nil
}

func renderWorkflowWorkerArtifacts(
	candidate WorkflowCandidate,
) (RenderedContract, RenderedForm, error) {
	iface := candidate.Interface
	if iface.Name != WorkflowCandidateInterfaceName || iface.Version != WorkflowCandidateInterfaceVersion {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate Interface identity = %s@%s, want %s@%s",
			iface.Name, iface.Version,
			WorkflowCandidateInterfaceName, WorkflowCandidateInterfaceVersion,
		)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(iface.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("workflow candidate Interface: %w", err)
	}
	computedDigest, err := formpackage.DigestCanonicalJSON([]byte(iface.DefinitionJSON))
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("workflow candidate Interface digest: %w", err)
	}
	if computedDigest != iface.SchemaDigest {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate Interface digest = %q, computed %q",
			iface.SchemaDigest, computedDigest,
		)
	}

	bindingDefinition, err := workflowCandidateBinding(iface)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, err
	}
	binding, err := renderContract(
		bindingDefinition.Name, bindingDefinition.Version, bindingDefinition,
	)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("workflow candidate Binding: %w", err)
	}
	if err := formpackage.ValidateBindingDefinition([]byte(binding.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("workflow candidate Binding: %w", err)
	}
	bindingDigest, err := formpackage.DigestCanonicalJSON([]byte(binding.DefinitionJSON))
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf("workflow candidate Binding digest: %w", err)
	}
	if bindingDigest != binding.SchemaDigest {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate Binding digest = %q, computed %q",
			binding.SchemaDigest, bindingDigest,
		)
	}

	workerForm, err := workflowWorkerVersionForm()
	if err != nil {
		return RenderedContract{}, RenderedForm{}, err
	}
	if err := workerForm.Validate(); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate WorkerVersion authoring: %w", err,
		)
	}
	workerVersion, err := renderForm(workerForm, newWorkflowWorkerCandidateResolver(iface))
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate WorkerVersion: %w", err,
		)
	}

	// The current render resolver can resolve only registered Binding
	// identities. It renders the cloned Form with the old workflow BindingRef,
	// then this forward projection replaces that one ref in the output. Keeping
	// the replacement by name (rather than appending) prevents two versions of
	// one caller capability from appearing in the same WorkerVersion.
	workflowBindingRefs := 0
	for index, ref := range workerVersion.Definition.AcceptedBindings {
		if ref.Name != WorkflowCandidateBindingName {
			continue
		}
		workflowBindingRefs++
		workerVersion.Definition.AcceptedBindings[index] = formpackage.BindingRef{
			APIVersion:   BindingAPIVersion,
			Name:         binding.Name,
			Version:      binding.Version,
			SchemaDigest: binding.SchemaDigest,
		}
	}
	if workflowBindingRefs != 1 {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"current WorkerVersion has %d %s accepted BindingRefs, want exactly one",
			workflowBindingRefs, WorkflowCandidateBindingName,
		)
	}
	workerVersion.DefinitionJSON, err = marshalIndented(workerVersion.Definition)
	if err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate WorkerVersion JSON: %w", err,
		)
	}
	if _, err := formpackage.ValidateDefinition([]byte(workerVersion.DefinitionJSON)); err != nil {
		return RenderedContract{}, RenderedForm{}, fmt.Errorf(
			"workflow candidate WorkerVersion Core validation: %w", err,
		)
	}
	return binding, workerVersion, nil
}

func workflowCandidateBinding(iface RenderedContract) (BindingDefinition, error) {
	definitions, err := BindingDefinitions()
	if err != nil {
		return BindingDefinition{}, fmt.Errorf("workflow candidate current Bindings: %w", err)
	}
	var current BindingDefinition
	found := false
	for _, definition := range definitions {
		if definition.Name != WorkflowCandidateBindingName {
			continue
		}
		if found {
			return BindingDefinition{}, fmt.Errorf(
				"current catalog has duplicate %s Bindings", WorkflowCandidateBindingName,
			)
		}
		current = definition
		found = true
	}
	if !found {
		return BindingDefinition{}, fmt.Errorf(
			"current catalog has no %s Binding", WorkflowCandidateBindingName,
		)
	}
	if got, want := current.RuntimeProjection.Operations,
		[]string{"create", "get", "status", "sendEvent", "terminate"}; !slices.Equal(got, want) {
		return BindingDefinition{}, fmt.Errorf(
			"current workflow Binding operations = %#v, want %#v", got, want,
		)
	}

	// BindingDefinitions returns values whose slices may share backing arrays
	// with the catalog's authoring data. Clone every slice before changing the
	// forward copy, even though this projection currently preserves them.
	current.AllowedTargetForms = append([]AllowedTargetForm(nil), current.AllowedTargetForms...)
	current.RuntimeProjection.Operations = append([]string(nil), current.RuntimeProjection.Operations...)
	current.Name = WorkflowCandidateBindingName
	current.Version = WorkflowCandidateBindingVersion
	current.TargetInterface = formpackage.InterfaceRef{
		APIVersion:   InterfaceAPIVersion,
		Name:         iface.Name,
		Version:      iface.Version,
		SchemaDigest: iface.SchemaDigest,
	}
	current.Description = "Unpublished forward Binding targeting the exact worker.workflow@2.0.0 Interface. " +
		"The callee ABI belongs to that Interface and is not exposed to callers. The following caller " +
		"projection is preserved, including the returned WorkflowInstance.id; its operation outcomes and " +
		"status reasons follow the exact new Interface. " + current.Description
	return current, nil
}

func workflowWorkerVersionForm() (model.Form, error) {
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

	candidate := base
	candidate.Fields = append([]model.Field(nil), base.Fields...)
	candidate.Outputs = append([]model.Field(nil), base.Outputs...)
	candidate.ProvidedInterfaces = append([]model.InterfaceRefSource(nil), base.ProvidedInterfaces...)
	candidate.AcceptedBindings = append([]model.BindingRefSource(nil), base.AcceptedBindings...)
	candidate.StructuralConstraints = append([]model.Constraint(nil), base.StructuralConstraints...)
	candidate.ResolvedUIDConstraints = append([]model.Constraint(nil), base.ResolvedUIDConstraints...)
	candidate.DefinitionVersion = WorkflowWorkerVersionCandidateVersion

	workflowFields := 0
	for index, field := range candidate.Fields {
		if field.Wire != "workflowBindings" {
			continue
		}
		workflowFields++
		required := model.TargetContract{
			Interface: &model.InterfaceRefSource{
				Name: WorkflowCandidateInterfaceName, Version: WorkflowCandidateInterfaceVersion,
			},
		}
		if field.ResourceTarget != nil {
			target := *field.ResourceTarget
			target.Contract = required
			field.ResourceTarget = &target
			field.Target = model.TargetContract{}
		} else {
			field.Target = required
		}
		candidate.Fields[index] = field
	}
	if workflowFields != 1 {
		return model.Form{}, fmt.Errorf(
			"current WorkerVersion has %d workflowBindings fields, want exactly one", workflowFields,
		)
	}
	return candidate, nil
}

type workflowWorkerCandidateResolver struct {
	base           *targetContractResolver
	workflowRef    model.RequiredInterface
	workflowTarget model.ResourceTarget
}

func newWorkflowWorkerCandidateResolver(iface RenderedContract) *workflowWorkerCandidateResolver {
	return &workflowWorkerCandidateResolver{
		base: newTargetContractResolver(),
		workflowRef: model.RequiredInterface{
			APIVersion:   InterfaceAPIVersion,
			Name:         iface.Name,
			Version:      iface.Version,
			SchemaDigest: iface.SchemaDigest,
		},
		workflowTarget: model.ResourceTarget{
			Group: Family.APIVersion(),
			Kind:  "DurableWorkflow",
			Contract: model.TargetContract{Interface: &model.InterfaceRefSource{
				Name: WorkflowCandidateInterfaceName, Version: WorkflowCandidateInterfaceVersion,
			}},
		},
	}
}

func (r *workflowWorkerCandidateResolver) ResolveResourceTarget(
	target model.ResourceTarget,
) (model.ResolvedResourceTarget, error) {
	if target.Group == r.workflowTarget.Group &&
		target.Kind == r.workflowTarget.Kind &&
		target.Contract.Interface != nil &&
		target.Contract.Interface.Name == r.workflowTarget.Contract.Interface.Name &&
		target.Contract.Interface.Version == r.workflowTarget.Contract.Interface.Version {
		ref := r.workflowRef
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			RequiredInterface:   &ref,
		}, nil
	}
	return r.base.ResolveResourceTarget(target)
}

func renderWorkflowWorkerDeployment(workerVersion RenderedForm) (RenderedForm, error) {
	base, ok := ByKind("WorkerDeployment")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no WorkerDeployment Form")
	}
	if base.Role != model.RoleDeployment || base.Kind != "WorkerDeployment" || base.Slug != "worker-deployment" {
		return RenderedForm{}, fmt.Errorf(
			"current WorkerDeployment identity drifted to %s/%s/%s",
			base.Kind, base.Slug, base.Role,
		)
	}

	digest, err := formpackage.DigestCanonicalJSON([]byte(workerVersion.DefinitionJSON))
	if err != nil {
		return RenderedForm{}, fmt.Errorf("workflow candidate WorkerVersion digest: %w", err)
	}
	candidate := base
	candidate.Fields = append([]model.Field(nil), base.Fields...)
	candidate.Outputs = append([]model.Field(nil), base.Outputs...)
	candidate.ProvidedInterfaces = append([]model.InterfaceRefSource(nil), base.ProvidedInterfaces...)
	candidate.AcceptedBindings = append([]model.BindingRefSource(nil), base.AcceptedBindings...)
	candidate.StructuralConstraints = append([]model.Constraint(nil), base.StructuralConstraints...)
	candidate.ResolvedUIDConstraints = append([]model.Constraint(nil), base.ResolvedUIDConstraints...)
	candidate.DefinitionVersion = WorkflowWorkerDeploymentCandidateVersion
	resolver := &workflowWorkerDeploymentResolver{
		base: newTargetContractResolver(),
		versionRef: model.TargetFormRef{
			APIVersion:        Family.APIVersion(),
			Kind:              workerVersion.Kind,
			DefinitionVersion: workerVersion.Definition.DefinitionVersion,
			SchemaDigest:      digest,
		},
	}
	deployment, err := renderForm(candidate, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("workflow candidate WorkerDeployment: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(deployment.DefinitionJSON)); err != nil {
		return RenderedForm{}, fmt.Errorf("workflow candidate WorkerDeployment Core validation: %w", err)
	}
	return deployment, nil
}

type workflowWorkerDeploymentResolver struct {
	base       *targetContractResolver
	versionRef model.TargetFormRef
}

func (r *workflowWorkerDeploymentResolver) ResolveResourceTarget(
	target model.ResourceTarget,
) (model.ResolvedResourceTarget, error) {
	if target.Group == r.versionRef.APIVersion &&
		target.Kind == r.versionRef.Kind &&
		target.Contract.ExactForm {
		ref := r.versionRef
		return model.ResolvedResourceTarget{
			ResourceNamePattern: model.PatternResourceName,
			TargetFormRefs:      []model.TargetFormRef{ref},
		}, nil
	}
	return r.base.ResolveResourceTarget(target)
}
