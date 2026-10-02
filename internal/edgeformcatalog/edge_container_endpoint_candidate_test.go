package edgeformcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
	coresnapshot "github.com/tako0614/takoform/snapshot"
)

func TestRenderEdgeContainerEndpointCandidateEmitsCoreVerifiedUnpublishedClosure(t *testing.T) {
	beforeForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	beforeInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	beforeBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RenderEdgeContainerEndpointFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertContainerEndpointCandidateStatus(candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.Form.Definition.Kind != EdgeContainerEndpointCandidateFormKind ||
		candidate.Form.Definition.DefinitionVersion != EdgeContainerEndpointCandidateFormVersion {
		t.Fatalf("candidate identity = %s@%s", candidate.Form.Definition.Kind, candidate.Form.Definition.DefinitionVersion)
	}
	if _, exists := ByKind(EdgeContainerEndpointCandidateFormKind); exists {
		t.Fatal("ContainerEndpoint was added to the registered current catalog")
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.Form.DefinitionJSON)); err != nil {
		t.Fatalf("released Core rejected the Endpoint Definition: %v", err)
	}
	if _, err := formpackage.ValidatePackageIndex([]byte(candidate.Package.IndexJSON)); err != nil {
		t.Fatalf("released Core rejected the Endpoint package index: %v", err)
	}
	if got, want := candidate.Form.Definition.LifecycleCapabilities, []string{"create", "read", "delete", "observe"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycleCapabilities = %v, want exact no-update/no-import set %v", got, want)
	}
	if candidate.Form.Definition.Role != "attachment" {
		t.Fatalf("candidate role = %q, want attachment", candidate.Form.Definition.Role)
	}
	if len(candidate.Form.Definition.ProvidedInterfaces) != 0 || len(candidate.Form.Definition.AcceptedBindings) != 0 {
		t.Fatalf("candidate unexpectedly claims Interfaces or Bindings: provided=%v accepted=%v",
			candidate.Form.Definition.ProvidedInterfaces, candidate.Form.Definition.AcceptedBindings)
	}
	if got, want := candidate.Form.Definition.ImmutableFields, []string{"/service"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("immutableFields = %v, want %v", got, want)
	}
	wantConstraints := []formpackage.FormConstraint{
		{Kind: "exclusive", Reference: "/service"},
		{Kind: "hostAssigned", Output: "/hostname"},
		{Kind: "hostAssigned", Output: "/url"},
	}
	if !reflect.DeepEqual(candidate.Form.Definition.Constraints, wantConstraints) {
		t.Fatalf("resource constraints = %#v, want exclusive service and both assigned outputs %#v",
			candidate.Form.Definition.Constraints, wantConstraints)
	}
	if got, want := len(candidate.Package.Files), len(candidate.Form.Fixtures)+1; got != want {
		t.Fatalf("package payload count = %d, want %d (one Definition plus fixtures)", got, want)
	}
	verifiedEndpoint := verifyRenderedCandidatePackage(t, candidate.Package)
	if got, want := verifiedEndpoint.PackageDigest(), candidate.Package.PackageDigest; got != want {
		t.Fatalf("Core verified package digest = %s, renderer digest = %s", got, want)
	}

	serviceCandidate, err := RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	serviceSchemaDigest, err := formpackage.DigestCanonicalJSON([]byte(serviceCandidate.Form.DefinitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	if serviceSchemaDigest != EdgeContainerServiceCandidateSchemaDigest ||
		serviceCandidate.Package.PackageDigest != EdgeContainerServiceCandidatePackageDigest {
		t.Fatalf("frozen service input drifted: schema=%s package=%s", serviceSchemaDigest, serviceCandidate.Package.PackageDigest)
	}
	verifiedService := verifyRenderedCandidatePackage(t, serviceCandidate.Package)

	formRef := candidateFormRef(t, candidate.Form.DefinitionJSON)
	snapshot, diagnostics := coresnapshot.Compile(coresnapshot.Input{
		HostAPI: "forms.takoform.com/v1",
		Packages: []coresnapshot.PackageArtifact{
			{Origin: "frozen-container-service-0.1.0", ExpectedDigest: EdgeContainerServiceCandidatePackageDigest, Package: verifiedService},
			{Origin: "unpublished-container-endpoint-0.1.0", ExpectedDigest: candidate.Package.PackageDigest, Package: verifiedEndpoint},
		},
		DefaultCreates: []coresnapshot.DefaultPin{
			{Group: Family.APIVersion(), Kind: EdgeContainerServiceCandidateFormKind, Ref: containerEndpointPackageTargetRef()},
			{Group: Family.APIVersion(), Kind: EdgeContainerEndpointCandidateFormKind, Ref: formRef},
		},
	})
	if len(diagnostics) != 0 || snapshot == nil {
		t.Fatalf("Core did not compile the exact two-package Snapshot: snapshot=%v diagnostics=%+v", snapshot, diagnostics)
	}
	if got := len(snapshot.Forms()); got != 2 {
		t.Fatalf("exact service/endpoint Snapshot contains %d Forms, want 2", got)
	}
	if _, found := snapshot.Definition(formRef); !found {
		t.Fatal("Core Snapshot omitted the exact ContainerEndpoint Definition")
	}

	afterForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	afterInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	afterBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeForms, afterForms) || len(afterForms) != 17 {
		t.Fatalf("rendering unpublished Endpoint changed the 17 current Forms: before=%d after=%d", len(beforeForms), len(afterForms))
	}
	if !reflect.DeepEqual(beforeInterfaces, afterInterfaces) || len(afterInterfaces) != 8 {
		t.Fatalf("rendering unpublished Endpoint changed the 8 current Interfaces: before=%d after=%d", len(beforeInterfaces), len(afterInterfaces))
	}
	if !reflect.DeepEqual(beforeBindings, afterBindings) || len(afterBindings) != 7 {
		t.Fatalf("rendering unpublished Endpoint changed the 7 current Bindings: before=%d after=%d", len(beforeBindings), len(afterBindings))
	}

	var badDesired map[string]any
	badDesiredJSON, err := json.Marshal(candidate.Form.Fixtures["negative-service.json"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(badDesiredJSON, &badDesired); err != nil {
		t.Fatalf("parse wrong-target fixture: %v", err)
	}
	if err := snapshot.Validate(formRef, badDesiredJSON); err == nil {
		t.Fatal("Core Snapshot accepted the wrong-target desired fixture")
	}
	if !strings.Contains(candidate.Form.Definition.Description, "same name") {
		t.Fatal("Definition does not state the new-UID same-name no-retarget boundary")
	}
}

func TestContainerEndpointDesiredAndOutputsSchemasRejectNegativeFixtures(t *testing.T) {
	candidate, err := RenderEdgeContainerEndpointFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	desiredSchema := compileContainerCandidateDesiredSchema(t, candidate.Form.Definition.DesiredSchema)
	if err := desiredSchema.Validate(candidate.Form.Fixtures["desired.json"]); err != nil {
		t.Fatalf("canonical desired fixture rejected: %v", err)
	}
	for name, fixture := range candidate.Form.Fixtures {
		if !strings.HasPrefix(name, "negative-") {
			continue
		}
		if err := desiredSchema.Validate(fixture); err == nil {
			t.Errorf("negative desired fixture %q passed the emitted schema: %#v", name, fixture)
		}
	}
	serviceProperty := candidate.Form.Definition.DesiredSchema["properties"].(map[string]any)["service"].(map[string]any)
	targetRefs, ok := serviceProperty[model.TargetFormRefsAnnotationKey].([]any)
	if !ok || len(targetRefs) != 1 {
		t.Fatalf("service exact FormRef annotation = %#v", serviceProperty[model.TargetFormRefsAnnotationKey])
	}
	var target map[string]any
	encoded, _ := json.Marshal(targetRefs[0])
	if err := json.Unmarshal(encoded, &target); err != nil {
		t.Fatal(err)
	}
	if target["apiVersion"] != Family.APIVersion() ||
		target["kind"] != EdgeContainerServiceCandidateFormKind ||
		target["definitionVersion"] != EdgeContainerServiceCandidateFormVersion ||
		target["schemaDigest"] != EdgeContainerServiceCandidateSchemaDigest {
		t.Fatalf("service target is not the exact frozen FormRef: %#v", target)
	}

	outputSchema := compileContainerEndpointOutputSchema(t, candidate.Form.Definition.OutputSchema)
	completeOutputs := map[string]any{
		"hostname": "api.container.example.invalid",
		"url":      "https://api.container.example.invalid/",
	}
	if err := outputSchema.Validate(completeOutputs); err != nil {
		t.Fatalf("canonical HTTPS output fixture rejected: %v", err)
	}
	if strings.TrimPrefix(completeOutputs["url"].(string), "https://") != completeOutputs["hostname"].(string)+"/" {
		t.Fatal("canonical output fixture URL does not equal HTTPS root of assigned hostname")
	}
	for _, incomplete := range []map[string]any{
		{"hostname": completeOutputs["hostname"]},
		{"url": completeOutputs["url"]},
		{},
	} {
		if err := outputSchema.Validate(incomplete); err == nil {
			t.Errorf("missing-output fixture passed OutputSchema: %#v", incomplete)
		}
	}
	for _, invalidURL := range []string{
		"http://api.container.example.invalid/",
		"https://api.container.example.invalid:443/",
		"https://api.container.example.invalid/path",
		"https://UPPER.container.example.invalid/",
	} {
		if err := outputSchema.Validate(map[string]any{"hostname": completeOutputs["hostname"], "url": invalidURL}); err == nil {
			t.Errorf("invalid endpoint URL passed OutputSchema: %q", invalidURL)
		}
	}

	fixtures := map[string]ContainerEndpointLifecycleFixture{}
	for _, fixture := range candidate.LocalLifecycleFixtures {
		fixtures[fixture.Name] = fixture
	}
	for _, name := range []string{
		"healthy-service-generation-routes",
		"same-name-new-service-uid-does-not-retarget",
		"service-delete-with-live-endpoint-is-refused",
		"missing-host-assigned-output-is-refused",
		"wrong-target-or-schema-digest-is-unresolved",
	} {
		if _, found := fixtures[name]; !found {
			t.Errorf("missing local lifecycle fixture %q", name)
		}
	}
	if !fixtures["same-name-new-service-uid-does-not-retarget"].Negative ||
		!fixtures["missing-host-assigned-output-is-refused"].Negative {
		t.Fatal("UID retarget and missing-output scenarios must be negative cases")
	}
}

func TestContainerEndpointSnapshotRejectsWrongPinnedServiceDigestWithoutPartialResult(t *testing.T) {
	endpoint, err := RenderEdgeContainerEndpointFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	service, err := RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	verifiedService := verifyRenderedCandidatePackage(t, service.Package)
	wrongDigestPackage := containerEndpointPackageWithTargetSchemaDigest(t, endpoint, "sha256:"+strings.Repeat("0", 64))
	verifiedWrongEndpoint := verifyRenderedCandidatePackage(t, wrongDigestPackage)
	var wrongIndex formpackage.PackageIndex
	if err := json.Unmarshal([]byte(wrongDigestPackage.IndexJSON), &wrongIndex); err != nil {
		t.Fatal(err)
	}

	snapshot, diagnostics := coresnapshot.Compile(coresnapshot.Input{
		HostAPI: "forms.takoform.com/v1",
		Packages: []coresnapshot.PackageArtifact{
			{Origin: "frozen-container-service-0.1.0", ExpectedDigest: EdgeContainerServiceCandidatePackageDigest, Package: verifiedService},
			{Origin: "endpoint-with-wrong-service-schema-digest", ExpectedDigest: wrongDigestPackage.PackageDigest, Package: verifiedWrongEndpoint},
		},
		DefaultCreates: []coresnapshot.DefaultPin{
			{Group: Family.APIVersion(), Kind: EdgeContainerServiceCandidateFormKind, Ref: containerEndpointPackageTargetRef()},
			{Group: Family.APIVersion(), Kind: EdgeContainerEndpointCandidateFormKind, Ref: wrongIndex.FormRef},
		},
	})
	if snapshot != nil {
		t.Fatal("Core returned a partial Snapshot despite the wrong target schema digest")
	}
	foundUnresolved := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == coresnapshot.DiagnosticUnresolvedReference {
			foundUnresolved = true
		}
	}
	if !foundUnresolved {
		t.Fatalf("wrong target digest did not produce an unresolved-reference diagnostic: %+v", diagnostics)
	}
}

func candidateFormRef(t *testing.T, definitionJSON string) formpackage.FormRef {
	t.Helper()
	definition, err := formpackage.ValidateDefinition([]byte(definitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := formpackage.DigestCanonicalJSON([]byte(definitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	return formpackage.FormRef{
		APIVersion: definition.APIVersion, Kind: definition.Kind,
		DefinitionVersion: definition.DefinitionVersion, SchemaDigest: digest,
	}
}

func verifyRenderedCandidatePackage(t *testing.T, packageClosure RenderedFormPackage) formpackage.VerifiedPackage {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, formpackage.PackageIndexFilename), []byte(packageClosure.IndexJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, file := range packageClosure.Files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := formpackage.VerifyDirectory(root)
	if err != nil {
		t.Fatalf("Core rejected materialized candidate package: %v", err)
	}
	verified, ok := report.VerifiedPackage()
	if !ok {
		t.Fatal("Core verification report did not issue a VerifiedPackage")
	}
	return verified
}

func compileContainerEndpointOutputSchema(t *testing.T, schema map[string]any) *jsonschema.Schema {
	t.Helper()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	const id = "https://candidate.test/container-endpoint-outputs.json"
	if err := compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func containerEndpointPackageWithTargetSchemaDigest(
	t *testing.T,
	candidate EdgeContainerEndpointFormCandidate,
	digest string,
) RenderedFormPackage {
	t.Helper()
	var definition map[string]any
	if err := json.Unmarshal([]byte(candidate.Form.DefinitionJSON), &definition); err != nil {
		t.Fatal(err)
	}
	desired := definition["desiredSchema"].(map[string]any)
	properties := desired["properties"].(map[string]any)
	service := properties["service"].(map[string]any)
	targets := service[model.TargetFormRefsAnnotationKey].([]any)
	target := targets[0].(map[string]any)
	target["schemaDigest"] = digest
	definitionJSON, err := marshalIndented(definition)
	if err != nil {
		t.Fatal(err)
	}
	form := candidate.Form
	form.DefinitionJSON = definitionJSON
	return mustRenderContainerFormPackage(t, form)
}

func mustRenderContainerFormPackage(t *testing.T, form RenderedForm) RenderedFormPackage {
	t.Helper()
	packageClosure, err := renderContainerFormPackage(form)
	if err != nil {
		t.Fatal(err)
	}
	return packageClosure
}
