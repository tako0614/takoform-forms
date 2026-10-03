package edgeformcatalog

import (
	"fmt"
	"reflect"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// ActorWorkflowCandidate is one unpublished, source-only Actor+Workflow
// successor. It deliberately has one WorkerVersion/WorkerDeployment pair and
// contains no Vector or Container contract. The numeric development versions
// carried by its rendered artifacts are not release allocations.
type ActorWorkflowCandidate struct {
	RuntimeInterface  RenderedContract     `json:"runtimeInterface"`
	ModuleWorker      RenderedForm         `json:"moduleWorker"`
	Actor             ActorCandidate       `json:"actor"`
	Workflow          RuntimeCandidatePair `json:"workflow"`
	RuntimeDependants []RenderedForm       `json:"runtimeDependants"`
	WorkerVersion     RenderedForm         `json:"workerVersion"`
	WorkerDeployment  RenderedForm         `json:"workerDeployment"`
}

const actorWorkflowLifecycleDescription = " Generic DELETE is a side-effect-free refusal before mutation: " +
	"a live Binding or any queued/running/sleeping/waiting instance returns dependency_in_use (409). " +
	"These active instances are dependent execution identities under this Workflow UID; an execution " +
	"owner/continuation that could still commit also keeps DELETE refused with dependency_in_use (409), " +
	"preserving instance state/history. The failure detail identifies whether a live Binding or an active " +
	"execution identity is the dependency; resource_busy is reserved for bounded transient concurrent mutation " +
	"or index maintenance, never this long-lived refusal. " +
	"Only when every instance is terminal, no Binding remains, and all owners are stopped and fenced may DELETE " +
	"succeed and purge terminal histories and unmatched queued events. maxTerminalRetentionSeconds=2592000 bounds " +
	"retention only while this DurableWorkflow UID lives; successful DELETE is the sole early-purge exception. " +
	"Recreate has a new Host UID, starts empty, and old owner tokens cannot commit. This identity has no update: " +
	"className and worker remain immutable. Code/weight promotion is through WorkerDeployment only. " +
	"Each execution context pins one exact WorkerVersion; retry/wake selects the current weighted deployment anew. " +
	"History is never migrated or rewritten; promoted Versions must be step-history compatible, or the " +
	"deployment/workflow stays not Ready rather than silently migrating."

// RenderActorWorkflowCandidate composes the existing reviewed source-only
// renderers without selecting the unrelated Vector proposal or writing any
// current candidate, release, trust, or Host-support state.
func RenderActorWorkflowCandidate() (ActorWorkflowCandidate, error) {
	base, err := renderRuntimeCandidate(false)
	if err != nil {
		return ActorWorkflowCandidate{}, err
	}
	candidate := ActorWorkflowCandidate{
		RuntimeInterface:  base.RuntimeInterface,
		ModuleWorker:      base.ModuleWorker,
		Actor:             base.Actor,
		Workflow:          base.Workflow,
		RuntimeDependants: base.RuntimeDependants,
		WorkerVersion:     base.WorkerVersion,
		WorkerDeployment:  base.WorkerDeployment,
	}
	if err := validateActorWorkflowCandidate(candidate); err != nil {
		return ActorWorkflowCandidate{}, err
	}
	return candidate, nil
}

func validateActorWorkflowCandidate(candidate ActorWorkflowCandidate) error {
	for _, contract := range []RenderedContract{
		candidate.RuntimeInterface, candidate.Actor.Interface, candidate.Actor.Binding,
		candidate.Workflow.Interface, candidate.Workflow.Binding,
	} {
		if err := validateRenderedContract(contract); err != nil {
			return err
		}
	}
	if err := validateProvidedInterface(candidate.ModuleWorker, requiredInterfaceFromContract(candidate.RuntimeInterface)); err != nil {
		return err
	}
	if err := validateProvidedInterface(candidate.Actor.Form, requiredInterfaceFromContract(candidate.Actor.Interface)); err != nil {
		return err
	}
	if err := validateProvidedInterface(candidate.Workflow.Form, requiredInterfaceFromContract(candidate.Workflow.Interface)); err != nil {
		return err
	}
	expected := map[string]model.RequiredInterface{
		candidate.RuntimeInterface.Name:   requiredInterfaceFromContract(candidate.RuntimeInterface),
		candidate.Actor.Interface.Name:    requiredInterfaceFromContract(candidate.Actor.Interface),
		candidate.Workflow.Interface.Name: requiredInterfaceFromContract(candidate.Workflow.Interface),
	}
	forms := []RenderedForm{
		candidate.ModuleWorker, candidate.Actor.Form, candidate.Workflow.Form,
		candidate.WorkerVersion, candidate.WorkerDeployment,
	}
	forms = append(forms, candidate.RuntimeDependants...)
	if len(forms) != 9 {
		return fmt.Errorf("Actor+Workflow candidate has %d Forms, want nine", len(forms))
	}
	seenKinds := make(map[string]bool, len(forms))
	for _, form := range forms {
		if seenKinds[form.Kind] {
			return fmt.Errorf("Actor+Workflow candidate repeats Form %s", form.Kind)
		}
		seenKinds[form.Kind] = true
		if _, err := formpackage.ValidateDefinition([]byte(form.DefinitionJSON)); err != nil {
			return fmt.Errorf("%s Core Form validation: %w", form.Kind, err)
		}
		if err := validateCandidateInterfaceRefs(form.Definition.DesiredSchema, expected); err != nil {
			return fmt.Errorf("%s: %w", form.Kind, err)
		}
	}
	refs := candidate.WorkerVersion.Definition.AcceptedBindings
	if len(refs) != 7 || countBindingName(refs, ActorCandidateBindingName) != 1 ||
		countBindingName(refs, WorkflowCandidateBindingName) != 1 ||
		countBindingName(refs, VectorIndexCandidateBindingName) != 0 {
		return fmt.Errorf("Actor+Workflow WorkerVersion has missing, duplicate, or unwanted BindingRefs")
	}
	for _, pair := range []RenderedContract{candidate.Actor.Binding, candidate.Workflow.Binding} {
		found := false
		for _, ref := range refs {
			if ref.Name == pair.Name {
				found = ref.Version == pair.Version && ref.SchemaDigest == pair.SchemaDigest
			}
		}
		if !found {
			return fmt.Errorf("Actor+Workflow WorkerVersion has stale %s BindingRef", pair.Name)
		}
	}
	currentVersion, ok := ByKind("WorkerVersion")
	if !ok {
		return fmt.Errorf("current catalog has no WorkerVersion Form")
	}
	retainedRefs, err := resolveBindingRefs(currentVersion.AcceptedBindings)
	if err != nil {
		return fmt.Errorf("current WorkerVersion BindingRefs: %w", err)
	}
	for _, retained := range retainedRefs {
		if retained.Name == ActorCandidateBindingName || retained.Name == WorkflowCandidateBindingName {
			continue
		}
		matched := false
		for _, ref := range refs {
			if ref.Name == retained.Name && reflect.DeepEqual(ref, retained) {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("Actor+Workflow WorkerVersion lost or changed retained %s BindingRef", retained.Name)
		}
	}
	properties, ok := candidate.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	if !ok || properties["vectorBindings"] != nil ||
		!candidateBindingFieldMatches(properties["actorBindings"], candidate.Actor.Interface, ActorCandidateBindingName) ||
		!candidateBindingFieldMatches(properties["workflowBindings"], candidate.Workflow.Interface, WorkflowCandidateBindingName) {
		return fmt.Errorf("Actor+Workflow WorkerVersion binding fields are missing, stale, or include Vector")
	}
	workerRef, err := renderedFormRef(candidate.ModuleWorker)
	if err != nil {
		return err
	}
	versionRef, err := renderedFormRef(candidate.WorkerVersion)
	if err != nil {
		return err
	}
	relations, err := model.DeriveRelations(candidate.WorkerDeployment.Definition.DesiredSchema)
	if err != nil {
		return fmt.Errorf("Actor+Workflow deployment relations: %w", err)
	}
	seenWorker, seenVersion := false, false
	for _, relation := range relations {
		switch relation.Pointer {
		case "/worker":
			seenWorker = true
			if !reflect.DeepEqual(relation.TargetFormRefs, []model.TargetFormRef{workerRef}) {
				return fmt.Errorf("Actor+Workflow deployment target /worker is not the exact ModuleWorker successor")
			}
		case "/versions/*/workerVersion":
			seenVersion = true
			if !reflect.DeepEqual(relation.TargetFormRefs, []model.TargetFormRef{versionRef}) {
				return fmt.Errorf("Actor+Workflow deployment target /versions/*/workerVersion is not the exact WorkerVersion successor")
			}
		}
	}
	if !seenWorker || !seenVersion {
		return fmt.Errorf("Actor+Workflow deployment target relation is missing")
	}
	return nil
}

func candidateBindingFieldMatches(raw any, iface RenderedContract, bindingName string) bool {
	field, ok := raw.(map[string]any)
	if !ok || field["x-takoform-binding"] != bindingName {
		return false
	}
	items, ok := field["items"].(map[string]any)
	if !ok {
		return false
	}
	properties, ok := items["properties"].(map[string]any)
	if !ok {
		return false
	}
	resource, ok := properties["resource"].(map[string]any)
	if !ok {
		return false
	}
	ref, ok := resource[model.RequiredInterfaceAnnotationKey].(map[string]any)
	return ok && ref["apiVersion"] == InterfaceAPIVersion && ref["name"] == iface.Name &&
		ref["version"] == iface.Version && ref["schemaDigest"] == iface.SchemaDigest
}
