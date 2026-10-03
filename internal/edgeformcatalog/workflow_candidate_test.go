package edgeformcatalog

import (
	"reflect"
	"slices"
	"strings"
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

func TestWorkflowCandidateSpecifiesLifecycleAndBoundedSignalSemantics(t *testing.T) {
	t.Parallel()
	candidate, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	definition := WorkflowCandidateInterface()
	if definition.Semantics.Ordering != "per_key" {
		t.Fatalf("workflow event ordering = %q, want per_key", definition.Semantics.Ordering)
	}
	if got := definition.Limits["maxPendingEventCount"]; got != 1024 {
		t.Fatalf("maxPendingEventCount = %d, want 1024", got)
	}
	if got := definition.Limits["maxPendingEventBytes"]; got != 1_048_576 {
		t.Fatalf("maxPendingEventBytes = %d, want 1048576", got)
	}

	operations := make(map[string]InterfaceOperation, len(definition.Operations))
	for _, operation := range definition.Operations {
		operations[operation.Name] = operation
	}
	sendEvent := operations["sendEvent"]
	if !slices.Contains(sendEvent.Errors, "event_queue_full") {
		t.Fatal("sendEvent does not declare event_queue_full")
	}
	for _, phrase := range []string{
		"per-instance FIFO",
		"without durable acceptance",
	} {
		if !strings.Contains(sendEvent.Description, phrase) {
			t.Errorf("sendEvent description is missing %q", phrase)
		}
	}
	waitForEvent := operations["stepWaitForEvent"]
	for _, phrase := range []string{
		"oldest accepted event of the matching type",
		"same per-instance acceptance order",
		"equal deadline",
	} {
		if !strings.Contains(waitForEvent.Description, phrase) {
			t.Errorf("stepWaitForEvent description is missing %q", phrase)
		}
	}
	terminate := operations["terminate"]
	for _, phrase := range []string{
		"Fence the current execution owner",
		"physically stopped",
		"before terminated is visible",
	} {
		if !strings.Contains(terminate.Description, phrase) {
			t.Errorf("terminate description is missing %q", phrase)
		}
	}

	for _, phrase := range []string{
		"exact WorkerVersion for each context",
		"history is never migrated",
		"incompatible deployment/workflow remains not Ready",
		"resource_busy",
		"DELETE is refused before mutation",
		"all instances are terminal",
		"all execution owners are stopped and fenced",
		"successful delete purges retained terminal history and queued events",
		"delete/recreate receives a new Host UID",
	} {
		if !strings.Contains(candidate.Form.Definition.Description, phrase) {
			t.Errorf("DurableWorkflow Form description is missing %q", phrase)
		}
	}

	var orderedEvents *InterfaceFixture
	for index := range definition.Fixtures {
		if definition.Fixtures[index].Name == "queued-events-consume-oldest-match" {
			orderedEvents = &definition.Fixtures[index]
			break
		}
	}
	if orderedEvents == nil {
		t.Fatal("workflow Interface is missing the queued event ordering fixture")
	}
	if len(orderedEvents.Steps) < 6 {
		t.Fatalf("queued event ordering fixture has %d steps, want at least 6", len(orderedEvents.Steps))
	}

	aggregate := RuntimeWorkflowCandidateInterface()
	if aggregate.Name != RuntimeWorkflowCandidateInterfaceName ||
		aggregate.Version != RuntimeWorkflowCandidateInterfaceVersion {
		t.Fatalf("joint Workflow identity = %s@%s", aggregate.Name, aggregate.Version)
	}
	if aggregate.Semantics.Ordering != "per_key" ||
		aggregate.Limits["maxPendingEventCount"] != 1024 ||
		aggregate.Limits["maxPendingEventBytes"] != 1_048_576 {
		t.Fatal("joint Workflow candidate lost adopted event queue semantics")
	}
	var aggregateSendEvent, aggregateWaitForEvent InterfaceOperation
	for _, operation := range aggregate.Operations {
		switch operation.Name {
		case "sendEvent":
			aggregateSendEvent = operation
		case "stepWaitForEvent":
			aggregateWaitForEvent = operation
		}
	}
	if !slices.Contains(aggregateSendEvent.Errors, "event_queue_full") ||
		!strings.Contains(aggregateWaitForEvent.Description, "same per-instance acceptance order") {
		t.Fatal("joint Workflow candidate lost bounded FIFO signal semantics")
	}
}

func TestWorkflowCandidateTreatsActiveExecutionsAsDeleteDependencies(t *testing.T) {
	t.Parallel()

	candidate, err := RenderWorkflowCandidate()
	if err != nil {
		t.Fatal(err)
	}
	description := candidate.Form.Definition.Description
	for _, phrase := range []string{
		"live Workflow Binding remains a dependency_in_use (409)",
		"active execution identity owned by this Workflow UID remains a dependency_in_use (409)",
		"failure detail identifying that Binding",
		"failure detail identifies the active execution identity",
		"resource_busy is reserved for bounded transient concurrent mutation or index maintenance",
	} {
		if !strings.Contains(description, phrase) {
			t.Errorf("DurableWorkflow delete contract is missing %q", phrase)
		}
	}
	if strings.Contains(description, "resource_busy while any instance") {
		t.Fatal("long-lived active Workflow execution is incorrectly classified as retryable resource_busy")
	}
}
