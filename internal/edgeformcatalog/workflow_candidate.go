package edgeformcatalog

import (
	"fmt"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// WorkflowCandidate is the forward Workflow closure. Rendering alone does not
// create a package or advertise Provider/Host support.
type WorkflowCandidate struct {
	Form             RenderedForm     `json:"form"`
	Interface        RenderedContract `json:"interface"`
	Binding          RenderedContract `json:"binding"`
	WorkerVersion    RenderedForm     `json:"workerVersion"`
	WorkerDeployment RenderedForm     `json:"workerDeployment"`
}

const (
	WorkflowCandidateInterfaceName    = "worker.workflow"
	WorkflowCandidateInterfaceVersion = "2.0.0"
	WorkflowCandidateFormVersion      = "0.2.0"
	workflowCandidateMaxPendingEvents = 1024
)

// Existing worker.workflow@1.0.0 and all of its exact closures remain unchanged.
func WorkflowCandidateInterface() InterfaceDefinition {
	definition := workerWorkflowInterface()
	definition.Version = WorkflowCandidateInterfaceVersion
	definition.Title = "Portable durable workflow class execution"
	definition.Semantics.Ordering = "per_key"
	definition.Limits["maxPendingEventsPerInstance"] = workflowCandidateMaxPendingEvents
	definition.Description = "Portable worker.workflow class ABI; supplements worker.runtime@1.1.0 without " +
		"changing its plain-object default export, handlers, globals or environment. All documents " +
		"use create's data-only rules; undefined is absent only for an entire optional document. Instance " +
		"status is exactly queued, running, sleeping, waiting, complete, errored or terminated. Instances are " +
		"runtime data, not Resources. Retained ids cannot be reused; terminal records and ids remain retained " +
		"for 2592000 seconds.\n\nEach fresh context selects the deployment's THEN-CURRENT weighted " +
		"WorkerVersion and pins it for that context. Replay rebuilds context and class; fields, " +
		"closures, module state and Promises do not survive. Step effects are at-least-once with " +
		"memoized outcomes; an uncommitted effect may repeat after process death. Callbacks must be " +
		"idempotent; code outside steps, including module evaluation and " +
		"constructors, must be side-effect-free and deterministic against history. Deployment changes must " +
		"remain compatible with in-flight histories.\n\nThe " +
		"Host stops code before publishing sleeping, waiting or any terminal state; no context remains " +
		"parked. Nothing runs after the absolute 31536000-second lifetime cutoff. Bounds and infrastructure " +
		"interruptions are Host control, never catchable app sentinels. Step success is durably " +
		"saved before resolution; first resolution and replay " +
		"return decoded persisted clones, not the original result object. Invalid/oversized callback results " +
		"are failed attempts under the saved retry policy; invalid/oversized final run output is " +
		"run_threw.\n\nstep is bound to this context and instance; the app never passes instanceId. Steps are " +
		"sequential. A second overlapping call, ANY run settlement while a step is unsettled, or incomplete " +
		"cross-kind name reuse causes uncatchable step_definition_mismatch, ahead of run_threw. Stop precedes " +
		"terminal publication; no mismatch rejection reaches app catch/finally. Its operation-error listing " +
		"is an outcome, not a catchable Error. Finished names, including failed ones, replay by name " +
		"regardless of new kind, without new effects; sleep discards a recorded success value. Validate the " +
		"name before finished-history lookup, then ignore unused arguments for finished names. For a " +
		"new/pending name, validate the listed argument positions and closed option schemas before new " +
		"writes/effects; invalid JS calls reject with TypeError. An omitted optional argument may also be " +
		"undefined. Pending same-kind calls use their first saved configuration, not valid new options. " +
		"Names/types contain 1-256 Unicode scalar values. All state changes are ordered per " +
		"(DurableWorkflow resource, instance id); matching events use acceptance order.\n\nThe Host owns the instance journal, " +
		"event inbox, wake scheduling and execution fence. One owner may run an instance at a time; every " +
		"commit checks its current fence, and a stale context cannot commit, use a Host-mediated effect, " +
		"or publish state. External effects remain at-least-once and require application idempotency. " +
		"Wakes/retries may repeat but acquire one fenced owner before code runs. " +
		"State changes and event consumption are atomic and visible to status only after they commit. " +
		"A terminated, completed, errored or expired instance cannot be resumed. The Host must retire the " +
		"old execution context before exposing a terminal state or handing the instance to a new owner.\n\n" +
		"Host step Errors have an own, non-enumerable, " +
		"non-writable, non-configurable name equal to the exact code. Provenance is private and bound to the " +
		"exact object and context, not an app property. Rethrowing the same object preserves provenance; " +
		"copying names or wrapping errors does not. Other fields may be annotated. Only an uncaught error " +
		"privately branded step_failed promotes a run failure to terminal step_failed. All other uncaught " +
		"errors, including branded wait_timeout and TypeError, are run_threw. Handling a step failure may " +
		"still lead to complete."

	for i := range definition.Operations {
		operation := &definition.Operations[i]
		switch operation.Name {
		case "create":
			operation.Description = "Start one instance; id is optional and scoped to this DurableWorkflow. " +
				"Commit queued state before replying; a retained id fails instance_exists. A minted id is not a " +
				"safe retry key; choose a stable id when retrying an uncertain create. No serving deployment " +
				"fails unsupported_capability. All documents (params, step results, event payloads, run output) " +
				"have a plain or null-prototype object root with at most 1024 properties and 1048576 RFC 8785 " +
				"canonical UTF-8 bytes. Nested values are null, booleans, finite numbers, Unicode-scalar " +
				"strings, such objects, or dense ordinary arrays. Refuse accessors, symbols, array holes or " +
				"extra properties, other prototypes, cycles, functions, BigInt, and nested undefined. Shared " +
				"acyclic objects are allowed. Inspect descriptors; do not invoke getters or toJSON. Invalid " +
				"params fail invalid_params; byte overflow fails document_too_large."
		case "status":
			operation.Description = "Read a retained instance. Output occurs only on complete and error only on " +
				"errored. The closed error reasons are run_threw, step_failed, step_limit_exceeded, lifetime_exceeded " +
				"and step_definition_mismatch. Workflow failure is a successful status read, not an operation rejection. " +
				"Read a committed snapshot; never expose a parked or terminal state before its execution context is retired."
			operation.OutputSchema = withDialect(workflowCandidateInstanceStatus())
		case "sendEvent":
			operation.Description = "JS: instance.sendEvent({type, payload?, eventId?}) durably admits one event " +
				"before resolving. eventId is an optional instance-scoped idempotency key. Retain its first " +
				"accepted type and canonical payload fingerprint after consumption, until the record expires. " +
				"Repeating the same key and canonical event succeeds without enqueueing, including after terminal; " +
				"different content under the key fails event_conflict. Deduplicate before checking terminal. " +
				"Retain type and canonical payload digest, not the consumed payload, for keyed deduplication. " +
				"Unkeyed retries may duplicate. Consume matches in acceptance order. Retain at most 1024 " +
				"unconsumed events per instance; " +
				"a new event at that bound fails event_queue_full without acceptance. A new event after terminal " +
				"fails instance_terminal. Admission and terminal transition serialize. Transport/backend " +
				"interruption leaves acceptance unknown; keyed retry settles it. Keys are retained only " +
				"for this instance record; no retry guarantee survives its expiry or id reuse."
			properties := operation.InputSchema["properties"].(map[string]any)
			properties["eventId"] = stringSchema(1, workflowMaxNameBytes)
			operation.Errors = []string{"unknown_instance", "instance_terminal", "event_conflict", "event_queue_full", "document_too_large", "backend_unavailable"}
		case "terminate":
			operation.Description = "JS: instance.terminate() stops an active instance, fences and retires its " +
				"execution context, then commits terminated before resolving. It discards pending events and " +
				"wakes. If already complete, errored or terminated, it succeeds without changing that terminal " +
				"status. Concurrent completion and termination serialize; exactly one terminal outcome is visible. " +
				"No continuation may commit an effect or state after terminal publication."
		case "run":
			operation.Description = "The main module's className named export must be constructible with a callable prototype run in " +
				"every weighted version before Ready. No vendor base class or native context is required. Per fresh " +
				"context use ordinary ECMAScript instance = new Export(env), then resolve instance.run and call it " +
				"once with this=instance and (event, step). env is exactly that selected version's resolved " +
				"worker.runtime@1.1.0 environment, never Host-private controls. event has exactly instanceId and " +
				"params only if supplied at create; mutations are not durable. run returns a data-only object or " +
				"undefined, directly or by Promise, not {output}. The schema wrapper is an observation only. " +
				"Construction, run lookup/call, missing/non-callable execution-time run and uncaught failures yield " +
				"run_threw, except genuine step_failed provenance and Host-controlled outcomes described above. Save " +
				"a valid output before complete."
			operation.Errors = workflowCandidateErrorReasonStrings()
		case "stepDo":
			operation.Description = "JS: step.do(name, effect, retryPolicy?) returns Promise<object|undefined>, not {result}. effect is a " +
				"zero-argument callable. Save normalized policy before the first callback: omission means " +
				"maxAttempts=1; supplied policy requires maxAttempts (1-100 including first). initialDelaySeconds " +
				"defaults to 0, backoff to constant, maxDelaySeconds to 43200; delays are integers 0-43200. After " +
				"failed attempt k (one-based), delay is min(maxDelaySeconds,initialDelaySeconds) for constant or " +
				"min(maxDelaySeconds,initialDelaySeconds*2^(k-1)) for exponential, capped without overflow. Every " +
				"attempt uses the saved policy and calls effect once. Throw, rejection or invalid/oversized result " +
				"fails that attempt. If retry remains, persist progress/due time, stop and replay in a fresh context " +
				"not earlier than due, even for zero delay. Otherwise journal step_failed before rejecting with that " +
				"Host Error. Commit success before resolving. Finished-name replay follows the common rules."
			operation.Errors = []string{"step_failed", "wait_timeout", "step_definition_mismatch", "step_limit_exceeded", "lifetime_exceeded", "backend_unavailable"}
		case "stepSleep":
			operation.Description = "JS: step.sleep(name, seconds) returns Promise<void>. seconds is an integer 0-maxSleepSeconds. First " +
				"registration saves duration and due time; replay keeps those values. A future sleep stops this " +
				"context before sleeping becomes visible; its Promise never resolves there. At or after due time, a " +
				"new context replays and durably completes the step before resolving undefined. A zero sleep commits " +
				"and resolves in the current context. A sleep beyond the absolute lifetime stops with " +
				"lifetime_exceeded, never a shortened sleep. Finished-name replay discards a recorded success value " +
				"or rethrows its recorded step Error."
			operation.Errors = []string{"step_failed", "wait_timeout", "step_definition_mismatch", "step_limit_exceeded", "lifetime_exceeded", "backend_unavailable"}
		case "stepWaitForEvent":
			operation.Description = "JS: step.waitForEvent(name, {type, timeoutSeconds}) returns Promise<object|undefined>, not " +
				"{payload}. type and integer timeoutSeconds (1-maxWaitTimeoutSeconds) are required. First " +
				"registration fixes type and absolute timeout. Atomically consume one retained matching event with " +
				"step completion; an event already committed when timeout is decided and accepted no later than " +
				"its deadline wins, even at the same timestamp, and cannot satisfy two " +
				"waits. A retained eligible event resolves after commit in this context. Otherwise stop before " +
				"publishing waiting; that Promise never resolves there. A new context replays to resolve the consumed " +
				"payload, or durably records wait_timeout before rejecting with that Host Error. Absolute instance " +
				"lifetime wins even if timeout is later. Finished-name replay follows the common rules."
			operation.Errors = []string{"step_failed", "wait_timeout", "step_definition_mismatch", "step_limit_exceeded", "lifetime_exceeded", "backend_unavailable"}
		}
	}
	definition.Fixtures = append(definition.Fixtures,
		InterfaceFixture{Name: "event-key-retry-is-one-admission", Steps: []InterfaceFixtureStep{
			{Operation: "create", Input: map[string]any{"id": "keyed-events"}, Expected: map[string]any{"id": "keyed-events", "status": "queued"}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-events", "type": "approved", "eventId": "approval-1", "payload": map[string]any{"approved": true}}, Expected: map[string]any{}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-events", "type": "approved", "eventId": "approval-1", "payload": map[string]any{"approved": true}}, Expected: map[string]any{}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-events", "type": "approved", "eventId": "approval-1", "payload": map[string]any{"approved": false}}, ExpectedError: "event_conflict"},
		}},
		InterfaceFixture{Name: "keyed-event-retry-after-terminal", Steps: []InterfaceFixtureStep{
			{Operation: "create", Input: map[string]any{"id": "keyed-terminal"}, Expected: map[string]any{"id": "keyed-terminal", "status": "queued"}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-terminal", "type": "approved", "eventId": "approval-1"}, Expected: map[string]any{}},
			{Operation: "terminate", Input: map[string]any{"id": "keyed-terminal"}, Expected: map[string]any{}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-terminal", "type": "approved", "eventId": "approval-1"}, Expected: map[string]any{}},
			{Operation: "sendEvent", Input: map[string]any{"id": "keyed-terminal", "type": "approved"}, ExpectedError: "instance_terminal"},
		}},
	)
	return definition
}

func workflowCandidateErrorReasonStrings() []string {
	return []string{"run_threw", "step_failed", "step_limit_exceeded", "lifetime_exceeded", "step_definition_mismatch"}
}

func workflowCandidateInstanceStatus() map[string]any {
	reasons := make([]any, 0, 5)
	for _, reason := range workflowCandidateErrorReasonStrings() {
		reasons = append(reasons, reason)
	}
	return closedObject([]string{"status"}, map[string]any{
		"status": map[string]any{"type": "string", "enum": workflowStatusVocabulary},
		"output": workflowDocument(),
		"error": closedObject([]string{"reason"}, map[string]any{
			"reason":  map[string]any{"type": "string", "enum": reasons},
			"message": stringSchema(0, 8192),
		}),
	})
}

func renderWorkflowCandidatePair() (WorkflowCandidate, error) {
	definition := WorkflowCandidateInterface()
	if err := ValidateInterfaceDefinitions([]InterfaceDefinition{definition}); err != nil {
		return WorkflowCandidate{}, fmt.Errorf("workflow candidate Interface authoring: %w", err)
	}
	iface, err := renderInterfaceContract(definition.Name, definition.Version, definition)
	if err != nil {
		return WorkflowCandidate{}, fmt.Errorf("workflow candidate Interface: %w", err)
	}
	base, ok := ByKind("DurableWorkflow")
	if !ok {
		return WorkflowCandidate{}, fmt.Errorf("current catalog has no DurableWorkflow Form")
	}
	base.DefinitionVersion = WorkflowCandidateFormVersion
	base.Description = "DurableWorkflow with the exact worker.workflow@2.0.0 portable " +
		"class execution contract. The worker and className fix identity, not code: each new context uses the " +
		"active deployment's then-current weighted WorkerVersion. Every weighted version must expose the named " +
		"constructible class with callable prototype run before Ready. Instances and step journals are runtime " +
		"data, not Resources. Old DurableWorkflow definitions retain their exact earlier contracts."
	// Rendering resolves only registered provided interfaces. Attach the exact
	// new Interface after rendering; never temporarily register it globally.
	base.ProvidedInterfaces = nil
	base.Fields = append([]model.Field(nil), base.Fields...)
	for i := range base.Fields {
		if base.Fields[i].Wire == "className" {
			base.Fields[i].Doc = "Immutable named class export in the serving main ES module. Each weighted " +
				"version must export a constructible class with callable prototype run; construction takes only " +
				"the declared env. Absence or incompatible export keeps the workflow from Ready and create fails " +
				"unsupported_capability; no vendor base class or native context is required."
		}
	}
	if err := base.Validate(); err != nil {
		return WorkflowCandidate{}, fmt.Errorf("workflow candidate Form authoring: %w", err)
	}
	form, err := renderForm(base, newTargetContractResolver())
	if err != nil {
		return WorkflowCandidate{}, fmt.Errorf("workflow candidate Form: %w", err)
	}
	form.Definition.ProvidedInterfaces = []formpackage.InterfaceRef{{
		APIVersion: InterfaceAPIVersion, Name: iface.Name, Version: iface.Version, SchemaDigest: iface.SchemaDigest,
	}}
	form.DefinitionJSON, err = marshalIndented(form.Definition)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	if _, err := formpackage.ValidateDefinition([]byte(form.DefinitionJSON)); err != nil {
		return WorkflowCandidate{}, fmt.Errorf("workflow candidate Form Core validation: %w", err)
	}
	return WorkflowCandidate{Form: form, Interface: iface}, nil
}
