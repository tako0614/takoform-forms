package edgeformcatalog

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
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

func TestWorkflowCandidateEventIdSchemaAndSemanticFixtures(t *testing.T) {
	t.Parallel()
	definition := WorkflowCandidateInterface()
	if definition.Limits["maxPendingEventsPerInstance"] != workflowCandidateMaxPendingEvents {
		t.Fatal("pending-event bound is not declared on the Interface")
	}
	var send InterfaceOperation
	for _, operation := range definition.Operations {
		if operation.Name == "sendEvent" {
			send = operation
			break
		}
	}
	if send.Name == "" || !slices.Contains(send.Errors, "event_conflict") || !slices.Contains(send.Errors, "event_queue_full") {
		t.Fatalf("event admission outcomes = %#v", send.Errors)
	}
	raw, err := json.Marshal(send.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	const id = "https://candidate.test/workflow-send-event.json"
	if err := compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		input map[string]any
		valid bool
	}{
		{"keyed", map[string]any{"id": "i", "type": "approval", "eventId": "send-1"}, true},
		{"unkeyed", map[string]any{"id": "i", "type": "approval"}, true},
		{"empty key", map[string]any{"id": "i", "type": "approval", "eventId": ""}, false},
		{"numeric key", map[string]any{"id": "i", "type": "approval", "eventId": 1}, false},
		{"unknown option", map[string]any{"id": "i", "type": "approval", "dedupe": true}, false},
	} {
		err := schema.Validate(tc.input)
		if (err == nil) != tc.valid {
			t.Errorf("%s: valid=%v, error=%v", tc.name, tc.valid, err)
		}
	}
	fixtures := map[string]InterfaceFixture{}
	for _, fixture := range definition.Fixtures {
		fixtures[fixture.Name] = fixture
	}
	for _, name := range []string{"event-key-retry-is-one-admission", "keyed-event-retry-after-terminal"} {
		fixture, ok := fixtures[name]
		if !ok || len(fixture.Steps) < 4 {
			t.Errorf("missing keyed event semantic trace %q", name)
		}
	}
	for _, phrase := range []string{"stale context cannot commit", "Deduplicate before checking terminal", "No continuation may commit"} {
		if !strings.Contains(definition.Description+send.Description+operationDescription(definition, "terminate"), phrase) {
			t.Errorf("missing owner/fence semantic %q", phrase)
		}
	}
}

func operationDescription(definition InterfaceDefinition, name string) string {
	for _, operation := range definition.Operations {
		if operation.Name == name {
			return operation.Description
		}
	}
	return ""
}
