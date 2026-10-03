package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func TestRenderSourceSelectsUnreleasedJointVersions(t *testing.T) {
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	if document.PublicationStatus != "unpublished" || len(document.Families) != 1 {
		t.Fatalf("source status/families = %s/%d", document.PublicationStatus, len(document.Families))
	}
	var forms []struct {
		Kind       string `json:"kind"`
		Definition struct {
			DefinitionVersion string `json:"definitionVersion"`
		} `json:"definition"`
	}
	if err := json.Unmarshal(document.Families[0].Forms, &forms); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ModuleWorker":       edgeformcatalog.ActorWorkflowSourceFormVersion,
		"ActorNamespace":     edgeformcatalog.ActorWorkflowSourceFormVersion,
		"DurableWorkflow":    edgeformcatalog.ActorWorkflowSourceFormVersion,
		"WorkerCustomDomain": edgeformcatalog.ActorWorkflowSourceFormVersion,
		"WorkerEndpoint":     edgeformcatalog.ActorWorkflowSourceFormVersion,
		"WorkerCronTrigger":  edgeformcatalog.ActorWorkflowSourceFormVersion,
		"QueueConsumer":      edgeformcatalog.ActorWorkflowSourceFormVersion,
		"WorkerVersion":      edgeformcatalog.ActorWorkflowSourceWorkerVersion,
		"WorkerDeployment":   edgeformcatalog.ActorWorkflowSourceWorkerDeployment,
	}
	for _, form := range forms {
		if version, changed := want[form.Kind]; changed {
			if form.Definition.DefinitionVersion != version {
				t.Fatalf("%s source version = %s, want %s", form.Kind, form.Definition.DefinitionVersion, version)
			}
			delete(want, form.Kind)
		}
	}
	if len(want) != 0 {
		t.Fatalf("source omitted selected Forms: %+v", want)
	}
	wantContracts := map[string]string{
		"worker.runtime": "2.0.0", "worker.actor": "2.0.0", "worker.workflow": "3.0.0",
		"module-worker.actor": "2.0.0", "module-worker.workflow": "3.0.0",
	}
	for _, contract := range append(append([]sourceContract(nil), document.Interfaces...), document.Bindings...) {
		if version, changed := wantContracts[contract.Name]; changed {
			if contract.Version != version {
				t.Fatalf("%s source version = %s, want %s", contract.Name, contract.Version, version)
			}
			delete(wantContracts, contract.Name)
		}
	}
	if len(wantContracts) != 0 {
		t.Fatalf("source omitted selected contracts: %+v", wantContracts)
	}
}

func TestRenderSourceEmitsOnlyTheCurrentEdgeFamily(t *testing.T) {
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	wantGroups := []string{"edge.forms.takoform.com"}
	wantCounts := []int{17}
	if len(document.Families) != len(wantGroups) {
		t.Fatalf("families = %d, want %d", len(document.Families), len(wantGroups))
	}
	for index, family := range document.Families {
		if family.Group != wantGroups[index] {
			t.Fatalf("family[%d] = %q, want %q", index, family.Group, wantGroups[index])
		}
		var forms []struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(family.Forms, &forms); err != nil {
			t.Fatalf("decode %s Forms: %v", family.Group, err)
		}
		if len(forms) != wantCounts[index] {
			t.Fatalf("%s Forms = %d, want %d", family.Group, len(forms), wantCounts[index])
		}
		foundObjectBucket := false
		for _, form := range forms {
			if form.Kind == "ObjectBucket" {
				foundObjectBucket = true
				break
			}
		}
		if !foundObjectBucket {
			t.Fatal("current aggregate omits ObjectBucket")
		}
	}
}

func TestRenderSourceKeepsSourceOnlyFormsOutsideTheCurrentFamilyIndexProjection(t *testing.T) {
	t.Parallel()
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Families) != 1 {
		t.Fatalf("current families = %d, want unchanged single current family", len(document.Families))
	}
	if len(document.SourceOnlyForms) != 1 {
		t.Fatalf("source-only family projections = %d, want 1", len(document.SourceOnlyForms))
	}
	source := document.SourceOnlyForms[0]
	if source.Group != "edge.forms.takoform.com" || source.PublicationStatus != "UNPUBLISHED" {
		t.Fatalf("source-only metadata = %+v", source)
	}
	var forms []struct {
		Kind       string `json:"kind"`
		Slug       string `json:"slug"`
		Definition struct {
			LifecycleCapabilities []string `json:"lifecycleCapabilities"`
		} `json:"definition"`
	}
	if err := json.Unmarshal(source.Forms, &forms); err != nil {
		t.Fatalf("decode source-only Forms: %v", err)
	}
	if len(forms) != 2 || forms[0].Kind != "ContainerService" || forms[1].Kind != "ContainerEndpoint" {
		t.Fatalf("source-only Forms = %+v, want exactly the Container pair", forms)
	}
	for _, form := range forms {
		if form.Slug == "" {
			t.Errorf("source-only %s has no package slug", form.Kind)
		}
		if containsString(form.Definition.LifecycleCapabilities, "import") {
			t.Errorf("source-only %s advertises import: %v", form.Kind, form.Definition.LifecycleCapabilities)
		}
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestRenderSourceBuildsGlobalProviderNeutralContracts(t *testing.T) {
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Interfaces) != 8 {
		t.Fatalf("interfaces = %d, want 8", len(document.Interfaces))
	}
	if len(document.Bindings) != 7 {
		t.Fatalf("bindings = %d, want 7", len(document.Bindings))
	}
	seen := map[string]struct{}{}
	foundObjectInterface := false
	foundObjectBinding := false
	for _, contract := range append(append([]sourceContract(nil), document.Interfaces...), document.Bindings...) {
		if _, duplicate := seen[contract.Name]; duplicate {
			t.Fatalf("duplicate global contract %q", contract.Name)
		}
		seen[contract.Name] = struct{}{}
		if contract.SchemaDigest == "" || contract.DefinitionJSON == "" {
			t.Fatalf("contract %q has no exact identity", contract.Name)
		}
		if contract.Name == "edge.objects" {
			foundObjectInterface = true
		}
		if strings.Contains(contract.Name, "object-bucket") {
			foundObjectBinding = true
		}
	}
	if !foundObjectInterface || !foundObjectBinding {
		t.Fatalf("current aggregate omits ObjectBucket contracts: interface=%v binding=%v", foundObjectInterface, foundObjectBinding)
	}
}

func TestSortExactContractsAllowsVersionsAndRejectsExactDuplicate(t *testing.T) {
	t.Parallel()
	contracts := []sourceContract{
		{Name: "example.contract", Version: "2.0.0"},
		{Name: "example.contract", Version: "1.0.0"},
		{Name: "another.contract", Version: "1.0.0"},
	}
	if err := sortExactContracts(contracts); err != nil {
		t.Fatal(err)
	}
	if contracts[0].Name != "another.contract" || contracts[1].Version != "1.0.0" || contracts[2].Version != "2.0.0" {
		t.Fatalf("exact contracts are not deterministically sorted: %#v", contracts)
	}
	contracts = append(contracts, sourceContract{Name: "example.contract", Version: "2.0.0"})
	if err := sortExactContracts(contracts); err == nil || !strings.Contains(err.Error(), "example.contract@2.0.0") {
		t.Fatalf("exact duplicate error = %v", err)
	}
}
