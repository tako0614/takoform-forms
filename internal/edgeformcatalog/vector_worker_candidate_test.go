package edgeformcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

func TestVectorWorkerCandidateRendersBindingAndForwardWorkerVersion(t *testing.T) {
	t.Parallel()
	candidate, err := RenderVectorIndexCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := formpackage.ValidateBindingDefinition([]byte(candidate.Binding.DefinitionJSON)); err != nil {
		t.Fatalf("candidate Binding rejected by released Core: %v", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.WorkerVersion.DefinitionJSON)); err != nil {
		t.Fatalf("candidate WorkerVersion rejected by released Core: %v", err)
	}

	if candidate.Binding.Name != VectorIndexCandidateBindingName ||
		candidate.Binding.Version != VectorIndexCandidateBindingVersion {
		t.Fatalf("Binding identity = %s@%s", candidate.Binding.Name, candidate.Binding.Version)
	}
	if candidate.Binding.SchemaDigest == "" {
		t.Fatal("candidate Binding has no schema digest")
	}
	if candidate.WorkerVersion.Kind != "WorkerVersion" || candidate.WorkerVersion.Slug != "worker-version" ||
		candidate.WorkerVersion.Role != string(model.RoleRevision) {
		t.Fatalf("WorkerVersion identity = %s/%s/%s", candidate.WorkerVersion.Kind, candidate.WorkerVersion.Slug, candidate.WorkerVersion.Role)
	}
	if got := candidate.WorkerVersion.Definition.DefinitionVersion; got != VectorWorkerVersionCandidateVersion {
		t.Fatalf("WorkerVersion definitionVersion = %q, want %q", got, VectorWorkerVersionCandidateVersion)
	}
	if got := len(candidate.WorkerVersion.Definition.AcceptedBindings); got != 8 {
		t.Fatalf("WorkerVersion acceptedBindings = %d, want seven existing plus vector", got)
	}
	last := candidate.WorkerVersion.Definition.AcceptedBindings[7]
	if last.APIVersion != BindingAPIVersion || last.Name != candidate.Binding.Name ||
		last.Version != candidate.Binding.Version || last.SchemaDigest != candidate.Binding.SchemaDigest {
		t.Fatalf("candidate BindingRef = %#v, want exact rendered Binding identity", last)
	}

	var bindingDefinition BindingDefinition
	if err := json.Unmarshal([]byte(candidate.Binding.DefinitionJSON), &bindingDefinition); err != nil {
		t.Fatal(err)
	}
	if bindingDefinition.SourceRole != string(model.RoleRevision) {
		t.Fatalf("Binding sourceRole = %q, want revision", bindingDefinition.SourceRole)
	}
	if got, want := bindingDefinition.TargetInterface.SchemaDigest, candidate.Interface.SchemaDigest; got != want {
		t.Fatalf("Binding target Interface digest = %q, want %q", got, want)
	}
	if got, want := bindingDefinition.TargetInterface.Name, VectorIndexCandidateInterfaceName; got != want {
		t.Fatalf("Binding target Interface name = %q, want %q", got, want)
	}
	if got, want := bindingDefinition.TargetInterface.Version, VectorIndexCandidateInterfaceVersion; got != want {
		t.Fatalf("Binding target Interface version = %q, want %q", got, want)
	}
	if got, want := bindingDefinition.AllowedTargetForms, []AllowedTargetForm{{
		APIVersion: Family.APIVersion(), Kind: VectorIndexCandidateFormKind,
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Binding allowedTargetForms = %#v, want %#v", got, want)
	}
	if got, want := bindingDefinition.BindingNameGrammar, model.PatternBindingName; got != want {
		t.Fatalf("Binding bindingNameGrammar = %q, want %q", got, want)
	}
	if got, want := bindingDefinition.RuntimeProjection.Operations, []string{"upsert", "get", "delete", "query"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Binding operations = %#v, want %#v", got, want)
	}
	if got := bindingDefinition.Lifecycle.TargetDeletion; got != "refuse_while_bound" {
		t.Fatalf("Binding targetDeletion = %q", got)
	}
	for _, phrase := range []string{
		"env.NAME",
		"EXACTLY one",
		"closed JSON-object",
		"upsert(input)", "get(input)", "delete(input)", "query(input)",
		"invalid_spec",
		"get resolves to {vectors}",
		"upsert and delete resolve to {ids,count}",
		"query resolves to {matches,count}",
		"Promise",
		"transport unavailable is named unavailable",
		"no vendor or native methods",
		"native IDs", "endpoints",
		"quota", "unavailable",
	} {
		if !strings.Contains(bindingDefinition.Description, phrase) {
			t.Errorf("Binding description is missing %q: %s", phrase, bindingDefinition.Description)
		}
	}
	for _, withdrawn := range []string{"not_found", "conflict"} {
		if strings.Contains(bindingDefinition.Description, withdrawn) {
			t.Errorf("Binding description carries withdrawn error code %q: %s", withdrawn, bindingDefinition.Description)
		}
	}

	properties, ok := candidate.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("WorkerVersion desired schema properties = %#v", candidate.WorkerVersion.Definition.DesiredSchema["properties"])
	}
	vectorBindings, ok := properties["vectorBindings"].(map[string]any)
	if !ok {
		t.Fatalf("WorkerVersion vectorBindings schema = %#v", properties["vectorBindings"])
	}
	if got := vectorBindings[model.BindingAnnotationKey]; got != VectorIndexCandidateBindingName {
		t.Fatalf("vectorBindings binding annotation = %#v, want %q", got, VectorIndexCandidateBindingName)
	}
	if got, ok := vectorBindings["default"].([]any); !ok || len(got) != 0 {
		t.Fatalf("vectorBindings default = %#v, want []", vectorBindings["default"])
	}
	items, ok := vectorBindings["items"].(map[string]any)
	if !ok {
		t.Fatalf("vectorBindings items = %#v", vectorBindings["items"])
	}
	resource, ok := items["properties"].(map[string]any)["resource"].(map[string]any)
	if !ok {
		t.Fatalf("vectorBindings resource schema = %#v", items["properties"])
	}
	if got := resource[model.RequiredInterfaceAnnotationKey]; got == nil {
		t.Fatal("vectorBindings resource has no required Interface annotation")
	} else {
		annotation, ok := got.(map[string]any)
		if !ok || annotation["name"] != VectorIndexCandidateInterfaceName ||
			annotation["version"] != VectorIndexCandidateInterfaceVersion ||
			annotation["schemaDigest"] != candidate.Interface.SchemaDigest {
			t.Fatalf("vectorBindings required Interface annotation = %#v", got)
		}
	}
}

func TestVectorWorkerCandidatePreservesCurrentWorkerVersionAndBindings(t *testing.T) {
	t.Parallel()
	base, ok := ByKind("WorkerVersion")
	if !ok {
		t.Fatal("current WorkerVersion is missing")
	}
	candidateForm, err := vectorWorkerVersionForm()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(candidateForm.Fields), len(base.Fields)+1; got != want {
		t.Fatalf("candidate WorkerVersion field count = %d, want %d", got, want)
	}
	for index, field := range base.Fields {
		if !reflect.DeepEqual(candidateForm.Fields[index], field) {
			t.Fatalf("candidate changed existing WorkerVersion field %d: %#v != %#v", index, candidateForm.Fields[index], field)
		}
	}
	if got, want := len(candidateForm.AcceptedBindings), len(base.AcceptedBindings); got != want {
		t.Fatalf("candidate authoring acceptedBindings = %d, want %d", got, want)
	}
	if !reflect.DeepEqual(candidateForm.AcceptedBindings, base.AcceptedBindings) {
		t.Fatalf("candidate changed existing acceptedBindings: %#v != %#v", candidateForm.AcceptedBindings, base.AcceptedBindings)
	}
	vectorField := candidateForm.Fields[len(candidateForm.Fields)-1]
	if vectorField.HCL != "vector_bindings" || vectorField.Wire != "vectorBindings" ||
		vectorField.Kind != model.KindBindingList || vectorField.TargetKind != VectorIndexCandidateFormKind ||
		vectorField.BindingType != VectorIndexCandidateBindingName {
		t.Fatalf("vector binding field = %#v", vectorField)
	}
	if vectorField.Default == nil {
		t.Fatal("vector binding field has no default")
	}
	if vectorField.Target.Interface == nil || vectorField.Target.Interface.Name != VectorIndexCandidateInterfaceName ||
		vectorField.Target.Interface.Version != VectorIndexCandidateInterfaceVersion {
		t.Fatalf("vector binding target Interface = %#v", vectorField.Target)
	}
	if err := model.ValidateEnvironmentNamespace(candidateForm); err != nil {
		t.Fatalf("candidate WorkerVersion canonical environment names collide: %v", err)
	}
	// EnvironmentNameFields discovers every binding list dynamically, so the
	// forward vector slot participates in the same collision rule as vars and
	// the existing binding slots without changing the registered catalog.
	colliding := candidateForm
	colliding.Fields = append([]model.Field(nil), candidateForm.Fields...)
	colliding.Fields[len(colliding.Fields)-1].Example = []any{
		bindingInstance("CACHE", VectorIndexCandidateFormKind, VectorIndexCandidateFormSlug),
	}
	if err := model.ValidateEnvironmentNamespace(colliding); err == nil {
		t.Fatal("candidate vector binding accepted a canonical name already used by kvBindings")
	} else if !strings.Contains(err.Error(), "CACHE") || !strings.Contains(err.Error(), "vectorBindings") {
		t.Fatalf("vector binding collision refusal omitted the colliding name or field: %v", err)
	}

	after, ok := ByKind("WorkerVersion")
	if !ok {
		t.Fatal("current WorkerVersion disappeared")
	}
	if !reflect.DeepEqual(base, after) {
		t.Fatal("building the WorkerVersion candidate mutated the registered WorkerVersion")
	}
}

func TestVectorWorkerCandidateDoesNotRegisterBinding(t *testing.T) {
	t.Parallel()
	before, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(before), 7; got != want {
		t.Fatalf("current Binding count = %d, want %d", got, want)
	}
	if _, err := RenderVectorIndexCandidate(); err != nil {
		t.Fatal(err)
	}
	after, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rendering the vector candidate changed registered Bindings")
	}
	for _, binding := range after {
		if binding.Name == VectorIndexCandidateBindingName {
			t.Fatal("candidate Binding was registered in current Bindings")
		}
	}
}

func TestVectorWorkerCandidateIncludesExactForwardDeployment(t *testing.T) {
	t.Parallel()
	base, ok := ByKind("WorkerDeployment")
	if !ok {
		t.Fatal("current WorkerDeployment is missing")
	}
	before, err := renderForm(base, newTargetContractResolver())
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RenderVectorIndexCandidate()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	var artifacts map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &artifacts); err != nil {
		t.Fatal(err)
	}
	data, exists := artifacts["workerDeployment"]
	if !exists {
		t.Fatal("vector candidate has no WorkerDeployment; the current deployment pins the old WorkerVersion")
	}
	var deployment RenderedForm
	if err := json.Unmarshal(data, &deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.Kind != "WorkerDeployment" || deployment.Definition.DefinitionVersion != "0.3.0-dev.1" {
		t.Fatalf("unexpected forward deployment identity: %s@%s", deployment.Kind, deployment.Definition.DefinitionVersion)
	}
	if _, err := formpackage.ValidateDefinition([]byte(deployment.DefinitionJSON)); err != nil {
		t.Fatalf("candidate WorkerDeployment rejected by released Core: %v", err)
	}
	digest, err := formpackage.DigestCanonicalJSON([]byte(candidate.WorkerVersion.DefinitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	// Parse the exact CLI bytes: decoded schema arrays use the same shape a
	// Host consumes, rather than the renderer's in-memory Go slice types.
	var definition formpackage.FormDefinition
	if err := json.Unmarshal([]byte(deployment.DefinitionJSON), &definition); err != nil {
		t.Fatal(err)
	}
	relations, err := model.DeriveRelations(definition.DesiredSchema)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, relation := range relations {
		if relation.Pointer != "/versions/*/workerVersion" {
			continue
		}
		found = true
		want := []model.TargetFormRef{{
			APIVersion: Family.APIVersion(), Kind: "WorkerVersion",
			DefinitionVersion: candidate.WorkerVersion.Definition.DefinitionVersion,
			SchemaDigest:      digest,
		}}
		if !reflect.DeepEqual(relation.TargetFormRefs, want) {
			t.Fatalf("deployment does not pin its exact candidate version: %#v, want %#v", relation.TargetFormRefs, want)
		}
	}
	if !found {
		t.Fatal("candidate deployment has no WorkerVersion relation")
	}
	after, err := renderForm(base, newTargetContractResolver())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("candidate rendering changed the registered WorkerDeployment")
	}
	beforeFixtures, err := json.Marshal(before.Fixtures)
	if err != nil {
		t.Fatal(err)
	}
	afterFixtures, err := json.Marshal(deployment.Fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Definition.Constraints, deployment.Definition.Constraints) ||
		string(beforeFixtures) != string(afterFixtures) {
		t.Fatal("forward deployment changed lifecycle constraints or desired fixtures")
	}
}
