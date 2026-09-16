package edgeformcatalog

import (
	"fmt"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// WorkflowCandidate is a local, unpublished forward closure. Rendering does
// not register contracts, create a package, or advertise Provider/Host support.
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
	WorkflowCandidateFormVersion      = "0.2.0-workflow.1"
)

// These numeric Interface/Binding identities are development authoring values,
// not a publication or reservation. Core v1.1.0 disallows prerelease versions
// for those two contract types. The enclosing Forms have explicit prereleases.
// Existing worker.workflow@1.0.0 and all of its exact closures remain unchanged.
func WorkflowCandidateInterface() InterfaceDefinition {
	definition := workerWorkflowInterface()
	definition.Version = WorkflowCandidateInterfaceVersion
	definition.Title = "Portable durable workflow class execution"
	definition.Description = "Unpublished forward worker.workflow contract. Its named class ABI is defined by run below. It " +
		"supplements worker.runtime@1.1.0 without changing the required plain-object default export, " +
		"fetch/scheduled/queue handlers, globals or declared environment. All documents use the create " +
		"operation's data-only rules; undefined means absent only as a whole optional document. Instance " +
		"status is exactly queued, running, sleeping, waiting, complete, errored or terminated. Instances are " +
		"runtime data, not Resources. Retained ids cannot be reused; terminal records and ids remain retained " +
		"for 2592000 seconds.\n\nEach fresh context selects the deployment's THEN-CURRENT weighted " +
		"WorkerVersion and pins it for that context. Replay creates a new context and class object: fields, " +
		"closures, module state and pending Promises do not survive. Step execution is at-least-once with " +
		"memoized outcomes. A process death after an effect but before its outcome commit can repeat the " +
		"effect. Callbacks must be idempotent; code outside steps, including module evaluation and " +
		"constructors, must be side-effect-free and deterministic against history. Deployment changes must " +
		"remain compatible with in-flight histories. The Host cannot verify these author obligations.\n\nThe " +
		"Host stops code before publishing sleeping, waiting or any terminal state; no context remains " +
		"parked. Nothing runs after the absolute 31536000-second lifetime cutoff. Bounds and infrastructure " +
		"interruptions are Host control, never catchable app sentinels. A lost context may be retried without " +
		"claiming completion. Step success is durably saved before resolution; first resolution and replay " +
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
		"Names/types contain 1-256 Unicode scalar values.\n\nHost step Errors have an own, non-enumerable, " +
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
			operation.Description = "Start one instance with optional id and params. Refuse a retained id with instance_exists; mint an " +
				"id if absent. No serving deployment yields unsupported_capability. ALL workflow documents (params, " +
				"results, payloads, outputs) have a plain/null-prototype object root with at most 1024 properties and " +
				"1048576 RFC 8785 canonical UTF-8 bytes. Nested values are null, booleans, finite numbers, " +
				"Unicode-scalar strings, plain/null-prototype objects or dense ordinary Arrays. Arrays have only " +
				"length and enumerable index data properties; objects have only enumerable own string data " +
				"properties. Refuse accessors, symbols, holes, extra array properties, other prototypes, cycles, " +
				"functions, BigInt, nested undefined and non-JSON values. Shared acyclic objects are allowed. " +
				"Serialization inspects descriptors, never invokes getters or toJSON. Invalid params are " +
				"invalid_params; canonical byte overflow is document_too_large."
		case "status":
			operation.Description = "Read a retained instance. Output occurs only on complete and error only on " +
				"errored. The closed error reasons are run_threw, step_failed, step_limit_exceeded, lifetime_exceeded " +
				"and step_definition_mismatch. Workflow failure is a successful status read, not an operation rejection."
			operation.OutputSchema = withDialect(workflowCandidateInstanceStatus())
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
				"step completion; an event accepted no later than timeout wins over timeout and cannot satisfy two " +
				"waits. A retained eligible event resolves after commit in this context. Otherwise stop before " +
				"publishing waiting; that Promise never resolves there. A new context replays to resolve the consumed " +
				"payload, or durably records wait_timeout before rejecting with that Host Error. Absolute instance " +
				"lifetime wins even if timeout is later. Finished-name replay follows the common rules."
			operation.Errors = []string{"step_failed", "wait_timeout", "step_definition_mismatch", "step_limit_exceeded", "lifetime_exceeded", "backend_unavailable"}
		}
	}
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
	base.Description = "Unpublished forward DurableWorkflow with the exact worker.workflow@2.0.0 portable " +
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
