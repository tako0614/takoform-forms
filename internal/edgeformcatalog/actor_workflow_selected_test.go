package edgeformcatalog

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tako0614/takoform/formpackage"
	coresnapshot "github.com/tako0614/takoform/snapshot"
)

func TestActorWorkflowSourceSelectsExactlyNineThreeTwoAndPreservesOtherBytes(t *testing.T) {
	baseForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	baseInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	baseBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Forms) != 17 || len(selected.Interfaces) != 8 || len(selected.Bindings) != 7 {
		t.Fatalf("selected counts = %d/%d/%d, want 17/8/7", len(selected.Forms), len(selected.Interfaces), len(selected.Bindings))
	}
	changedForms := 0
	for index, old := range baseForms {
		got := selected.Forms[index]
		if got.Kind != old.Kind || got.Slug != old.Slug {
			t.Fatalf("Form[%d] identity moved from %s/%s to %s/%s", index, old.Kind, old.Slug, got.Kind, got.Slug)
		}
		if !reflect.DeepEqual(old, got) {
			changedForms++
			if got.Definition.DefinitionVersion == old.Definition.DefinitionVersion {
				t.Fatalf("changed %s kept old version %s", old.Kind, got.Definition.DefinitionVersion)
			}
		}
	}
	if changedForms != 9 {
		t.Fatalf("changed %d Forms, want exactly nine and eight byte-exact retained", changedForms)
	}
	changedContracts := func(label string, old, next []RenderedContract, want int) {
		t.Helper()
		changed := 0
		for index, contract := range old {
			got := next[index]
			if got.Name != contract.Name {
				t.Fatalf("%s[%d] name moved from %s to %s", label, index, contract.Name, got.Name)
			}
			if !reflect.DeepEqual(contract, got) {
				changed++
				if got.Version == contract.Version || got.SchemaDigest == contract.SchemaDigest {
					t.Fatalf("changed %s %s retained old version or digest", label, got.Name)
				}
			}
		}
		if changed != want {
			t.Fatalf("changed %d %s, want %d and byte-exact retained remainder", changed, label, want)
		}
	}
	changedContracts("Interfaces", baseInterfaces, selected.Interfaces, 3)
	changedContracts("Bindings", baseBindings, selected.Bindings, 2)
	if !reflect.DeepEqual(baseForms, mustRenderCurrentForms(t)) {
		t.Fatal("source selection mutated the legacy current catalog")
	}
}

func TestActorWorkflowSourceKeepsBroaderRuntimeExperimentOnDevelopmentVersions(t *testing.T) {
	broader, err := RenderRuntimeCandidate()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := renderActorWorkflowSourceCandidate()
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		name string
		old  RenderedForm
		new  RenderedForm
	}{
		{"ModuleWorker", broader.ModuleWorker, selected.ModuleWorker},
		{"ActorNamespace", broader.Actor.Form, selected.Actor.Form},
		{"DurableWorkflow", broader.Workflow.Form, selected.Workflow.Form},
		{"WorkerVersion", broader.WorkerVersion, selected.WorkerVersion},
		{"WorkerDeployment", broader.WorkerDeployment, selected.WorkerDeployment},
	} {
		if pair.old.Definition.DefinitionVersion == pair.new.Definition.DefinitionVersion {
			t.Fatalf("%s development and selected source versions collide", pair.name)
		}
	}
	if broader.WorkerVersion.Definition.DefinitionVersion != RuntimeCandidateWorkerVersionVersion ||
		broader.WorkerDeployment.Definition.DefinitionVersion != RuntimeCandidateWorkerDeploymentVersion {
		t.Fatal("broader runtime experiment lost its old development versions")
	}
}

func TestActorWorkflowSelectedWorkerVersionProseNamesItsExactRuntime(t *testing.T) {
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	var workerVersion RenderedForm
	for _, form := range selected.Forms {
		if form.Kind == "WorkerVersion" {
			workerVersion = form
			break
		}
	}
	if workerVersion.Kind == "" {
		t.Fatal("selected WorkerVersion missing")
	}
	properties, ok := workerVersion.Definition.DesiredSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("selected WorkerVersion properties missing")
	}
	handlers, ok := properties["handlers"].(map[string]any)
	if !ok {
		t.Fatal("selected WorkerVersion handlers missing")
	}
	want := WorkerRuntimeInterfaceName + "@" + RuntimeCandidateInterfaceVersion
	for label, prose := range map[string]string{
		"description":               workerVersion.Definition.Description,
		"desiredSchema.description": workerVersion.Definition.DesiredSchema["description"].(string),
		"handlers.description":      handlers["description"].(string),
	} {
		if !strings.Contains(prose, want) || strings.Contains(prose, "worker.runtime@1.1.0") {
			t.Errorf("%s must name exact selected runtime %s without stale 1.1.0: %q", label, want, prose)
		}
	}
}

func TestActorWorkflowSelectedInterfacesCarryExactJavaScriptActorFacade(t *testing.T) {
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	var actor, runtime RenderedContract
	for _, contract := range selected.Interfaces {
		switch contract.Name {
		case ActorCandidateInterfaceName:
			actor = contract
		case RuntimeCandidateInterfaceName:
			runtime = contract
		}
	}
	if actor.Name == "" || runtime.Name == "" {
		t.Fatalf("selected Actor/runtime Interfaces missing: actor=%q runtime=%q", actor.Name, runtime.Name)
	}
	actorWant := []string{
		"type EdgeSqlValue = null | number | string | { encoding: \"base64\"; data: string };",
		"interface SqlResult { readonly rows: readonly Readonly<Record<string, EdgeSqlValue>>[]; readonly rowsWritten: number; }",
		"ActorStorage.execute(sql: string, params?: readonly EdgeSqlValue[]): Promise<SqlResult>.",
		"ActorStorage.query(sql: string, params?: readonly EdgeSqlValue[]): Promise<SqlResult>.",
		"interface SqlStatement { readonly sql: string; readonly params?: readonly EdgeSqlValue[]; }",
		"ActorStorage.transaction(statements: readonly SqlStatement[]): Promise<{ readonly results: readonly SqlResult[]; }>.",
		"ActorAlarm.set(atMillis: number): Promise<void>.",
		"ActorAlarm.get(): Promise<number | null>.",
		"ActorAlarm.clear(): Promise<void>.",
	}
	runtimeWant := []string{
		"interface ActorSocket {",
		"readonly id: string;",
		"send(data: string | Uint8Array): void;",
		"close(code?: number, reason?: string): void;",
		"getAttachment(): Promise<Uint8Array | null>;",
		"setAttachment(value: Uint8Array | null): Promise<void>;",
		"interface ActorSocketErrorEvent { readonly code: \"transport_error\"; }",
		"interface ActorUpgradeResponse extends Response { readonly status: 101; readonly body: null; }",
		"interface ActorSockets {",
		"interface ActorTurn { readonly signal: AbortSignal; }",
		"interface ActorContext { readonly id: string; readonly storage: ActorStorage; readonly alarm: ActorAlarm; readonly sockets: ActorSockets; }",
		"accept(",
		"options?: { protocol?: string; attachment?: Uint8Array },",
		"): Promise<{ response: ActorUpgradeResponse; socket: ActorSocket }>;",
		"get(id: string): Promise<ActorSocket | null>;",
		"list(): Promise<readonly ActorSocket[]>;",
		"interface ActorInstance {",
		"start?(turn: ActorTurn): void | Promise<void>;",
		"fetch(request: Request, turn: ActorTurn): Response | Promise<Response>;",
		"alarm(turn: ActorTurn): void | Promise<void>;",
		"socketMessage(socket: ActorSocket, data: string | Uint8Array, turn: ActorTurn): void | Promise<void>;",
		"socketClose(socket: ActorSocket, event: { code: number; reason: string; wasClean: boolean }, turn: ActorTurn): void | Promise<void>;",
		"socketError(socket: ActorSocket, event: ActorSocketErrorEvent, turn: ActorTurn): void | Promise<void>;",
		"type ActorConstructor = new ( context: ActorContext, env: Readonly<Record<string, unknown>>, ) => ActorInstance;",
	}
	descriptions := func(contract RenderedContract) string {
		var definition struct {
			Description string `json:"description"`
			Operations  []struct {
				Description string `json:"description"`
			} `json:"operations"`
		}
		if err := json.Unmarshal([]byte(contract.DefinitionJSON), &definition); err != nil {
			t.Fatalf("decode %s@%s: %v", contract.Name, contract.Version, err)
		}
		all := []string{definition.Description}
		for _, operation := range definition.Operations {
			all = append(all, operation.Description)
		}
		return strings.Join(strings.Fields(strings.Join(all, " ")), " ")
	}
	for _, fragment := range actorWant {
		if !strings.Contains(descriptions(actor), fragment) {
			t.Errorf("worker.actor@%s JavaScript facade mapping omitted %q", actor.Version, fragment)
		}
	}
	for _, fragment := range runtimeWant {
		if !strings.Contains(descriptions(runtime), fragment) {
			t.Errorf("worker.runtime@%s JavaScript facade mapping omitted %q", runtime.Version, fragment)
		}
	}
}

func TestActorWorkflowSelectedSourceClosesOneCoreSnapshot(t *testing.T) {
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	base, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	input := coresnapshot.Input{HostAPI: "forms.takoform.com/v1"}
	for index, form := range selected.Forms {
		var pkg formpackage.VerifiedPackage
		if reflect.DeepEqual(form, base[index]) {
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
			Ref: formpackage.FormRef{APIVersion: ref.APIVersion, Kind: ref.Kind, DefinitionVersion: ref.DefinitionVersion, SchemaDigest: ref.SchemaDigest},
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
		t.Fatalf("selected source did not close Core Snapshot: snapshot=%v diagnostics=%+v", compiled, diagnostics)
	}
	if len(compiled.Forms()) != 17 {
		t.Fatalf("selected source compiled %d Forms, want exact 17", len(compiled.Forms()))
	}
}

func mustRenderCurrentForms(t *testing.T) []RenderedForm {
	t.Helper()
	forms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	return forms
}

func TestActorWorkflowSourceRejectsMissingDuplicateAndWrongSlug(t *testing.T) {
	base := []RenderedForm{{Kind: "ModuleWorker", Slug: "module-worker"}}
	if _, err := replaceSelectedForms(base, []RenderedForm{{Kind: "ModuleWorker", Slug: "module-worker"}}); err == nil {
		t.Fatal("short Form roster accepted")
	}
	contracts := []RenderedContract{{Name: "worker.runtime", Version: "1.1.0"}}
	for _, replacements := range [][]RenderedContract{
		{{Name: "missing", Version: "2.0.0"}},
		{{Name: "worker.runtime", Version: "2.0.0"}, {Name: "worker.runtime", Version: "2.0.0"}},
	} {
		if _, err := replaceSelectedContracts(contracts, replacements); err == nil {
			t.Fatalf("invalid contract replacements accepted: %+v", replacements)
		}
	}
	current, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	replacements := append([]RenderedForm(nil), current[:9]...)
	replacements[0].Slug += "-wrong"
	if _, err := replaceSelectedForms(current, replacements); err == nil || !strings.Contains(err.Error(), "wrong-slug") {
		t.Fatalf("wrong-slug replacement error = %v", err)
	}
}
