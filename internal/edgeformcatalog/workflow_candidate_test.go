package edgeformcatalog

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tako0614/takoform/formpackage"
)

func TestWorkflowCandidatePairPinsForwardInterfaceAndPreservesRuntime(t *testing.T) {
	t.Parallel()
	candidate, err := renderWorkflowCandidatePair()
	if err != nil {
		t.Fatal(err)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(candidate.Interface.DefinitionJSON)); err != nil {
		t.Fatal(err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.Form.DefinitionJSON)); err != nil {
		t.Fatal(err)
	}
	refs := candidate.Form.Definition.ProvidedInterfaces
	if len(refs) != 1 || refs[0].Name != WorkflowCandidateInterfaceName ||
		refs[0].Version != WorkflowCandidateInterfaceVersion || refs[0].SchemaDigest != candidate.Interface.SchemaDigest {
		t.Fatalf("forward workflow InterfaceRef = %#v", refs)
	}
	if candidate.Form.Definition.DefinitionVersion != WorkflowCandidateFormVersion {
		t.Fatalf("forward Form version = %q", candidate.Form.Definition.DefinitionVersion)
	}
	current, ok := ByKind("DurableWorkflow")
	if !ok {
		t.Fatal("current DurableWorkflow disappeared")
	}
	old, err := renderForm(current, newTargetContractResolver())
	if err != nil {
		t.Fatal(err)
	}
	oldProperties := old.Definition.DesiredSchema["properties"].(map[string]any)
	newProperties := candidate.Form.Definition.DesiredSchema["properties"].(map[string]any)
	if !reflect.DeepEqual(oldProperties["worker"], newProperties["worker"]) {
		t.Fatal("forward workflow changed the ModuleWorker/runtime target")
	}
	if !reflect.DeepEqual(old.Definition.LifecycleCapabilities, candidate.Form.Definition.LifecycleCapabilities) {
		t.Fatal("forward workflow changed resource lifecycle capabilities")
	}
}

func TestWorkflowCandidateHasOneConsistentForwardReasonSet(t *testing.T) {
	t.Parallel()
	definition := WorkflowCandidateInterface()
	want := workflowCandidateErrorReasonStrings()
	for _, operation := range definition.Operations {
		switch operation.Name {
		case "run":
			if !reflect.DeepEqual(operation.Errors, want) {
				t.Fatalf("run reasons = %#v", operation.Errors)
			}
		case "status":
			properties := operation.OutputSchema["properties"].(map[string]any)
			errorProperties := properties["error"].(map[string]any)["properties"].(map[string]any)
			reasons := errorProperties["reason"].(map[string]any)["enum"].([]any)
			if len(reasons) != len(want) {
				t.Fatalf("status reasons = %#v", reasons)
			}
			for i, reason := range want {
				if reasons[i] != reason {
					t.Fatalf("status reason[%d] = %#v, want %q", i, reasons[i], reason)
				}
			}
		case "stepDo", "stepSleep", "stepWaitForEvent":
			for _, code := range []string{"step_failed", "wait_timeout", "step_definition_mismatch"} {
				if !slices.Contains(operation.Errors, code) {
					t.Errorf("%s cannot replay %s", operation.Name, code)
				}
			}
			if slices.Contains(operation.Errors, "document_too_large") {
				t.Errorf("%s retained obsolete separate result-admission error", operation.Name)
			}
		case "create", "sendEvent":
			if !slices.Contains(operation.Errors, "document_too_large") {
				t.Errorf("%s lost consumer document admission error", operation.Name)
			}
		}
	}
	if slices.Contains(workflowErrorReasons, any("step_definition_mismatch")) {
		t.Fatal("forward reason leaked into the registered v1 contract")
	}
}
