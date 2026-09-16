package edgeformcatalog

import (
	"fmt"
	"slices"
	"strings"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// RuntimeCandidatePair is the exact Form/Interface/Binding portion of one
// forward capability.  WorkerVersion and WorkerDeployment are intentionally
// not members: the aggregate emits one shared pair for Actor, Workflow and
// Vector instead of three independently drifting worker closures.
type RuntimeCandidatePair struct {
	Form      RenderedForm     `json:"form"`
	Interface RenderedContract `json:"interface"`
	Binding   RenderedContract `json:"binding"`
}

// RuntimeCandidate is one unpublished Actor+Workflow+Vector closure.  Every
// artifact is rendered and Core-validated in memory.  It never registers a
// catalog identity, allocates a release version, mutates current package
// bytes, or advertises Host support.
type RuntimeCandidate struct {
	RuntimeInterface RenderedContract     `json:"runtimeInterface"`
	ModuleWorker     RenderedForm         `json:"moduleWorker"`
	Actor            ActorCandidate       `json:"actor"`
	Workflow         RuntimeCandidatePair `json:"workflow"`
	Vector           RuntimeCandidatePair `json:"vector"`
	// RuntimeDependants contains only the current attachment Forms whose
	// worker relation requires worker.runtime.  ActorNamespace and
	// DurableWorkflow appear in Actor and Workflow respectively; WorkerVersion
	// and WorkerDeployment are the aggregate's single shared pair below.
	RuntimeDependants []RenderedForm `json:"runtimeDependants"`
	WorkerVersion     RenderedForm   `json:"workerVersion"`
	WorkerDeployment  RenderedForm   `json:"workerDeployment"`
}

const (
	// These numeric Interface/Binding identities are Core-valid development
	// values only.  They are deliberately not release allocations or registry
	// entries; final identities remain a separate qualification decision.
	RuntimeCandidateInterfaceName    = WorkerRuntimeInterfaceName
	RuntimeCandidateInterfaceVersion = "2.0.0"

	RuntimeCandidateModuleWorkerFormVersion = "0.2.0-runtime.1"
	// The aggregate DurableWorkflow Form has a distinct development identity
	// from the standalone workflow draft because its provided Interface points
	// at worker.workflow@3.0.0 (the runtime-2 closure).  The prerelease lane is
	// an unpublished authoring value only; it allocates nothing for release.
	RuntimeWorkflowCandidateFormVersion      = "0.2.0-runtime.1"
	RuntimeWorkflowCandidateInterfaceName    = WorkflowCandidateInterfaceName
	RuntimeWorkflowCandidateInterfaceVersion = "3.0.0"
	RuntimeWorkflowCandidateBindingName      = WorkflowCandidateBindingName
	RuntimeWorkflowCandidateBindingVersion   = "3.0.0"
	RuntimeCandidateWorkerVersionVersion     = "0.4.0-runtime.1"
	RuntimeCandidateWorkerDeploymentVersion  = "0.3.0-runtime.1"
	RuntimeCandidateDependentFormVersion     = "0.1.0-runtime.1"
)

// RuntimeCandidateInterface is the forward Worker runtime identity required
// by the Actor class/socket proposal.  The existing load/handler operation
// schemas remain the runtime ABI; the class/context and transport rules are
// stated on this exact new Interface identity rather than represented by
// invented provider-specific operations.
func RuntimeCandidateInterface() InterfaceDefinition {
	definition := workerRuntimeInterface()
	definition.Version = RuntimeCandidateInterfaceVersion
	definition.Title = "ES Module Worker and Actor runtime ABI"
	definition.Description = "Unpublished forward worker.runtime contract. The existing plain-object default export, " +
		"fetch/scheduled/queue handlers, streamed HTTP bodies, exact environment-name closure and ctx.waitUntil " +
		"semantics remain unchanged. In addition, a Worker Version that serves ActorNamespace classes exposes the " +
		"ordinary named class ABI fixed by worker.actor: constructor(context, env), optional start(turn), and callable " +
		"prototype fetch(request, turn), alarm(turn), socketMessage(socket, data, turn), socketClose(socket, event, " +
		"turn) and socketError(socket, event, turn). Inspection is side-effect-free and every weighted Version must " +
		"provide the complete class surface before Ready. The context carries only the actor id, private SQL store, " +
		"one alarm slot and Host-owned socket facades; no Host controls or credentials enter env. The owner pins one " +
		"selected Version per event, serializes same-id admission, drains actor-owned HTTP producers and proves child " +
		"retirement before the next event; different ids proceed independently. Construction/start has a 30-second wall " +
		"deadline, actor event CPU has a 30-second active limit, connected HTTP has no independent wall deadline while " +
		"its original caller remains connected, and alarm/message/close/error callbacks have a 15-minute wall limit. " +
		"Abort cancels owned streams and terminates the child; a promise rejection or storage fence is not proof of " +
		"retirement. Actor HTTP upgrade reservations are real branded Responses with status 101 and null body. Only " +
		"new Response(upgrade.body, upgrade) aliases the reservation; clone throws TypeError and an unbranded status-101 " +
		"Response throws RangeError. Copies, spreads, serialization, service forwarding and another invocation cannot " +
		"mint it. At outer commitment absent handshake fields may be filled, but conflicting reserved Connection, " +
		"Upgrade, Sec-WebSocket-Accept, Sec-WebSocket-Protocol or Sec-WebSocket-Extensions rejects commitment, abandons " +
		"provisional sends and returns HTTP 502 before any 101. The reservation has no independent timer: its lifetime " +
		"is the original connected HTTP invocation and abort signal. This exact identity is a local candidate only; it " +
		"does not reinterpret worker.runtime@1.1.0 or change published bytes."
	return definition
}

// RuntimeWorkflowCandidateInterface is an explicit aggregate Workflow
// contract.  The standalone imported draft remains worker.workflow@2.0.0 and
// deliberately describes worker.runtime@1.1.0; reusing that same identity
// here would give one name/version two different contracts.  This copy carries
// the reviewed operation schemas and fixtures under a distinct development
// identity while changing every runtime requirement to the aggregate's exact
// worker.runtime@2.0.0 Interface.
func RuntimeWorkflowCandidateInterface() InterfaceDefinition {
	definition := WorkflowCandidateInterface()
	definition.Name = RuntimeWorkflowCandidateInterfaceName
	definition.Version = RuntimeWorkflowCandidateInterfaceVersion
	definition.Title = "Aggregate durable workflow class execution"
	definition.Description = strings.ReplaceAll(definition.Description, "worker.runtime@1.1.0", "worker.runtime@2.0.0")
	definition.Description = "Unpublished aggregate worker.workflow contract. Its exact runtime requirement is " +
		"worker.runtime@2.0.0; it is a distinct development identity from the standalone workflow draft. " +
		definition.Description
	definition.Operations = append([]InterfaceOperation(nil), definition.Operations...)
	for index := range definition.Operations {
		definition.Operations[index].Description = strings.ReplaceAll(
			definition.Operations[index].Description,
			"worker.runtime@1.1.0",
			"worker.runtime@2.0.0",
		)
	}
	return definition
}

func renderRuntimeWorkflowCandidateInterface() (RenderedContract, error) {
	definition := RuntimeWorkflowCandidateInterface()
	if err := ValidateInterfaceDefinitions([]InterfaceDefinition{definition}); err != nil {
		return RenderedContract{}, fmt.Errorf("aggregate workflow candidate Interface authoring: %w", err)
	}
	rendered, err := renderInterfaceContract(definition.Name, definition.Version, definition)
	if err != nil {
		return RenderedContract{}, fmt.Errorf("aggregate workflow candidate Interface: %w", err)
	}
	if rendered.Name != RuntimeWorkflowCandidateInterfaceName || rendered.Version != RuntimeWorkflowCandidateInterfaceVersion {
		return RenderedContract{}, fmt.Errorf("aggregate workflow candidate Interface identity drifted to %s@%s", rendered.Name, rendered.Version)
	}
	if strings.Contains(rendered.DefinitionJSON, "worker.runtime@1.1.0") {
		return RenderedContract{}, fmt.Errorf("aggregate workflow candidate Interface retains stale worker.runtime@1.1.0 prose")
	}
	return rendered, nil
}

// RenderRuntimeCandidate emits one aggregate closure.  It derives an explicit
// aggregate Workflow Interface/Binding identity and uses the reviewed Vector
// pair renderer only for its pair artifacts; independent WorkerVersion and
// WorkerDeployment results are never included, so stale parallel closures
// cannot appear in this output.
func RenderRuntimeCandidate() (RuntimeCandidate, error) {
	runtimeContract, err := renderRuntimeCandidateInterface()
	if err != nil {
		return RuntimeCandidate{}, err
	}

	actor, err := renderActorCandidatePair(runtimeContract)
	if err != nil {
		return RuntimeCandidate{}, err
	}

	workflowInterface, err := renderRuntimeWorkflowCandidateInterface()
	if err != nil {
		return RuntimeCandidate{}, err
	}
	workflowForm, err := renderRuntimeWorkflowForm(runtimeContract, workflowInterface)
	if err != nil {
		return RuntimeCandidate{}, err
	}
	workflowBindingDefinition, err := runtimeWorkflowCandidateBinding(workflowInterface)
	if err != nil {
		return RuntimeCandidate{}, err
	}
	workflowBinding, err := renderCandidateBinding(workflowBindingDefinition)
	if err != nil {
		return RuntimeCandidate{}, err
	}

	vectorIndependent, err := renderVectorIndexCandidatePair()
	if err != nil {
		return RuntimeCandidate{}, err
	}
	if vectorIndependent.Interface.SchemaDigest != vectorIndexCandidateInterfaceSchemaDigest {
		return RuntimeCandidate{}, fmt.Errorf(
			"vector candidate Interface digest = %q, want reviewed digest %q",
			vectorIndependent.Interface.SchemaDigest, vectorIndexCandidateInterfaceSchemaDigest,
		)
	}
	vectorBindingDefinition := vectorIndexCandidateBinding(vectorIndependent.Interface)
	vectorBinding, err := renderCandidateBinding(vectorBindingDefinition)
	if err != nil {
		return RuntimeCandidate{}, err
	}

	moduleWorker, err := renderRuntimeModuleWorker(runtimeContract)
	if err != nil {
		return RuntimeCandidate{}, err
	}
	dependants, err := renderRuntimeDependants(runtimeContract)
	if err != nil {
		return RuntimeCandidate{}, err
	}
	workerVersion, err := renderAggregateWorkerVersion(runtimeContract, actor, workflowInterface, workflowBinding, vectorIndependent.Interface, vectorBinding)
	if err != nil {
		return RuntimeCandidate{}, err
	}
	workerDeployment, err := renderAggregateWorkerDeployment(runtimeContract, moduleWorker, workerVersion)
	if err != nil {
		return RuntimeCandidate{}, err
	}

	candidate := RuntimeCandidate{
		RuntimeInterface: runtimeContract,
		ModuleWorker:     moduleWorker,
		Actor:            actor,
		Workflow: RuntimeCandidatePair{
			Form: workflowForm, Interface: workflowInterface, Binding: workflowBinding,
		},
		Vector: RuntimeCandidatePair{
			Form: vectorIndependent.Form, Interface: vectorIndependent.Interface, Binding: vectorBinding,
		},
		RuntimeDependants: dependants,
		WorkerVersion:     workerVersion,
		WorkerDeployment:  workerDeployment,
	}
	if err := validateRuntimeCandidate(candidate); err != nil {
		return RuntimeCandidate{}, err
	}
	return candidate, nil
}

func renderRuntimeCandidateInterface() (RenderedContract, error) {
	definition := RuntimeCandidateInterface()
	if err := ValidateInterfaceDefinitions([]InterfaceDefinition{definition}); err != nil {
		return RenderedContract{}, fmt.Errorf("runtime candidate Interface authoring: %w", err)
	}
	rendered, err := renderInterfaceContract(definition.Name, definition.Version, definition)
	if err != nil {
		return RenderedContract{}, fmt.Errorf("runtime candidate Interface: %w", err)
	}
	if rendered.Name != RuntimeCandidateInterfaceName || rendered.Version != RuntimeCandidateInterfaceVersion {
		return RenderedContract{}, fmt.Errorf("runtime candidate Interface identity drifted to %s@%s", rendered.Name, rendered.Version)
	}
	return rendered, nil
}

func renderCandidateBinding(definition BindingDefinition) (RenderedContract, error) {
	rendered, err := renderContract(definition.Name, definition.Version, definition)
	if err != nil {
		return RenderedContract{}, err
	}
	if err := formpackage.ValidateBindingDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return RenderedContract{}, fmt.Errorf("candidate Binding Core validation: %w", err)
	}
	return rendered, nil
}

// runtimeWorkflowCandidateBinding is the aggregate's explicit Binding
// projection.  The standalone helper remains unchanged for its own
// worker.workflow@2.0.0 draft; this wrapper gives the runtime-2 Interface a
// distinct exact Binding identity and digest-bound target.
func runtimeWorkflowCandidateBinding(iface RenderedContract) (BindingDefinition, error) {
	definition, err := workflowCandidateBinding(iface)
	if err != nil {
		return BindingDefinition{}, err
	}
	definition.Name = RuntimeWorkflowCandidateBindingName
	definition.Version = RuntimeWorkflowCandidateBindingVersion
	definition.TargetInterface = formpackage.InterfaceRef{
		APIVersion:   InterfaceAPIVersion,
		Name:         iface.Name,
		Version:      iface.Version,
		SchemaDigest: iface.SchemaDigest,
	}
	definition.Description = strings.ReplaceAll(
		definition.Description,
		"worker.workflow@2.0.0",
		"worker.workflow@3.0.0",
	)
	definition.Description = "Unpublished aggregate Binding targeting the exact worker.workflow@3.0.0 Interface. " +
		definition.Description
	return definition, nil
}

func renderRuntimeModuleWorker(runtimeContract RenderedContract) (RenderedForm, error) {
	base, ok := ByKind("ModuleWorker")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no ModuleWorker Form")
	}
	candidate := cloneFormForCandidate(base, RuntimeCandidateModuleWorkerFormVersion)
	candidate.ProvidedInterfaces = nil
	candidate.Description = "Unpublished forward ModuleWorker whose exact worker.runtime@2.0.0 candidate " +
		"ABI includes the existing default-object handler runtime and the Actor class/context extension. The " +
		"worker identity remains the target of one active deployment; published ModuleWorker definitions retain their " +
		"existing runtime contract and bytes."
	resolver := newRuntimeCandidateResolver(runtimeContract)
	rendered, err := renderForm(candidate, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate ModuleWorker: %w", err)
	}
	service, err := InterfaceRefFor("worker.service", "1.0.0")
	if err != nil {
		return RenderedForm{}, err
	}
	rendered.Definition.ProvidedInterfaces = []formpackage.InterfaceRef{
		{APIVersion: InterfaceAPIVersion, Name: runtimeContract.Name, Version: runtimeContract.Version, SchemaDigest: runtimeContract.SchemaDigest},
		service,
	}
	rendered.DefinitionJSON, err = marshalIndented(rendered.Definition)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate ModuleWorker JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate ModuleWorker Core validation: %w", err)
	}
	return rendered, nil
}

func renderRuntimeWorkflowForm(runtimeContract, workflowInterface RenderedContract) (RenderedForm, error) {
	base, ok := ByKind("DurableWorkflow")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no DurableWorkflow Form")
	}
	candidate := cloneFormForCandidate(base, RuntimeWorkflowCandidateFormVersion)
	candidate.ProvidedInterfaces = nil
	candidate.Description = "Unpublished aggregate DurableWorkflow with the exact reviewed worker.workflow@3.0.0 " +
		"class/replay contract and the aggregate worker.runtime@2.0.0 requirement. Instances remain runtime data, " +
		"and code comes from the active deployment's weighted WorkerVersion."
	for index := range candidate.Fields {
		if candidate.Fields[index].Wire == "className" {
			candidate.Fields[index].Doc = "Immutable named class export in the serving main ES module. Each weighted " +
				"version must export a constructible class with callable prototype run; absence or incompatibility keeps " +
				"the workflow from Ready."
		}
	}
	retargetWorkerRuntimeFields(&candidate, RuntimeCandidateInterfaceVersion)
	if err := candidate.Validate(); err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate DurableWorkflow authoring: %w", err)
	}
	resolver := newRuntimeCandidateResolver(runtimeContract, workflowInterface)
	rendered, err := renderForm(candidate, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate DurableWorkflow: %w", err)
	}
	rendered.Definition.ProvidedInterfaces = []formpackage.InterfaceRef{{
		APIVersion: InterfaceAPIVersion, Name: workflowInterface.Name, Version: workflowInterface.Version, SchemaDigest: workflowInterface.SchemaDigest,
	}}
	rendered.DefinitionJSON, err = marshalIndented(rendered.Definition)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate DurableWorkflow JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return RenderedForm{}, fmt.Errorf("runtime candidate DurableWorkflow Core validation: %w", err)
	}
	return rendered, nil
}

// renderRuntimeDependants forwards the changed runtime Interface through the
// true inward-activation Forms.  The worker field on each is the sole
// dependency; resources such as WorkerBundle and queues do not receive a
// speculative version bump.
func renderRuntimeDependants(runtimeContract RenderedContract) ([]RenderedForm, error) {
	kinds := []string{"WorkerCustomDomain", "WorkerEndpoint", "WorkerCronTrigger", "QueueConsumer"}
	out := make([]RenderedForm, 0, len(kinds))
	for _, kind := range kinds {
		base, ok := ByKind(kind)
		if !ok {
			return nil, fmt.Errorf("current catalog has no runtime dependant Form %s", kind)
		}
		candidate := cloneFormForCandidate(base, RuntimeCandidateDependentFormVersion)
		retargetWorkerRuntimeFields(&candidate, RuntimeCandidateInterfaceVersion)
		if err := candidate.Validate(); err != nil {
			return nil, fmt.Errorf("runtime dependant %s authoring: %w", kind, err)
		}
		rendered, err := renderForm(candidate, newRuntimeCandidateResolver(runtimeContract))
		if err != nil {
			return nil, fmt.Errorf("runtime dependant %s: %w", kind, err)
		}
		out = append(out, rendered)
	}
	return out, nil
}

func renderAggregateWorkerVersion(runtimeContract RenderedContract, actor ActorCandidate, workflowInterface, workflowBinding, vectorInterface, vectorBinding RenderedContract) (RenderedForm, error) {
	base, ok := ByKind("WorkerVersion")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no WorkerVersion Form")
	}
	if len(base.AcceptedBindings) != 7 {
		return RenderedForm{}, fmt.Errorf("current WorkerVersion accepted %d bindings, want exactly seven", len(base.AcceptedBindings))
	}
	candidate := cloneFormForCandidate(base, RuntimeCandidateWorkerVersionVersion)
	retargetWorkerRuntimeFields(&candidate, RuntimeCandidateInterfaceVersion)
	setBindingFieldInterface(&candidate, "workflowBindings", workflowInterface.Name, workflowInterface.Version)
	setBindingFieldInterface(&candidate, "actorBindings", actor.Interface.Name, actor.Interface.Version)
	if hasWireField(candidate.Fields, "vectorBindings") {
		return RenderedForm{}, fmt.Errorf("current WorkerVersion already declares vectorBindings; aggregate would duplicate it")
	}
	candidate.Fields = append(candidate.Fields, model.Field{
		HCL: "vector_bindings", Wire: "vectorBindings", Kind: model.KindBindingList,
		TargetKind: VectorIndexCandidateFormKind, BindingType: VectorIndexCandidateBindingName,
		Target: requiresInterface(vectorInterface.Name, vectorInterface.Version), Default: []any{},
		Doc: "Typed module-worker.edge-vector bindings projecting the exact edge.vector candidate API under " +
			"JavaScript identifier names. Each method accepts one closed operation object and returns the exact " +
			"operation output document; omission declares no vector binding.",
		Example: []any{bindingInstance("VECTORS", VectorIndexCandidateFormKind, VectorIndexCandidateFormSlug)},
	})
	if err := candidate.Validate(); err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerVersion authoring: %w", err)
	}
	resolver := newRuntimeCandidateResolver(runtimeContract, actor.Interface, workflowInterface, vectorInterface)
	rendered, err := renderForm(candidate, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerVersion: %w", err)
	}

	actorRef := bindingRefFromContract(actor.Binding)
	workflowRef := bindingRefFromContract(workflowBinding)
	vectorRef := bindingRefFromContract(vectorBinding)
	if err := replaceBindingByName(&rendered.Definition.AcceptedBindings, ActorCandidateBindingName, actorRef); err != nil {
		return RenderedForm{}, err
	}
	if err := replaceBindingByName(&rendered.Definition.AcceptedBindings, WorkflowCandidateBindingName, workflowRef); err != nil {
		return RenderedForm{}, err
	}
	if countBindingName(rendered.Definition.AcceptedBindings, VectorIndexCandidateBindingName) != 0 {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerVersion already contains vector BindingRef")
	}
	rendered.Definition.AcceptedBindings = append(rendered.Definition.AcceptedBindings, vectorRef)
	rendered.DefinitionJSON, err = marshalIndented(rendered.Definition)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerVersion JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerVersion Core validation: %w", err)
	}
	return rendered, nil
}

func renderAggregateWorkerDeployment(runtimeContract RenderedContract, moduleWorker, workerVersion RenderedForm) (RenderedForm, error) {
	base, ok := ByKind("WorkerDeployment")
	if !ok {
		return RenderedForm{}, fmt.Errorf("current catalog has no WorkerDeployment Form")
	}
	candidate := cloneFormForCandidate(base, RuntimeCandidateWorkerDeploymentVersion)
	candidate.Description = "Unpublished aggregate WorkerDeployment selecting the one Actor+Workflow+Vector " +
		"WorkerVersion candidate. Its exact worker and version relations pin the forward ModuleWorker and shared " +
		"WorkerVersion Definitions; current deployment bytes remain unchanged."
	workerRef, err := renderedFormRef(moduleWorker)
	if err != nil {
		return RenderedForm{}, err
	}
	versionRef, err := renderedFormRef(workerVersion)
	if err != nil {
		return RenderedForm{}, err
	}
	resolver := newRuntimeCandidateResolver(runtimeContract)
	resolver.addFormRef(workerRef)
	resolver.addFormRef(versionRef)
	if err := candidate.Validate(); err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerDeployment authoring: %w", err)
	}
	rendered, err := renderForm(candidate, resolver)
	if err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerDeployment: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return RenderedForm{}, fmt.Errorf("aggregate WorkerDeployment Core validation: %w", err)
	}
	return rendered, nil
}

func bindingRefFromContract(contract RenderedContract) formpackage.BindingRef {
	return formpackage.BindingRef{
		APIVersion: BindingAPIVersion, Name: contract.Name, Version: contract.Version, SchemaDigest: contract.SchemaDigest,
	}
}

func replaceBindingByName(refs *[]formpackage.BindingRef, name string, replacement formpackage.BindingRef) error {
	count := countBindingName(*refs, name)
	if count != 1 {
		return fmt.Errorf("aggregate WorkerVersion has %d BindingRefs named %s, want exactly one", count, name)
	}
	for index := range *refs {
		if (*refs)[index].Name == name {
			(*refs)[index] = replacement
		}
	}
	return nil
}

func countBindingName(refs []formpackage.BindingRef, name string) int {
	count := 0
	for _, ref := range refs {
		if ref.Name == name {
			count++
		}
	}
	return count
}

func hasWireField(fields []model.Field, wire string) bool {
	for _, field := range fields {
		if field.Wire == wire {
			return true
		}
	}
	return false
}

func setBindingFieldInterface(form *model.Form, wire, name, version string) {
	for index := range form.Fields {
		if form.Fields[index].Wire != wire {
			continue
		}
		field := form.Fields[index]
		field.Target = requiresInterface(name, version)
		field.ResourceTarget = nil
		form.Fields[index] = field
	}
}

// runtimeCandidateResolver is a small exact-family overlay.  It delegates
// every current Form and Interface to the registered resolver and injects only
// candidate identities supplied by this renderer.
type runtimeCandidateResolver struct {
	base       *targetContractResolver
	interfaces map[string]model.RequiredInterface
	forms      map[string]model.TargetFormRef
}

func newRuntimeCandidateResolver(contracts ...RenderedContract) *runtimeCandidateResolver {
	resolver := &runtimeCandidateResolver{
		base: newTargetContractResolver(), interfaces: map[string]model.RequiredInterface{}, forms: map[string]model.TargetFormRef{},
	}
	for _, contract := range contracts {
		if contract.Name == "" {
			continue
		}
		key := contract.Name + "\x00" + contract.Version
		resolver.interfaces[key] = model.RequiredInterface{
			APIVersion: InterfaceAPIVersion, Name: contract.Name, Version: contract.Version, SchemaDigest: contract.SchemaDigest,
		}
	}
	return resolver
}

func (r *runtimeCandidateResolver) addFormRef(ref model.TargetFormRef) {
	r.forms[ref.APIVersion+"\x00"+ref.Kind] = ref
}

func (r *runtimeCandidateResolver) ResolveResourceTarget(target model.ResourceTarget) (model.ResolvedResourceTarget, error) {
	if target.Contract.Interface != nil {
		key := target.Contract.Interface.Name + "\x00" + target.Contract.Interface.Version
		if required, ok := r.interfaces[key]; ok {
			copy := required
			return model.ResolvedResourceTarget{ResourceNamePattern: model.PatternResourceName, RequiredInterface: &copy}, nil
		}
	}
	if target.Contract.ExactForm {
		if ref, ok := r.forms[target.Group+"\x00"+target.Kind]; ok {
			return model.ResolvedResourceTarget{ResourceNamePattern: model.PatternResourceName, TargetFormRefs: []model.TargetFormRef{ref}}, nil
		}
	}
	return r.base.ResolveResourceTarget(target)
}

func requiredInterfaceFromContract(contract RenderedContract) model.RequiredInterface {
	return model.RequiredInterface{APIVersion: InterfaceAPIVersion, Name: contract.Name, Version: contract.Version, SchemaDigest: contract.SchemaDigest}
}

// cloneFormForCandidate deep-copies all mutable authoring slices before a
// candidate changes an identity or target.  ByKind returns values sharing
// backing arrays with the registered catalog, so a shallow append would make a
// renderer capable of mutating the current source set.
func cloneFormForCandidate(base model.Form, definitionVersion string) model.Form {
	candidate := base
	candidate.DefinitionVersion = definitionVersion
	candidate.Fields = cloneFields(base.Fields)
	candidate.Outputs = cloneFields(base.Outputs)
	candidate.ProvidedInterfaces = append([]model.InterfaceRefSource(nil), base.ProvidedInterfaces...)
	candidate.AcceptedBindings = append([]model.BindingRefSource(nil), base.AcceptedBindings...)
	candidate.StructuralConstraints = append([]model.Constraint(nil), base.StructuralConstraints...)
	candidate.ResolvedUIDConstraints = append([]model.Constraint(nil), base.ResolvedUIDConstraints...)
	return candidate
}

func cloneFields(fields []model.Field) []model.Field {
	if fields == nil {
		return nil
	}
	out := make([]model.Field, len(fields))
	for index, field := range fields {
		out[index] = cloneField(field)
	}
	return out
}

func cloneField(field model.Field) model.Field {
	out := field
	out.Enum = append([]string(nil), field.Enum...)
	out.Fields = cloneFields(field.Fields)
	out.Variants = append([]model.TaggedObjectVariant(nil), field.Variants...)
	for index := range out.Variants {
		out.Variants[index].Fields = cloneFields(field.Variants[index].Fields)
	}
	if field.ResourceTarget != nil {
		target := *field.ResourceTarget
		if target.Contract.Interface != nil {
			iface := *target.Contract.Interface
			target.Contract.Interface = &iface
		}
		out.ResourceTarget = &target
	}
	if field.Target.Interface != nil {
		contract := *field.Target.Interface
		out.Target.Interface = &contract
	}
	if field.Exclusive != nil {
		exclusive := *field.Exclusive
		out.Exclusive = &exclusive
	}
	if field.Sum != nil {
		sum := *field.Sum
		out.Sum = &sum
	}
	return out
}

// retargetWorkerRuntimeFields updates only references that actually require
// worker.runtime.  It walks nested object/list/tagged fields so one true
// dependant cannot silently retain the old Interface in a child relation.
func retargetWorkerRuntimeFields(form *model.Form, version string) {
	var walk func([]model.Field)
	walk = func(fields []model.Field) {
		for index := range fields {
			field := &fields[index]
			if field.Target.Interface != nil && field.Target.Interface.Name == WorkerRuntimeInterfaceName {
				contract := *field.Target.Interface
				contract.Version = version
				field.Target.Interface = &contract
			}
			if field.ResourceTarget != nil && field.ResourceTarget.Contract.Interface != nil &&
				field.ResourceTarget.Contract.Interface.Name == WorkerRuntimeInterfaceName {
				resource := *field.ResourceTarget
				contract := *resource.Contract.Interface
				contract.Version = version
				resource.Contract.Interface = &contract
				field.ResourceTarget = &resource
			}
			walk(field.Fields)
			for variantIndex := range field.Variants {
				walk(field.Variants[variantIndex].Fields)
			}
		}
	}
	walk(form.Fields)
}

func renderedFormRef(rendered RenderedForm) (model.TargetFormRef, error) {
	digest, err := formpackage.DigestCanonicalJSON([]byte(rendered.DefinitionJSON))
	if err != nil {
		return model.TargetFormRef{}, err
	}
	return model.TargetFormRef{
		APIVersion: Family.APIVersion(), Kind: rendered.Kind,
		DefinitionVersion: rendered.Definition.DefinitionVersion, SchemaDigest: digest,
	}, nil
}

func (r *runtimeCandidateResolver) FamilyAPIVersion() string    { return Family.APIVersion() }
func (r *runtimeCandidateResolver) ResourceNamePattern() string { return model.PatternResourceName }

func sameStrings(left, right []string) bool { return slices.Equal(left, right) }

// validateRuntimeCandidate is intentionally strict about references.  A
// candidate containing both old and forward runtime/interface/digest refs is
// not a composed closure; failing here prevents a stale artifact from looking
// Core-valid merely because each individual Definition is structurally valid.
func validateRuntimeCandidate(candidate RuntimeCandidate) error {
	contracts := []RenderedContract{
		candidate.RuntimeInterface, candidate.Actor.Interface, candidate.Actor.Binding,
		candidate.Workflow.Interface, candidate.Workflow.Binding,
		candidate.Vector.Interface, candidate.Vector.Binding,
	}
	for _, contract := range contracts {
		if err := validateRenderedContract(contract); err != nil {
			return err
		}
	}
	runtimeRef := requiredInterfaceFromContract(candidate.RuntimeInterface)
	expectedInterfaces := map[string]model.RequiredInterface{
		runtimeRef.Name:                   runtimeRef,
		candidate.Actor.Interface.Name:    requiredInterfaceFromContract(candidate.Actor.Interface),
		candidate.Workflow.Interface.Name: requiredInterfaceFromContract(candidate.Workflow.Interface),
		candidate.Vector.Interface.Name:   requiredInterfaceFromContract(candidate.Vector.Interface),
	}
	if err := validateProvidedInterface(candidate.ModuleWorker, runtimeRef); err != nil {
		return err
	}
	if err := validateProvidedInterface(candidate.Actor.Form, requiredInterfaceFromContract(candidate.Actor.Interface)); err != nil {
		return err
	}
	if err := validateProvidedInterface(candidate.Workflow.Form, requiredInterfaceFromContract(candidate.Workflow.Interface)); err != nil {
		return err
	}
	if err := validateProvidedInterface(candidate.Vector.Form, requiredInterfaceFromContract(candidate.Vector.Interface)); err != nil {
		return err
	}
	forms := []RenderedForm{candidate.ModuleWorker, candidate.Actor.Form, candidate.Workflow.Form, candidate.Vector.Form, candidate.WorkerVersion, candidate.WorkerDeployment}
	forms = append(forms, candidate.RuntimeDependants...)
	for _, rendered := range forms {
		if rendered.Definition.Kind == "" {
			return fmt.Errorf("runtime candidate contains an empty Form artifact")
		}
		if digest, err := formpackage.DigestCanonicalJSON([]byte(rendered.DefinitionJSON)); err != nil {
			return fmt.Errorf("%s digest: %w", rendered.Kind, err)
		} else if digest == "" {
			return fmt.Errorf("%s has an empty Definition digest", rendered.Kind)
		}
		if err := validateCandidateInterfaceRefs(rendered.Definition.DesiredSchema, expectedInterfaces); err != nil {
			return fmt.Errorf("%s interface refs: %w", rendered.Kind, err)
		}
	}
	if countBindingName(candidate.WorkerVersion.Definition.AcceptedBindings, ActorCandidateBindingName) != 1 ||
		countBindingName(candidate.WorkerVersion.Definition.AcceptedBindings, WorkflowCandidateBindingName) != 1 ||
		countBindingName(candidate.WorkerVersion.Definition.AcceptedBindings, VectorIndexCandidateBindingName) != 1 {
		return fmt.Errorf("aggregate WorkerVersion must contain exactly one Actor, Workflow and Vector BindingRef")
	}
	if len(candidate.WorkerVersion.Definition.AcceptedBindings) != 8 {
		return fmt.Errorf("aggregate WorkerVersion acceptedBindings = %d, want seven existing plus one Vector", len(candidate.WorkerVersion.Definition.AcceptedBindings))
	}
	for _, ref := range candidate.WorkerVersion.Definition.AcceptedBindings {
		if ref.Name == ActorCandidateBindingName && (ref.Version != candidate.Actor.Binding.Version || ref.SchemaDigest != candidate.Actor.Binding.SchemaDigest) {
			return fmt.Errorf("aggregate WorkerVersion Actor BindingRef is not the exact candidate digest")
		}
		if ref.Name == WorkflowCandidateBindingName && (ref.Version != candidate.Workflow.Binding.Version || ref.SchemaDigest != candidate.Workflow.Binding.SchemaDigest) {
			return fmt.Errorf("aggregate WorkerVersion Workflow BindingRef is not the exact candidate digest")
		}
		if ref.Name == VectorIndexCandidateBindingName && (ref.Version != candidate.Vector.Binding.Version || ref.SchemaDigest != candidate.Vector.Binding.SchemaDigest) {
			return fmt.Errorf("aggregate WorkerVersion Vector BindingRef is not the exact candidate digest")
		}
	}
	return nil
}

func validateRenderedContract(contract RenderedContract) error {
	if contract.Name == "" || contract.Version == "" || contract.SchemaDigest == "" {
		return fmt.Errorf("candidate contract has incomplete identity %q@%q", contract.Name, contract.Version)
	}
	digest, err := formpackage.DigestCanonicalJSON([]byte(contract.DefinitionJSON))
	if err != nil {
		return fmt.Errorf("%s@%s digest: %w", contract.Name, contract.Version, err)
	}
	if digest != contract.SchemaDigest {
		return fmt.Errorf("%s@%s carries stale schemaDigest %s; rendered bytes are %s", contract.Name, contract.Version, contract.SchemaDigest, digest)
	}
	return nil
}

func validateCandidateInterfaceRefs(schema map[string]any, expected map[string]model.RequiredInterface) error {
	var mismatch error
	var walk func(any)
	walk = func(value any) {
		if mismatch != nil {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			if raw, ok := typed[model.RequiredInterfaceAnnotationKey]; ok {
				annotation, ok := raw.(map[string]any)
				if ok {
					name, _ := annotation["name"].(string)
					version, _ := annotation["version"].(string)
					digest, _ := annotation["schemaDigest"].(string)
					if required, candidate := expected[name]; candidate && (version != required.Version || digest != required.SchemaDigest) {
						mismatch = fmt.Errorf("stale %s Interface reference @%s %s; want @%s %s", name, version, digest, required.Version, required.SchemaDigest)
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(schema)
	return mismatch
}

func validateProvidedInterface(form RenderedForm, expected model.RequiredInterface) error {
	count := 0
	for _, provided := range form.Definition.ProvidedInterfaces {
		if provided.Name != expected.Name {
			continue
		}
		count++
		if provided.Version != expected.Version || provided.SchemaDigest != expected.SchemaDigest {
			return fmt.Errorf("%s provides stale %s Interface @%s %s", form.Kind, expected.Name, provided.Version, provided.SchemaDigest)
		}
	}
	if count == 0 {
		return fmt.Errorf("%s does not provide candidate %s Interface", form.Kind, expected.Name)
	}
	if count != 1 {
		return fmt.Errorf("%s provides %d %s Interface refs; want exactly one candidate ref", form.Kind, count, expected.Name)
	}
	return nil
}

func staleRuntimeRefs(schema map[string]any, runtime model.RequiredInterface) (bool, error) {
	var stale bool
	var walk func(any)
	walk = func(value any) {
		if stale {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			if raw, ok := typed[model.RequiredInterfaceAnnotationKey]; ok {
				annotation, ok := raw.(map[string]any)
				if !ok {
					return
				}
				name, _ := annotation["name"].(string)
				version, _ := annotation["version"].(string)
				digest, _ := annotation["schemaDigest"].(string)
				if name == WorkerRuntimeInterfaceName && (version != runtime.Version || digest != runtime.SchemaDigest) {
					stale = true
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(schema)
	return stale, nil
}

var _ model.TargetContractResolver = (*runtimeCandidateResolver)(nil)
