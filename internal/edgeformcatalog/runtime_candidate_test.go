package edgeformcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

func TestRuntimeCandidateRendersOneCoreValidatedClosure(t *testing.T) {
	candidate, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(candidate.RuntimeInterface.DefinitionJSON)); err != nil {
		t.Fatalf("runtime Interface rejected by Core: %v", err)
	}
	for name, contract := range map[string]RenderedContract{
		"actor Interface":    candidate.Actor.Interface,
		"actor Binding":      candidate.Actor.Binding,
		"workflow Interface": candidate.Workflow.Interface,
		"workflow Binding":   candidate.Workflow.Binding,
		"vector Interface":   candidate.Vector.Interface,
		"vector Binding":     candidate.Vector.Binding,
	} {
		if strings.HasSuffix(name, "Interface") {
			if err := formpackage.ValidateInterfaceDefinition([]byte(contract.DefinitionJSON)); err != nil {
				t.Fatalf("%s rejected by Core: %v", name, err)
			}
		} else if err := formpackage.ValidateBindingDefinition([]byte(contract.DefinitionJSON)); err != nil {
			t.Fatalf("%s rejected by Core: %v", name, err)
		}
	}
	forms := []RenderedForm{
		candidate.ModuleWorker, candidate.Actor.Form, candidate.Workflow.Form, candidate.Vector.Form,
		candidate.WorkerVersion, candidate.WorkerDeployment,
	}
	forms = append(forms, candidate.RuntimeDependants...)
	for _, form := range forms {
		if _, err := formpackage.ValidateDefinition([]byte(form.DefinitionJSON)); err != nil {
			t.Fatalf("%s@%s rejected by Core: %v", form.Kind, form.Definition.DefinitionVersion, err)
		}
	}
	if candidate.RuntimeInterface.Name != RuntimeCandidateInterfaceName || candidate.RuntimeInterface.Version != RuntimeCandidateInterfaceVersion {
		t.Fatalf("runtime Interface identity = %s@%s", candidate.RuntimeInterface.Name, candidate.RuntimeInterface.Version)
	}
	if candidate.Actor.Interface.Version != ActorCandidateInterfaceVersion || candidate.Actor.Binding.Version != ActorCandidateBindingVersion {
		t.Fatalf("actor contract identities = %s@%s and %s@%s", candidate.Actor.Interface.Name, candidate.Actor.Interface.Version, candidate.Actor.Binding.Name, candidate.Actor.Binding.Version)
	}
	if candidate.WorkerVersion.Definition.DefinitionVersion != RuntimeCandidateWorkerVersionVersion ||
		candidate.WorkerDeployment.Definition.DefinitionVersion != RuntimeCandidateWorkerDeploymentVersion {
		t.Fatalf("aggregate worker identities = %s and %s", candidate.WorkerVersion.Definition.DefinitionVersion, candidate.WorkerDeployment.Definition.DefinitionVersion)
	}
}

func TestRuntimeCandidateUsesOneSharedWorkerPairAndExactBindingRefs(t *testing.T) {
	candidate, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	refs := candidate.WorkerVersion.Definition.AcceptedBindings
	if len(refs) != 8 {
		t.Fatalf("aggregate WorkerVersion acceptedBindings = %d, want eight", len(refs))
	}
	want := map[string]RenderedContract{
		ActorCandidateBindingName:       candidate.Actor.Binding,
		WorkflowCandidateBindingName:    candidate.Workflow.Binding,
		VectorIndexCandidateBindingName: candidate.Vector.Binding,
	}
	counts := map[string]int{}
	for _, ref := range refs {
		counts[ref.Name]++
		if contract, ok := want[ref.Name]; ok {
			if ref.Version != contract.Version || ref.SchemaDigest != contract.SchemaDigest {
				t.Fatalf("BindingRef %s = %#v, want exact %s@%s %s", ref.Name, ref, contract.Name, contract.Version, contract.SchemaDigest)
			}
			if ref.Name == ActorCandidateBindingName || ref.Name == WorkflowCandidateBindingName {
				if ref.Version == "1.0.0" {
					t.Fatalf("aggregate retained old %s Binding version", ref.Name)
				}
			}
		}
	}
	for name := range want {
		if counts[name] != 1 {
			t.Fatalf("aggregate WorkerVersion has %d refs named %s, want one", counts[name], name)
		}
	}
	properties, ok := candidate.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("WorkerVersion desired schema has no properties")
	}
	for wire, contract := range map[string]RenderedContract{
		"actorBindings":    candidate.Actor.Interface,
		"workflowBindings": candidate.Workflow.Interface,
		"vectorBindings":   candidate.Vector.Interface,
	} {
		field, ok := properties[wire].(map[string]any)
		if !ok {
			t.Fatalf("WorkerVersion has no %s field", wire)
		}
		items, ok := field["items"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no items schema", wire)
		}
		itemProperties, ok := items["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s items have no properties", wire)
		}
		resource, ok := itemProperties["resource"].(map[string]any)
		if !ok {
			t.Fatalf("%s items have no resource", wire)
		}
		annotation, ok := resource[model.RequiredInterfaceAnnotationKey].(map[string]any)
		if !ok || annotation["name"] != contract.Name || annotation["version"] != contract.Version || annotation["schemaDigest"] != contract.SchemaDigest {
			t.Fatalf("%s required Interface annotation = %#v, want %s@%s %s", wire, resource[model.RequiredInterfaceAnnotationKey], contract.Name, contract.Version, contract.SchemaDigest)
		}
	}
}

func TestRuntimeCandidatePropagatesRuntimeAndModuleWorkerExactRefsOnce(t *testing.T) {
	candidate, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	runtime := requiredInterfaceFromContract(candidate.RuntimeInterface)
	forms := map[string]RenderedForm{
		candidate.ModuleWorker.Kind:  candidate.ModuleWorker,
		candidate.Actor.Form.Kind:    candidate.Actor.Form,
		candidate.Workflow.Form.Kind: candidate.Workflow.Form,
		candidate.WorkerVersion.Kind: candidate.WorkerVersion,
	}
	for _, form := range candidate.RuntimeDependants {
		forms[form.Kind] = form
	}
	for kind, form := range forms {
		stale, err := staleRuntimeRefs(form.Definition.DesiredSchema, runtime)
		if err != nil {
			t.Fatalf("%s runtime references: %v", kind, err)
		}
		if stale {
			t.Fatalf("%s retained an old worker.runtime reference", kind)
		}
	}
	provided := candidate.ModuleWorker.Definition.ProvidedInterfaces
	if len(provided) != 2 || provided[0].Name != runtime.Name || provided[0].Version != runtime.Version || provided[0].SchemaDigest != runtime.SchemaDigest {
		t.Fatalf("ModuleWorker providedInterfaces = %#v, want candidate runtime plus worker.service", provided)
	}
	workerRef, err := renderedFormRef(candidate.ModuleWorker)
	if err != nil {
		t.Fatal(err)
	}
	versionRef, err := renderedFormRef(candidate.WorkerVersion)
	if err != nil {
		t.Fatal(err)
	}
	relations, err := model.DeriveRelations(candidate.WorkerDeployment.Definition.DesiredSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		switch relation.Pointer {
		case "/worker":
			if !reflect.DeepEqual(relation.TargetFormRefs, []model.TargetFormRef{workerRef}) {
				t.Fatalf("aggregate deployment worker ref = %#v, want %#v", relation.TargetFormRefs, []model.TargetFormRef{workerRef})
			}
		case "/versions/*/workerVersion":
			if !reflect.DeepEqual(relation.TargetFormRefs, []model.TargetFormRef{versionRef}) {
				t.Fatalf("aggregate deployment WorkerVersion ref = %#v, want %#v", relation.TargetFormRefs, []model.TargetFormRef{versionRef})
			}
		}
	}
}

func TestRuntimeCandidatePreservesRegisteredCatalogAndRejectsStaleClosure(t *testing.T) {
	beforeForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	beforeInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	beforeBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	first, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("aggregate candidate rendering is not stable")
	}
	afterForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	afterInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	afterBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeForms, afterForms) || !reflect.DeepEqual(beforeInterfaces, afterInterfaces) || !reflect.DeepEqual(beforeBindings, afterBindings) {
		t.Fatal("aggregate rendering changed the registered catalog")
	}
	stale := first
	properties := stale.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	worker := properties["worker"].(map[string]any)
	worker[model.RequiredInterfaceAnnotationKey].(map[string]any)["version"] = "1.1.0"
	if err := validateRuntimeCandidate(stale); err == nil || !strings.Contains(err.Error(), "stale worker.runtime") {
		t.Fatalf("stale aggregate accepted or wrong error: %v", err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"container", "aiForm", "queueCandidate"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
			t.Fatalf("aggregate output unexpectedly contains %q", forbidden)
		}
	}
}

func TestActorCandidateCarriesResolvedSocketAndReservationRules(t *testing.T) {
	definition := ActorCandidateInterface()
	for _, phrase := range []string{
		"constructor(context, env)", "socketMessage", "socketClose", "socketError",
		"transport_error", "new Response(upgrade.body, upgrade)", "clone throws TypeError",
		"unbranded status-101", "HTTP 502", "before any 101", "no independent 30-second",
		"10000", "33554432", "16384", "123 UTF-8",
	} {
		if !strings.Contains(definition.Description, phrase) {
			t.Errorf("actor candidate Interface description missing %q", phrase)
		}
	}
	if !strings.Contains(definition.Description, "conflicting reserved") || !strings.Contains(definition.Description, "abandons") {
		t.Fatal("actor candidate does not state fail-closed handshake conflict semantics")
	}
}
