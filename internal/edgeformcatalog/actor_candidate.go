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
	definition.Description = "Unpublished worker.actor; global class/init/HTTP-failure rules are on fetch. " +
		"Constructable named export: constructor(context, env); pure inspection. context: opaque id, private SQL, one alarm slot, " +
		"Host sockets. Serialized per id. SQL keyed by incarnation/id survives eviction/rollback and permits actor schema. " +
		"Alarm: one successor and one unsettled delivery; set/clear change successor only; failure retains delivery; no " +
		"waitUntil/detached work. HTTP heads complete; same-id waits for request cancel/child retirement. " +
		"Construction/start wall and event CPU: 30 seconds; connected HTTP has no wall timer; alarm/message/close/error " +
		"wall: 15 minutes. Abort cancels streams/reservation then terminates child; rejection/storage fence/boolean is no retirement " +
		"proof. accept requires original WebSocket-upgrade Host chain, not Request identity. Protocol is " +
		"absent or exactly one token offered by the original client; else accept rejects invalid_upgrade. Returns branded 101/null " +
		"Response with one-shot reservation; only new Response(upgrade.body, upgrade) aliases it; clone throws TypeError and " +
		"unbranded status-101 throws RangeError. Non-null body, copy/spread/serialization/forwarding/other invocation cannot " +
		"preserve/mint it; cross-request/second commit fails. Outer commitment may fill absent reserved fields. A " +
		"conflicting Connection/Upgrade/Sec-WebSocket-Accept/Sec-WebSocket-Protocol/Sec-WebSocket-Extensions abandons " +
		"reservation/provisional sends and returns HTTP 502 before any 101. An outer response without its reservation, outer " +
		"throw or head-send failure abandons reservation/provisional sends; commitment/abort/abandonment settles every alias; " +
		"no independent reservation timer. " +
		"After head, callbacks serialize per id and admit exactly one close/error; unusable close code gives transport_error; " +
		"process/broker loss may prevent a callback. Socket ids persist across eviction/Version changes, are opaque/unique while " +
		"live/never reused; reconnect gets a new id. list returns committed " +
		"live/closing sockets lexically; provisional sockets visible only through accept. Terminal callback hides the " +
		"socket from get/list; send/close/setAttachment fail socket_closed; getAttachment remains until settle. Limits per namespace " +
		"incarnation/id: 10000 live/provisional connections counted from accept; 33554432 encoded bytes/frame each way; " +
		"16384-byte attachments; 33554432-byte outbound queues. UTF-8 " +
		"strings and Uint8Array byteLength count; oversized inbound closes 1009, oversized send " +
		"throws message_too_large without a partial frame. send returns after local broker acceptance, not peer receipt, " +
		"persistence or drain; occupancy falls after complete handoff; outbound overflow throws transport_overloaded without a partial frame. " +
		"close requests the handshake without guaranteeing a close frame. Attachments are copied; null clears; " +
		"oversized initial/replacement value rejects as attachment_too_large and leaves " +
		"the old value unchanged. setAttachment is atomic; bytes survive " +
		"eviction while transport survives and are removed on loss/close/deletion. close defaults omitted/undefined " +
		"code/reason to 1000/empty, reason without code to 1000 and " +
		"reason to at most 123 UTF-8 bytes. Application codes are 1000 or 3000-4999; invalid values throw invalid_close. " +
		"Host closures: 1009 oversized data, 1011 callback failure/deadline, 1012 planned restart and 1001 deletion. " +
		"Error name and code are readonly and share one stable snake-case value. " +
		"Facade errors: accept invalid_upgrade/connection_limit_exceeded/attachment_too_large; send message_too_large/transport_overloaded/socket_closed; " +
		"setAttachment attachment_too_large/socket_closed; close invalid_close/socket_closed. Callback failure/deadline closes 1011; " +
		"No callback replay after process/broker loss; durable ids/acks enable replay; close is not cleanup. Owner " +
		"keeps incarnation/id inventory, admission, alarms and broker; terminates child and proves quiescence before next " +
		"same-id event. Deletion advances epoch, cancels/abandons " +
		"work/upgrades, closes 1001 best effort and succeeds only after authoritative absence readback."

	// Keep the existing operation descriptions and fixtures as the abstract
	// actor data-plane witness, but make the changed invocation semantics
	// visible on the operation that carries the HTTP call.
	for index := range definition.Operations {
		if definition.Operations[index].Name == "fetch" {
			definition.Operations[index].Description = "Global Interface rules, not fetch-only: every weighted Version has class. " +
				"Required fetch/alarm/socketMessage/socketClose/socketError and optional start are callable prototype methods, " +
				"including inherited application methods. Accessors and per-instance replacements are refused. Constructor " +
				"captures context/env synchronously and starts no asynchronous work; start is awaited before delivery, repeats " +
				"after eviction and must be idempotent. Handlers share receiver. After execution begins, uncaught constructor/start/fetch " +
				"or facade failure yields a complete generic 500; its body exposes no tenant " +
				"exception details; stub fetch returns it. Only failure to begin: backend_unavailable; no alternate Version. Bodies " +
				"stream both ways; head completes fetch; same-id waits for actor producers/child retirement. Pre-head deadline yields " +
				"504; post-head unfinished body errors response_aborted; canceled request is request_aborted. A 101 commits only " +
				"original Host reservation."
			definition.Operations[index].Errors = []string{
				"request_too_large", "request_aborted", "response_aborted", "backend_unavailable",
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
