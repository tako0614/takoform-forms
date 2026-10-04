package edgeformcatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tako0614/takoform/formpackage"
	coresnapshot "github.com/tako0614/takoform/snapshot"
)

// This is the stdout digest of cmd/actor-workflow-candidate at clean source
// commit 1c7337c0 (the qualified 0025cf8 tree). It locks the preselection
// development candidate independently of the new source-only renderer.
const legacyActorWorkflowCandidateStdoutSHA256 = "6fbc72d537c49e5ab09e956b7d046bb38e35c3f04d17c15504b7bc308c3bd81b"

func TestActorWorkflowDevelopmentCandidatePreservesPreselectionStdoutBytes(t *testing.T) {
	candidate, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	encoder := json.NewEncoder(&stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		t.Fatal(err)
	}
	got := sha256.Sum256(stdout.Bytes())
	if gotSHA := fmt.Sprintf("%x", got); gotSHA != legacyActorWorkflowCandidateStdoutSHA256 {
		t.Fatalf("preselection candidate stdout SHA-256 = %s, want %s", gotSHA, legacyActorWorkflowCandidateStdoutSHA256)
	}
}

func TestActorWorkflowCandidateHasOneWorkerPairAndNoVectorBinding(t *testing.T) {
	candidate, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if candidate.WorkerVersion.Kind != "WorkerVersion" || candidate.WorkerDeployment.Kind != "WorkerDeployment" {
		t.Fatalf("joint worker pair = %s/%s", candidate.WorkerVersion.Kind, candidate.WorkerDeployment.Kind)
	}
	refs := candidate.WorkerVersion.Definition.AcceptedBindings
	if len(refs) != 7 {
		t.Fatalf("joint WorkerVersion has %d Bindings, want the existing seven with Actor and Workflow retargeted", len(refs))
	}
	if countBindingName(refs, VectorIndexCandidateBindingName) != 0 {
		t.Fatal("unadopted Vector Binding entered Actor+Workflow successor")
	}
	for name, contract := range map[string]RenderedContract{
		ActorCandidateBindingName:    candidate.Actor.Binding,
		WorkflowCandidateBindingName: candidate.Workflow.Binding,
	} {
		if countBindingName(refs, name) != 1 {
			t.Fatalf("joint WorkerVersion has %d %s BindingRefs, want one", countBindingName(refs, name), name)
		}
		for _, ref := range refs {
			if ref.Name == name && (ref.Version != contract.Version || ref.SchemaDigest != contract.SchemaDigest) {
				t.Fatalf("%s BindingRef = %+v, want exact %s@%s %s", name, ref, contract.Name, contract.Version, contract.SchemaDigest)
			}
		}
	}
}

func TestActorWorkflowCandidateIsDeterministicAndLeavesCurrentCatalogUntouched(t *testing.T) {
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
	first, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("joint source candidate changed bytes across repeated renders")
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
		t.Fatal("joint candidate rendering changed the selected current catalog")
	}
}

func TestActorWorkflowCandidateRejectsDuplicateAndStaleBindingRefs(t *testing.T) {
	for _, mutation := range []struct {
		name string
		edit func(*ActorWorkflowCandidate)
	}{
		{name: "duplicate actor", edit: func(candidate *ActorWorkflowCandidate) {
			refs := candidate.WorkerVersion.Definition.AcceptedBindings
			candidate.WorkerVersion.Definition.AcceptedBindings = append(refs, refs[len(refs)-1])
		}},
		{name: "stale actor", edit: func(candidate *ActorWorkflowCandidate) {
			for index := range candidate.WorkerVersion.Definition.AcceptedBindings {
				if candidate.WorkerVersion.Definition.AcceptedBindings[index].Name == ActorCandidateBindingName {
					candidate.WorkerVersion.Definition.AcceptedBindings[index].SchemaDigest = "sha256:" + strings.Repeat("0", 64)
				}
			}
		}},
		{name: "replaced retained binding", edit: func(candidate *ActorWorkflowCandidate) {
			candidate.WorkerVersion.Definition.AcceptedBindings[0].Name = "module-worker.rogue"
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			candidate, err := RenderActorWorkflowCandidate()
			if err != nil {
				t.Fatal(err)
			}
			mutation.edit(&candidate)
			if err := validateActorWorkflowCandidate(candidate); err == nil {
				t.Fatal("duplicate or stale BindingRef passed the joint closure check")
			}
		})
	}
}

func TestActorWorkflowCandidateRejectsOldRuntimeReference(t *testing.T) {
	candidate, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	properties := candidate.RuntimeDependants[0].Definition.DesiredSchema["properties"].(map[string]any)
	worker := properties["worker"].(map[string]any)
	worker["x-takoform-required-interface"].(map[string]any)["version"] = "1.1.0"
	if err := validateActorWorkflowCandidate(candidate); err == nil || !strings.Contains(err.Error(), "stale worker.runtime") {
		t.Fatalf("old runtime annotation was accepted or returned wrong error: %v", err)
	}
}

func TestActorWorkflowCandidateRejectsWrongDeploymentTarget(t *testing.T) {
	candidate, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	properties := candidate.WorkerDeployment.Definition.DesiredSchema["properties"].(map[string]any)
	worker := properties["worker"].(map[string]any)
	refs := worker["x-takoform-target-formrefs"].([]any)
	refs[0].(map[string]any)["definitionVersion"] = "0.1.0"
	if err := validateActorWorkflowCandidate(candidate); err == nil || !strings.Contains(err.Error(), "deployment target") {
		t.Fatalf("stale ModuleWorker target was accepted or returned wrong error: %v", err)
	}
}

func TestActorWorkflowCandidateRejectsMissingActorAndUnwantedVectorFields(t *testing.T) {
	for _, mutation := range []struct {
		name string
		edit func(map[string]any)
	}{
		{name: "missing actor", edit: func(properties map[string]any) { delete(properties, "actorBindings") }},
		{name: "unwanted vector", edit: func(properties map[string]any) { properties["vectorBindings"] = map[string]any{"type": "array"} }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			candidate, err := RenderActorWorkflowCandidate()
			if err != nil {
				t.Fatal(err)
			}
			properties := candidate.WorkerVersion.Definition.DesiredSchema["properties"].(map[string]any)
			mutation.edit(properties)
			if err := validateActorWorkflowCandidate(candidate); err == nil || !strings.Contains(err.Error(), "WorkerVersion binding fields") {
				t.Fatalf("mutated WorkerVersion binding fields were accepted or returned wrong error: %v", err)
			}
		})
	}
}

func TestActorWorkflowCandidateCarriesAdoptedWorkflowDeleteAndUpdateBoundary(t *testing.T) {
	candidate, err := RenderActorWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	description := candidate.Workflow.Form.Definition.Description
	for _, required := range []string{
		"dependency_in_use", "queued/running/sleeping/waiting", "dependent execution identities",
		"failure detail identifies whether a live Binding or an active execution identity",
		"resource_busy is reserved for bounded transient",
		"2592000", "new Host UID", "WorkerDeployment", "step-history compatible",
	} {
		if !strings.Contains(description, required) {
			t.Fatalf("joint DurableWorkflow Form omitted %q from adopted lifecycle contract", required)
		}
	}
	if strings.Contains(description, "returns resource_busy") {
		t.Fatal("long-lived active Workflow DELETE refusal must not use Host API v1 automatically retryable resource_busy")
	}
	for _, capability := range candidate.Workflow.Form.Definition.LifecycleCapabilities {
		if capability == "update" {
			t.Fatal("joint DurableWorkflow identity silently acquired update capability")
		}
	}
}

func TestActorWorkflowCandidateCompilesOneExactCoreSnapshotWithoutPromotingVector(t *testing.T) {
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	current, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Forms) != len(current) || len(selected.Forms) != 17 {
		t.Fatalf("selected Forms = %d, want exact replacement roster of 17", len(selected.Forms))
	}
	input := coresnapshot.Input{HostAPI: "forms.takoform.com/v1"}
	for index, form := range selected.Forms {
		var pkg formpackage.VerifiedPackage
		if reflect.DeepEqual(form, current[index]) {
			root := filepath.Join("..", "..", "forms", "candidates", Family.APIVersion(), form.Slug)
			report, err := formpackage.VerifyDirectory(root)
			if err != nil {
				t.Fatalf("retained %s package: %v", form.Kind, err)
			}
			verified, ok := report.VerifiedPackage()
			if !ok {
				t.Fatalf("retained %s issued no Core package", form.Kind)
			}
			pkg = verified
		} else {
			pkg = verifySourceOnlyPackage(t, form)
		}
		input.Packages = append(input.Packages, coresnapshot.PackageArtifact{
			Origin: form.Kind, ExpectedDigest: pkg.PackageDigest(), Package: pkg,
		})
		ref, err := renderedFormRef(form)
		if err != nil {
			t.Fatal(err)
		}
		input.DefaultCreates = append(input.DefaultCreates, coresnapshot.DefaultPin{
			Group: Family.APIVersion(), Kind: form.Kind,
			Ref: formpackage.FormRef{
				APIVersion: ref.APIVersion, Kind: ref.Kind,
				DefinitionVersion: ref.DefinitionVersion, SchemaDigest: ref.SchemaDigest,
			},
		})
	}
	for _, contract := range selected.Interfaces {
		input.Interfaces = append(input.Interfaces, coresnapshot.InterfaceArtifact{
			Origin:         contract.Name + "@" + contract.Version,
			ExpectedDigest: contract.SchemaDigest, Definition: []byte(contract.DefinitionJSON),
		})
	}
	for _, contract := range selected.Bindings {
		input.Bindings = append(input.Bindings, coresnapshot.BindingArtifact{
			Origin:         contract.Name + "@" + contract.Version,
			ExpectedDigest: contract.SchemaDigest, Definition: []byte(contract.DefinitionJSON),
		})
	}
	compiled, diagnostics := coresnapshot.Compile(input)
	if compiled == nil || len(diagnostics) != 0 {
		t.Fatalf("Core did not close Actor+Workflow successor: snapshot=%v diagnostics=%+v", compiled, diagnostics)
	}
	if got := len(compiled.Forms()); got != 17 {
		t.Fatalf("Core Snapshot has %d Forms, want exact selected 17", got)
	}
	for _, form := range compiled.Forms() {
		if form.Ref.Kind == "VectorIndex" {
			t.Fatal("selected Actor+Workflow snapshot promoted the unrelated Vector candidate")
		}
	}
}
