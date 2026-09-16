package edgeformcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

func TestWorkflowWorkerCandidateRendersAndValidatesWithReleasedCore(t *testing.T) {
	t.Parallel()
	candidate, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.Form.DefinitionJSON)); err != nil {
		t.Fatalf("candidate DurableWorkflow rejected by released Core: %v", err)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(candidate.Interface.DefinitionJSON)); err != nil {
		t.Fatalf("candidate Interface rejected by released Core: %v", err)
	}
	if err := formpackage.ValidateBindingDefinition([]byte(candidate.Binding.DefinitionJSON)); err != nil {
		t.Fatalf("candidate Binding rejected by released Core: %v", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.WorkerVersion.DefinitionJSON)); err != nil {
		t.Fatalf("candidate WorkerVersion rejected by released Core: %v", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.WorkerDeployment.DefinitionJSON)); err != nil {
		t.Fatalf("candidate WorkerDeployment rejected by released Core: %v", err)
	}

	if candidate.Form.Kind != "DurableWorkflow" || candidate.Form.Slug != "durable-workflow" ||
		candidate.Form.Definition.DefinitionVersion != WorkflowCandidateFormVersion {
		t.Fatalf("candidate DurableWorkflow identity = %s/%s@%s", candidate.Form.Kind, candidate.Form.Slug, candidate.Form.Definition.DefinitionVersion)
	}
	if candidate.Interface.Name != WorkflowCandidateInterfaceName ||
		candidate.Interface.Version != WorkflowCandidateInterfaceVersion {
		t.Fatalf("candidate Interface identity = %s@%s", candidate.Interface.Name, candidate.Interface.Version)
	}
	if candidate.Binding.Name != WorkflowCandidateBindingName ||
		candidate.Binding.Version != WorkflowCandidateBindingVersion {
		t.Fatalf("candidate Binding identity = %s@%s", candidate.Binding.Name, candidate.Binding.Version)
	}
	if candidate.WorkerVersion.Definition.DefinitionVersion != WorkflowWorkerVersionCandidateVersion {
		t.Fatalf("candidate WorkerVersion definitionVersion = %q, want %q", candidate.WorkerVersion.Definition.DefinitionVersion, WorkflowWorkerVersionCandidateVersion)
	}
	if candidate.WorkerDeployment.Definition.DefinitionVersion != WorkflowWorkerDeploymentCandidateVersion {
		t.Fatalf("candidate WorkerDeployment definitionVersion = %q, want %q", candidate.WorkerDeployment.Definition.DefinitionVersion, WorkflowWorkerDeploymentCandidateVersion)
	}

	for name, contract := range map[string]RenderedContract{
		"Interface": candidate.Interface,
		"Binding":   candidate.Binding,
	} {
		digest, err := formpackage.DigestCanonicalJSON([]byte(contract.DefinitionJSON))
		if err != nil {
			t.Fatalf("%s digest: %v", name, err)
		}
		if digest != contract.SchemaDigest {
			t.Fatalf("%s schemaDigest = %q, computed %q", name, contract.SchemaDigest, digest)
		}
	}
}

func TestWorkflowWorkerCandidateReplacesWorkflowBindingAndRetargetsField(t *testing.T) {
	t.Parallel()
	candidate, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}

	var binding BindingDefinition
	if err := json.Unmarshal([]byte(candidate.Binding.DefinitionJSON), &binding); err != nil {
		t.Fatal(err)
	}
	wantInterface := formpackage.InterfaceRef{
		APIVersion:   InterfaceAPIVersion,
		Name:         candidate.Interface.Name,
		Version:      candidate.Interface.Version,
		SchemaDigest: candidate.Interface.SchemaDigest,
	}
	if !reflect.DeepEqual(binding.TargetInterface, wantInterface) {
		t.Fatalf("candidate Binding targetInterface = %#v, want %#v", binding.TargetInterface, wantInterface)
	}
	if got, want := binding.RuntimeProjection.Operations,
		[]string{"create", "get", "status", "sendEvent", "terminate"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate caller operations = %#v, want %#v", got, want)
	}
	for _, phrase := range []string{
		"worker.workflow@2.0.0",
		"callee ABI",
		"create({id, params})",
		"get(id)",
		"status()",
		"sendEvent({type, payload})",
		"terminate()",
		"WorkflowInstance.id",
		"not exposed to callers",
	} {
		if !strings.Contains(binding.Description, phrase) {
			t.Errorf("candidate Binding description is missing %q: %s", phrase, binding.Description)
		}
	}

	workflowRefs := make([]formpackage.BindingRef, 0, 1)
	for _, ref := range candidate.WorkerVersion.Definition.AcceptedBindings {
		if ref.Name == WorkflowCandidateBindingName {
			workflowRefs = append(workflowRefs, ref)
		}
	}
	if len(workflowRefs) != 1 {
		t.Fatalf("candidate WorkerVersion has %d workflow BindingRefs, want one", len(workflowRefs))
	}
	if got := workflowRefs[0]; got.APIVersion != BindingAPIVersion || got.Version != candidate.Binding.Version ||
		got.SchemaDigest != candidate.Binding.SchemaDigest {
		t.Fatalf("candidate workflow BindingRef = %#v, want exact forward Binding", got)
	}
	if workflowRefs[0].Version == "1.0.0" {
		t.Fatal("candidate WorkerVersion retained old workflow Binding version")
	}

	properties, ok := candidate.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("WorkerVersion desired schema properties = %#v", candidate.WorkerVersion.Definition.DesiredSchema["properties"])
	}
	workflowBindings, ok := properties["workflowBindings"].(map[string]any)
	if !ok {
		t.Fatalf("WorkerVersion workflowBindings schema = %#v", properties["workflowBindings"])
	}
	if got := workflowBindings[model.BindingAnnotationKey]; got != WorkflowCandidateBindingName {
		t.Fatalf("workflowBindings binding annotation = %#v, want %q", got, WorkflowCandidateBindingName)
	}
	items, ok := workflowBindings["items"].(map[string]any)
	if !ok {
		t.Fatalf("workflowBindings items = %#v", workflowBindings["items"])
	}
	resource, ok := items["properties"].(map[string]any)["resource"].(map[string]any)
	if !ok {
		t.Fatalf("workflowBindings resource schema = %#v", items["properties"])
	}
	annotation, ok := resource[model.RequiredInterfaceAnnotationKey].(map[string]any)
	if !ok {
		t.Fatalf("workflowBindings required Interface annotation = %#v", resource[model.RequiredInterfaceAnnotationKey])
	}
	if annotation["name"] != candidate.Interface.Name || annotation["version"] != candidate.Interface.Version ||
		annotation["schemaDigest"] != candidate.Interface.SchemaDigest {
		t.Fatalf("workflowBindings required Interface annotation = %#v, want exact candidate Interface", annotation)
	}
}

func TestWorkflowWorkerCandidateIncludesExactForwardDeployment(t *testing.T) {
	t.Parallel()
	candidate, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	workerDigest, err := formpackage.DigestCanonicalJSON([]byte(candidate.WorkerVersion.DefinitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	var deployment formpackage.FormDefinition
	if err := json.Unmarshal([]byte(candidate.WorkerDeployment.DefinitionJSON), &deployment); err != nil {
		t.Fatal(err)
	}
	relations, err := model.DeriveRelations(deployment.DesiredSchema)
	if err != nil {
		t.Fatal(err)
	}
	foundVersion := false
	var workerRelation model.Relation
	for _, relation := range relations {
		switch relation.Pointer {
		case "/versions/*/workerVersion":
			foundVersion = true
			want := []model.TargetFormRef{{
				APIVersion: Family.APIVersion(), Kind: "WorkerVersion",
				DefinitionVersion: candidate.WorkerVersion.Definition.DefinitionVersion,
				SchemaDigest:      workerDigest,
			}}
			if !reflect.DeepEqual(relation.TargetFormRefs, want) {
				t.Fatalf("forward deployment WorkerVersion ref = %#v, want %#v", relation.TargetFormRefs, want)
			}
		case "/worker":
			workerRelation = relation
		}
	}
	if !foundVersion {
		t.Fatal("forward deployment has no WorkerVersion relation")
	}
	if len(workerRelation.TargetFormRefs) == 0 {
		t.Fatal("forward deployment has no ModuleWorker relation")
	}

	base, ok := ByKind("WorkerDeployment")
	if !ok {
		t.Fatal("current WorkerDeployment is missing")
	}
	current, err := renderForm(base, newTargetContractResolver())
	if err != nil {
		t.Fatal(err)
	}
	var currentDefinition formpackage.FormDefinition
	if err := json.Unmarshal([]byte(current.DefinitionJSON), &currentDefinition); err != nil {
		t.Fatal(err)
	}
	currentRelations, err := model.DeriveRelations(currentDefinition.DesiredSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range currentRelations {
		if relation.Pointer == "/worker" && !reflect.DeepEqual(workerRelation.TargetFormRefs, relation.TargetFormRefs) {
			t.Fatalf("forward deployment changed ModuleWorker ref = %#v, current %#v", workerRelation.TargetFormRefs, relation.TargetFormRefs)
		}
	}
}

func TestWorkflowWorkerCandidateRenderingLeavesCurrentCatalogUnchangedAndIsStable(t *testing.T) {
	t.Parallel()
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
	first, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated workflow candidate rendering is not stable")
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
	if !reflect.DeepEqual(beforeForms, afterForms) {
		t.Fatal("workflow candidate rendering changed current Forms")
	}
	if !reflect.DeepEqual(beforeInterfaces, afterInterfaces) {
		t.Fatal("workflow candidate rendering changed current Interfaces")
	}
	if !reflect.DeepEqual(beforeBindings, afterBindings) {
		t.Fatal("workflow candidate rendering changed current Bindings")
	}
	for _, binding := range afterBindings {
		if binding.Name == WorkflowCandidateBindingName && binding.Version == WorkflowCandidateBindingVersion {
			t.Fatal("workflow candidate Binding was registered in current Bindings")
		}
	}
}
