package edgeformcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/tako0614/takoform/formpackage"
)

// This is a bounded executable model of the candidate's observable traces,
// not an application runtime, a Host adapter, or qualification of workerd/WfP.
// Its limits and error vocabulary are checked against the owning Interface.
type workflowTraceModel struct {
	instances  map[string]*traceInstance
	pendingMax int
	mutant     traceMutant
}

type traceMutant struct {
	enqueueBeforeDedupe bool
	reexecuteFinished   bool
	acceptStaleOwner    bool
}

type traceInstance struct {
	status       string
	version      string
	owner        int
	pending      []traceEvent
	keys         map[string]traceEventKey
	steps        map[string]string
	stepErrors   map[string]string
	pendingKinds map[string]string
	attempts     map[string]int
	effects      map[string]int
	output       string
	terminal     bool
}

type traceEvent struct {
	typeName string
	key      string
	value    string
	accepted int64
}

type traceEventKey struct {
	typeName string
	value    string
}

func newWorkflowTraceModel(t *testing.T, mutant traceMutant) *workflowTraceModel {
	t.Helper()
	definition := WorkflowCandidateInterface()
	if definition.Semantics.Ordering != "per_key" {
		t.Fatal("candidate lost per-instance ordering")
	}
	limit := definition.Limits["maxPendingEventsPerInstance"]
	if limit < 1 || limit > 2048 {
		t.Fatalf("unbounded or absent pending event limit: %d", limit)
	}
	for _, required := range []struct {
		operation string
		errorCode string
	}{
		{"create", "instance_exists"},
		{"sendEvent", "event_conflict"},
		{"sendEvent", "event_queue_full"},
		{"sendEvent", "instance_terminal"},
		{"stepDo", "step_failed"},
		{"stepDo", "step_definition_mismatch"},
		{"stepWaitForEvent", "wait_timeout"},
	} {
		found := false
		for _, operation := range definition.Operations {
			if operation.Name == required.operation && slices.Contains(operation.Errors, required.errorCode) {
				found = true
			}
		}
		if !found {
			t.Fatalf("candidate omitted %s.%s", required.operation, required.errorCode)
		}
	}
	return &workflowTraceModel{instances: map[string]*traceInstance{}, pendingMax: int(limit), mutant: mutant}
}

func traceFailure(code string) error { return errors.New(code) }

func (m *workflowTraceModel) create(id string) error {
	if _, held := m.instances[id]; held {
		return traceFailure("instance_exists")
	}
	m.instances[id] = &traceInstance{status: "queued", keys: map[string]traceEventKey{}, steps: map[string]string{}, stepErrors: map[string]string{}, pendingKinds: map[string]string{}, attempts: map[string]int{}, effects: map[string]int{}}
	return nil
}

func (m *workflowTraceModel) expire(id string) { delete(m.instances, id) }

func canonicalTracePayload(value any) string {
	if value == nil {
		return "absent"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	digest, err := formpackage.DigestCanonicalJSON(raw)
	if err != nil {
		panic(err)
	}
	return "present:" + digest
}

func (m *workflowTraceModel) send(id, typeName, key string, payload any, at int64) error {
	i := m.instances[id]
	if i == nil {
		return traceFailure("unknown_instance")
	}
	fingerprint := canonicalTracePayload(payload)
	if m.mutant.enqueueBeforeDedupe && key != "" {
		i.pending = append(i.pending, traceEvent{typeName: typeName, key: key, value: fingerprint, accepted: at})
	}
	if key != "" {
		if saved, exists := i.keys[key]; exists {
			if saved.typeName != typeName || saved.value != fingerprint {
				return traceFailure("event_conflict")
			}
			return nil
		}
	}
	if i.terminal {
		return traceFailure("instance_terminal")
	}
	if len(i.pending) >= m.pendingMax {
		return traceFailure("event_queue_full")
	}
	if !m.mutant.enqueueBeforeDedupe || key == "" {
		i.pending = append(i.pending, traceEvent{typeName: typeName, key: key, value: fingerprint, accepted: at})
	}
	if key != "" {
		i.keys[key] = traceEventKey{typeName: typeName, value: fingerprint}
	}
	return nil
}

func (m *workflowTraceModel) wait(id, stepName, typeName string, deadline, now int64) (string, error) {
	i := m.instances[id]
	if i == nil || i.terminal {
		return "", traceFailure("instance_terminal")
	}
	if saved, finished := i.steps[stepName]; finished {
		return saved, nil // replay never consumes another event
	}
	if code, failed := i.stepErrors[stepName]; failed {
		return "", traceFailure(code) // failed names replay without re-evaluation
	}
	if kind, pending := i.pendingKinds[stepName]; pending && kind != "wait" {
		i.status, i.terminal = "errored", true
		i.owner++
		return "", traceFailure("step_definition_mismatch")
	}
	for index, event := range i.pending {
		if event.typeName == typeName && event.accepted <= deadline && event.accepted <= now {
			// The durable step outcome and event removal are one model transition.
			i.steps[stepName] = event.value
			delete(i.pendingKinds, stepName)
			i.pending = append(i.pending[:index], i.pending[index+1:]...)
			i.status = "running"
			return event.value, nil
		}
	}
	if now < deadline {
		i.pendingKinds[stepName] = "wait"
		i.status = "waiting"
		// Internal model marker: a real parked Promise never resolves/rejects.
		return "", traceFailure("wait_pending")
	}
	i.stepErrors[stepName] = "wait_timeout"
	delete(i.pendingKinds, stepName)
	return "", traceFailure("wait_timeout")
}

func (m *workflowTraceModel) doAttempt(id string, owner int, name string, validResult bool, maxAttempts int) error {
	i := m.instances[id]
	if i == nil || i.terminal || owner != i.owner {
		return traceFailure("stale_owner")
	}
	if _, finished := i.steps[name]; finished {
		return nil
	}
	if code, failed := i.stepErrors[name]; failed {
		return traceFailure(code)
	}
	if kind, pending := i.pendingKinds[name]; pending && kind != "do" {
		i.status, i.terminal = "errored", true
		i.owner++
		return traceFailure("step_definition_mismatch")
	}
	i.pendingKinds[name] = "do"
	i.attempts[name]++
	i.effects[name]++
	if validResult {
		i.steps[name] = "saved-result"
		delete(i.pendingKinds, name)
		return nil
	}
	if i.attempts[name] >= maxAttempts {
		i.stepErrors[name] = "step_failed"
		delete(i.pendingKinds, name)
		return traceFailure("step_failed")
	}
	i.status = "sleeping"
	i.owner++
	return traceFailure("retry_pending") // internal park marker, not a catchable application Error
}

func (m *workflowTraceModel) start(id, version string) (int, error) {
	i := m.instances[id]
	if i == nil || i.terminal {
		return 0, traceFailure("instance_terminal")
	}
	i.owner++
	i.version = version
	i.status = "running"
	return i.owner, nil
}

func (m *workflowTraceModel) step(id string, owner int, name, result string) error {
	i := m.instances[id]
	if i == nil || i.terminal || (!m.mutant.acceptStaleOwner && owner != i.owner) {
		return traceFailure("stale_owner")
	}
	if _, done := i.steps[name]; done {
		if m.mutant.reexecuteFinished {
			i.effects[name]++
		}
		return nil
	}
	i.effects[name]++
	i.steps[name] = result
	return nil
}

func (m *workflowTraceModel) sleep(id string, owner int) error {
	i := m.instances[id]
	if i == nil || i.terminal || owner != i.owner {
		return traceFailure("stale_owner")
	}
	i.status = "sleeping"
	// Retiring a context invalidates its old owner even before the next wake.
	i.owner++
	return nil
}

func (m *workflowTraceModel) complete(id string, owner int, output string) error {
	i := m.instances[id]
	if i == nil || i.terminal || (!m.mutant.acceptStaleOwner && owner != i.owner) {
		return traceFailure("stale_owner")
	}
	i.status, i.output, i.terminal = "complete", output, true
	i.owner++
	i.pending = nil
	return nil
}

func (m *workflowTraceModel) terminate(id string) error {
	i := m.instances[id]
	if i == nil {
		return traceFailure("unknown_instance")
	}
	if i.terminal {
		return nil
	}
	i.status, i.terminal = "terminated", true
	i.owner++
	i.pending = nil
	return nil
}

func expectTraceError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || err.Error() != code {
		t.Fatalf("error = %v, want %q", err, code)
	}
}

func TestWorkflowContractModelCreateAndEventAdmission(t *testing.T) {
	m := newWorkflowTraceModel(t, traceMutant{})
	if err := m.create("stable"); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, m.create("stable"), "instance_exists") // lost create reply
	if len(m.instances) != 1 || m.instances["stable"].status != "queued" {
		t.Fatal("duplicate create changed committed instance")
	}
	if err := m.send("stable", "approval", "e1", map[string]any{"ok": true}, 10); err != nil {
		t.Fatal(err)
	}
	if err := m.send("stable", "approval", "e1", map[string]any{"ok": true}, 11); err != nil {
		t.Fatal(err)
	}
	if len(m.instances["stable"].pending) != 1 {
		t.Fatal("keyed retry enqueued twice before consumption")
	}
	expectTraceError(t, m.send("stable", "approval", "e1", map[string]any{"ok": false}, 12), "event_conflict")
	accepted, err := m.wait("stable", "approval-step", "approval", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if replayed, err := m.wait("stable", "approval-step", "approval", 10, 11); err != nil || replayed != accepted {
		t.Fatalf("completed wait did not replay saved payload: %q, %v", replayed, err)
	}
	if err := m.send("stable", "approval", "e1", map[string]any{"ok": true}, 13); err != nil {
		t.Fatal(err)
	}
	if len(m.instances["stable"].pending) != 0 {
		t.Fatal("keyed retry enqueued after consumption")
	}
	if err := m.send("stable", "approval", "e2", map[string]any{"ok": false}, 13); err != nil {
		t.Fatal(err)
	}
	if replayed, err := m.wait("stable", "approval-step", "approval", 20, 20); err != nil || replayed != accepted || len(m.instances["stable"].pending) != 1 {
		t.Fatalf("replayed wait consumed a second event: %q, %v", replayed, err)
	}
	if err := m.terminate("stable"); err != nil {
		t.Fatal(err)
	}
	if err := m.send("stable", "approval", "e1", map[string]any{"ok": true}, 14); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, m.send("stable", "approval", "e3", map[string]any{"ok": true}, 14), "instance_terminal")
	m.expire("stable")
	if err := m.create("stable"); err != nil {
		t.Fatal(err)
	}
	if err := m.send("stable", "approval", "e1", map[string]any{"ok": true}, 20); err != nil {
		t.Fatal(err)
	}
	if len(m.instances["stable"].pending) != 1 {
		t.Fatal("old key incorrectly survived new instance generation")
	}
}

func TestWorkflowContractModelCapacityAndTimeoutBoundary(t *testing.T) {
	m := newWorkflowTraceModel(t, traceMutant{})
	if err := m.create("capacity"); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < m.pendingMax; index++ {
		if err := m.send("capacity", "later", fmt.Sprintf("event-%d", index), nil, 2); err != nil {
			t.Fatal(err)
		}
	}
	expectTraceError(t, m.send("capacity", "later", "overflow", nil, 2), "event_queue_full")
	if len(m.instances["capacity"].pending) != m.pendingMax {
		t.Fatal("capacity refusal changed inbox")
	}
	if err := m.send("capacity", "later", "event-0", nil, 2); err != nil {
		t.Fatalf("identical retry must win over capacity: %v", err)
	}
	if _, err := m.wait("capacity", "wait-late", "later", 1, 3); err == nil || err.Error() != "wait_timeout" {
		t.Fatalf("late event incorrectly won timeout: %v", err)
	}
	beforeReplay := len(m.instances["capacity"].pending)
	expectTraceError(t, func() error { _, err := m.wait("capacity", "wait-late", "later", 3, 3); return err }(), "wait_timeout")
	if len(m.instances["capacity"].pending) != beforeReplay {
		t.Fatal("timed-out step replay consumed a later event")
	}
	if _, err := m.wait("capacity", "wait-at-boundary", "later", 2, 3); err != nil {
		t.Fatalf("event accepted exactly at deadline must win: %v", err)
	}
	if err := m.create("waiting"); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, func() error { _, err := m.wait("waiting", "wait", "approval", 2, 1); return err }(), "wait_pending")
	if err := m.send("waiting", "approval", "event", nil, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := m.wait("waiting", "wait", "approval", 2, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.create("same-tick-after-timeout"); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, func() error { _, err := m.wait("same-tick-after-timeout", "wait", "approval", 2, 2); return err }(), "wait_timeout")
	if err := m.send("same-tick-after-timeout", "approval", "late-key", nil, 2); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, func() error { _, err := m.wait("same-tick-after-timeout", "wait", "approval", 2, 2); return err }(), "wait_timeout")
	if len(m.instances["same-tick-after-timeout"].pending) != 1 {
		t.Fatal("same-tick late event changed committed timeout")
	}
}

func TestWorkflowContractModelReplayVersionAndFence(t *testing.T) {
	m := newWorkflowTraceModel(t, traceMutant{})
	if err := m.create("replay"); err != nil {
		t.Fatal(err)
	}
	first, err := m.start("replay", "worker-v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.step("replay", first, "load", "order-1"); err != nil {
		t.Fatal(err)
	}
	if err := m.sleep("replay", first); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, m.step("replay", first, "charge", "paid"), "stale_owner")
	second, err := m.start("replay", "worker-v2")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.step("replay", second, "load", "changed-effect-would-be-wrong"); err != nil {
		t.Fatal(err)
	}
	if err := m.step("replay", second, "charge", "paid"); err != nil {
		t.Fatal(err)
	}
	if err := m.complete("replay", second, "done"); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, m.complete("replay", first, "stale"), "stale_owner")
	if i := m.instances["replay"]; i.version != "worker-v2" || i.steps["load"] != "order-1" || i.effects["load"] != 1 || i.effects["charge"] != 1 || i.status != "complete" || i.output != "done" {
		t.Fatalf("replay state/effects = %#v", i)
	}
	if err := m.terminate("replay"); err != nil {
		t.Fatal(err)
	}
	if m.instances["replay"].status != "complete" {
		t.Fatal("terminal terminate changed completed outcome")
	}
}

func TestWorkflowContractModelFailedCallbackAndNameCollision(t *testing.T) {
	m := newWorkflowTraceModel(t, traceMutant{})
	if err := m.create("bad-result"); err != nil {
		t.Fatal(err)
	}
	first, _ := m.start("bad-result", "worker-v1")
	expectTraceError(t, m.doAttempt("bad-result", first, "unsafe-result", false, 2), "retry_pending")
	second, _ := m.start("bad-result", "worker-v2")
	expectTraceError(t, m.doAttempt("bad-result", second, "unsafe-result", false, 2), "step_failed")
	expectTraceError(t, m.doAttempt("bad-result", second, "unsafe-result", true, 2), "step_failed")
	if i := m.instances["bad-result"]; i.effects["unsafe-result"] != 2 || i.attempts["unsafe-result"] != 2 || i.stepErrors["unsafe-result"] != "step_failed" {
		t.Fatalf("invalid callback result was not retried and journaled once: %#v", i)
	}
	if err := m.create("collision"); err != nil {
		t.Fatal(err)
	}
	owner, _ := m.start("collision", "worker-v1")
	expectTraceError(t, m.doAttempt("collision", owner, "shared-name", false, 2), "retry_pending")
	_, _ = m.start("collision", "worker-v2")
	expectTraceError(t, func() error { _, err := m.wait("collision", "shared-name", "approval", 10, 1); return err }(), "step_definition_mismatch")
	if i := m.instances["collision"]; i.status != "errored" || len(i.pending) != 0 || i.effects["shared-name"] != 1 {
		t.Fatalf("cross-kind pending name did not stop before new effects: %#v", i)
	}
}

func TestWorkflowContractModelTerminateWinsCompletion(t *testing.T) {
	m := newWorkflowTraceModel(t, traceMutant{})
	if err := m.create("race"); err != nil {
		t.Fatal(err)
	}
	owner, _ := m.start("race", "worker-v1")
	if err := m.terminate("race"); err != nil {
		t.Fatal(err)
	}
	expectTraceError(t, m.complete("race", owner, "too-late"), "stale_owner")
	if i := m.instances["race"]; i.status != "terminated" || i.output != "" {
		t.Fatalf("stale completion changed terminal state: %#v", i)
	}
}

func TestWorkflowContractModelMutantsAreRed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutant traceMutant
		probe  func(*workflowTraceModel) bool
	}{
		{"enqueue-before-dedupe", traceMutant{enqueueBeforeDedupe: true}, func(m *workflowTraceModel) bool {
			_ = m.create("i")
			_ = m.send("i", "event", "key", nil, 1)
			_ = m.send("i", "event", "key", nil, 2)
			return len(m.instances["i"].pending) == 1
		}},
		{"reexecute-finished", traceMutant{reexecuteFinished: true}, func(m *workflowTraceModel) bool {
			_ = m.create("i")
			owner, _ := m.start("i", "v1")
			_ = m.step("i", owner, "once", "saved")
			_ = m.step("i", owner, "once", "changed")
			return m.instances["i"].effects["once"] == 1
		}},
		{"accept-stale-owner", traceMutant{acceptStaleOwner: true}, func(m *workflowTraceModel) bool {
			_ = m.create("i")
			owner, _ := m.start("i", "v1")
			_ = m.sleep("i", owner)
			return m.step("i", owner, "late", "wrong") != nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.probe(newWorkflowTraceModel(t, tc.mutant)) {
				t.Fatal("deliberate contract mutant escaped the trace assertion")
			}
		})
	}
}
