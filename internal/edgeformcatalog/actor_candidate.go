package edgeformcatalog

import (
	"fmt"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// ActorCandidate is the unregistered actor half of the aggregate runtime
// candidate.  It is deliberately a pair: the Form and Binding each carry the
// exact digest of the forward Interface, but none is added to the registered
// catalogs by rendering.
type ActorCandidate struct {
	Form      RenderedForm     `json:"form"`
	Interface RenderedContract `json:"interface"`
	Binding   RenderedContract `json:"binding"`
}

const (
	// Numeric Interface/Binding versions are only the nearest Core-valid
	// development identities.  They are not allocations and do not authorize
	// publication or Host support.
	ActorCandidateInterfaceName    = "worker.actor"
	ActorCandidateInterfaceVersion = "2.0.0"
	ActorCandidateBindingName      = "module-worker.actor"
	ActorCandidateBindingVersion   = "2.0.0"
	ActorCandidateFormKind         = "ActorNamespace"
	ActorCandidateFormSlug         = "actor-namespace"
	ActorCandidateFormVersion      = "0.2.0-actor.1"
)

// ActorCandidateInterface is the exact forward class/context and socket
// contract selected in the Actor proposal.  The current worker.actor
// operation vocabulary remains the data-plane surface; the newly resolved
// class, retirement, upgrade and socket rules are normative prose on this
// forward identity rather than an invented provider-specific operation.
func ActorCandidateInterface() InterfaceDefinition {
	definition := workerActorInterface()
	definition.Version = ActorCandidateInterfaceVersion
	definition.Title = "Addressable single-context actor with sockets"
	definition.Description = "Unpublished forward worker.actor contract. An ordinary named class is constructed " +
		"with constructor(context, env), where context exposes the actor id, private SQL storage, one alarm slot and Host-owned " +
		"socket facades. The class has callable prototype fetch(request, turn), alarm(turn), socketMessage(socket, data, " +
		"turn), socketClose(socket, event, turn), and socketError(socket, event, turn) methods; optional start(turn) " +
		"runs before the creating event. Every weighted WorkerVersion must expose the class with this complete " +
		"surface. Constructor/start/fetch failures before an HTTP head produce a complete generic 500; failure to " +
		"begin execution is backend_unavailable, and no alternate weighted Version is tried. The actor's private SQL " +
		"store is keyed by namespace incarnation and opaque actor id, survives context eviction and code rollback, and " +
		"admits the existing EdgeSqlValue/result bounds plus actor-owned schema statements. One alarm successor and " +
		"one unsettled delivery obligation are distinct: set replaces only the successor, clear removes only that " +
		"successor, and a throw, deadline or owner loss retains the running obligation for retry. There is no actor " +
		"waitUntil or detached-background-work API. HTTP response heads retain worker.service streaming semantics; " +
		"same-id admission waits for actor-owned response/request production to finish and for child retirement, while " +
		"different ids proceed independently. Synchronous construction/start has a 30-second wall deadline; event " +
		"CPU is capped at 30 seconds, connected HTTP has no independent wall timer while its original caller remains " +
		"connected, and alarm/message/close/error callbacks have a 15-minute wall deadline. Abort cancels owned " +
		"streams and terminates the child; a promise rejection, storage fence or boolean is not retirement proof. " +
		"Socket accept is valid only in the original incoming client-upgrade invocation. It returns a real branded " +
		"Response with status 101 and null body plus one Host reservation; new Response(upgrade.body, upgrade) aliases " +
		"that reservation, clone throws TypeError, and an unbranded status-101 Response throws RangeError. Copies, " +
		"spreads, serialization, service forwarding and another invocation cannot mint or preserve the reservation. " +
		"conflicting reserved handshake fields are rejected at outer commitment: absent fields may be filled, but a conflicting " +
		"Connection, Upgrade, Sec-WebSocket-Accept, Sec-WebSocket-Protocol or Sec-WebSocket-Extensions value abandons " +
		"the reservation and provisional sends and returns HTTP 502 before any 101. There is no independent 30-second " +
		"reservation timer; its lifetime is the original connected HTTP invocation and abort signal. Socket callbacks " +
		"are serialized per actor id; exactly one socketClose or socketError is admitted for a terminal event, with " +
		"transport_error for loss without a usable close code. The candidate bounds connections at 10000, each encoded " +
		"inbound/outbound frame at 33554432 bytes, attachments at 16384 bytes and each outbound queue at 33554432 " +
		"bytes. close defaults omitted code/reason to 1000/empty, reason without a code uses 1000, and every reason " +
		"fits 123 UTF-8 bytes. Errors use the stable transport codes declared by the proposal. A stable Host owner " +
		"keeps the namespace incarnation, id inventory, admission gate, alarm obligations and socket broker; it must " +
		"terminate the replaceable child and prove no held completion can resume it before the next same-id event. " +
		"Namespace deletion advances the epoch, cancels queued work, abandons upgrades, closes connections with 1001 " +
		"best effort and reports success only after authoritative absence readback. This forward identity is not " +
		"registered and does not alter worker.actor@1.0.0 or any published bytes."

	// Keep the existing operation descriptions and fixtures as the abstract
	// actor data-plane witness, but make the changed invocation semantics
	// visible on the operation that carries the HTTP call.
	for index := range definition.Operations {
		if definition.Operations[index].Name == "fetch" {
			definition.Operations[index].Description = "Invokes the actor fetch(request, turn) method with the actor " +
				"instance as receiver. Bodies stream in both directions and the call completes at the response head; " +
				"same-id events remain queued until actor-owned producers finish and the child is retired. An uncaught " +
				"throw is a host-generated complete 500 and this operation succeeds with it; it fails only when delivery " +
				"could not begin. A response head with status 101 is committed only through the original Host-tracked " +
				"upgrade reservation described by this Interface."
			definition.Operations[index].Errors = []string{
				"request_too_large", "request_aborted", "response_aborted", "backend_unavailable",
				"invalid_upgrade", "connection_limit_exceeded", "message_too_large", "attachment_too_large",
				"transport_overloaded", "socket_closed", "invalid_close",
			}
		}
	}
	return definition
}

// renderActorCandidatePair renders the ActorNamespace Form and forward
// worker.actor Interface without touching InterfaceDefinitions or Forms.  The
// resolver supplies the aggregate's worker.runtime Interface for the class
// holder relation.
func renderActorCandidatePair(runtimeInterface RenderedContract) (ActorCandidate, error) {
	definition := ActorCandidateInterface()
	if err := ValidateInterfaceDefinitions([]InterfaceDefinition{definition}); err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Interface authoring: %w", err)
	}
	iface, err := renderInterfaceContract(definition.Name, definition.Version, definition)
	if err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Interface: %w", err)
	}

	base, ok := ByKind(ActorCandidateFormKind)
	if !ok {
		return ActorCandidate{}, fmt.Errorf("current catalog has no %s Form", ActorCandidateFormKind)
	}
	if base.Role != model.RoleIdentity || base.Slug != ActorCandidateFormSlug {
		return ActorCandidate{}, fmt.Errorf("current ActorNamespace identity drifted to %s/%s", base.Slug, base.Role)
	}
	candidate := cloneFormForCandidate(base, ActorCandidateFormVersion)
	candidate.ProvidedInterfaces = nil
	candidate.Description = "Unpublished forward ActorNamespace with the exact worker.actor@2.0.0 class, " +
		"context, alarm, private SQL, serialized invocation and Host-owned socket contract. The namespace " +
		"retains its durable identity and active deployment code selection; actors are runtime data, not Resources. " +
		"Existing ActorNamespace definitions retain their exact earlier contracts."
	for index := range candidate.Fields {
		if candidate.Fields[index].Wire == "className" {
			candidate.Fields[index].Doc = "Immutable named actor class export in the serving main ES module. Every weighted " +
				"version must provide callable prototype start (when present), fetch, alarm, socketMessage, socketClose " +
				"and socketError methods with constructor(context, env); an incompatible export keeps the namespace from " +
				"Ready and invocation is refused rather than queued."
		}
	}
	retargetWorkerRuntimeFields(&candidate, RuntimeCandidateInterfaceVersion)
	if err := candidate.Validate(); err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Form authoring: %w", err)
	}
	resolver := newRuntimeCandidateResolver(runtimeInterface)
	rendered, err := renderForm(candidate, resolver)
	if err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Form: %w", err)
	}
	rendered.Definition.ProvidedInterfaces = []formpackage.InterfaceRef{{
		APIVersion: InterfaceAPIVersion, Name: iface.Name, Version: iface.Version, SchemaDigest: iface.SchemaDigest,
	}}
	rendered.DefinitionJSON, err = marshalIndented(rendered.Definition)
	if err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Form JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON)); err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Form Core validation: %w", err)
	}

	bindingDefinition, err := actorCandidateBinding(iface)
	if err != nil {
		return ActorCandidate{}, err
	}
	binding, err := renderContract(bindingDefinition.Name, bindingDefinition.Version, bindingDefinition)
	if err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Binding: %w", err)
	}
	if err := formpackage.ValidateBindingDefinition([]byte(binding.DefinitionJSON)); err != nil {
		return ActorCandidate{}, fmt.Errorf("actor candidate Binding Core validation: %w", err)
	}
	return ActorCandidate{Form: rendered, Interface: iface, Binding: binding}, nil
}

func actorCandidateBinding(iface RenderedContract) (BindingDefinition, error) {
	definitions, err := BindingDefinitions()
	if err != nil {
		return BindingDefinition{}, fmt.Errorf("actor candidate current Bindings: %w", err)
	}
	var current BindingDefinition
	found := false
	for _, definition := range definitions {
		if definition.Name != ActorCandidateBindingName {
			continue
		}
		if found {
			return BindingDefinition{}, fmt.Errorf("current catalog has duplicate %s Bindings", ActorCandidateBindingName)
		}
		current = definition
		found = true
	}
	if !found {
		return BindingDefinition{}, fmt.Errorf("current catalog has no %s Binding", ActorCandidateBindingName)
	}
	if got, want := current.RuntimeProjection.Operations, []string{"idFromName", "newUniqueId", "fetch"}; !sameStrings(got, want) {
		return BindingDefinition{}, fmt.Errorf("current actor Binding operations = %#v, want %#v", got, want)
	}
	current.AllowedTargetForms = append([]AllowedTargetForm(nil), current.AllowedTargetForms...)
	current.RuntimeProjection.Operations = append([]string(nil), current.RuntimeProjection.Operations...)
	current.Name = ActorCandidateBindingName
	current.Version = ActorCandidateBindingVersion
	current.TargetInterface = formpackage.InterfaceRef{
		APIVersion: InterfaceAPIVersion, Name: iface.Name, Version: iface.Version, SchemaDigest: iface.SchemaDigest,
	}
	current.AllowedTargetForms = []AllowedTargetForm{{APIVersion: Family.APIVersion(), Kind: ActorCandidateFormKind}}
	current.Description = "Unpublished forward Binding targeting the exact worker.actor@2.0.0 class/context and " +
		"socket-capable Interface. The caller projection remains env.NAME.idFromName(name), " +
		"env.NAME.newUniqueId() and env.NAME.get(id).fetch(request): socket upgrade authority is available only " +
		"through the actor's original incoming client-upgrade invocation and is never a public token or native " +
		"WebSocket handle. The target's private SQL, alarm and callback rules are not projected to callers. " + current.Description
	return current, nil
}
