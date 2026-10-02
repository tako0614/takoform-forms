package edgeformcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tako0614/takoform/formpackage"
)

func TestRenderEdgeContainerServiceFormCandidateEmitsCoreVerifiedUnpublishedClosure(t *testing.T) {
	candidate, err := RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if candidate.PublicationStatus != "UNPUBLISHED" {
		t.Fatalf("publicationStatus = %q, want UNPUBLISHED", candidate.PublicationStatus)
	}
	if !strings.HasPrefix(candidate.ContractClosure, "form-only;") {
		t.Fatalf("contractClosure = %q, want explicit form-only boundary", candidate.ContractClosure)
	}
	if _, exists := ByKind(EdgeContainerHTTPCandidateFormKind); exists {
		t.Fatal("ContainerService was inserted into the registered current catalog")
	}
	if candidate.Form.Definition.Kind != EdgeContainerHTTPCandidateFormKind ||
		candidate.Form.Definition.DefinitionVersion != EdgeContainerHTTPCandidateFormVersion {
		t.Fatalf("unexpected candidate Form identity: %s@%s", candidate.Form.Definition.Kind, candidate.Form.Definition.DefinitionVersion)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.Form.DefinitionJSON)); err != nil {
		t.Fatalf("Core rejected rendered Form Definition: %v", err)
	}
	desiredSchema := compileContainerCandidateDesiredSchema(t, candidate.Form.Definition.DesiredSchema)
	if err := desiredSchema.Validate(candidate.Form.Fixtures["desired.json"]); err != nil {
		t.Fatalf("canonical positive fixture rejected: %v", err)
	}
	for name, fixture := range candidate.Form.Fixtures {
		if strings.HasPrefix(name, "negative-") {
			if err := desiredSchema.Validate(fixture); err == nil {
				t.Errorf("negative fixture %q was accepted by its emitted DesiredSchema", name)
			}
		}
	}

	var definition formpackage.FormDefinition
	if err := json.Unmarshal([]byte(candidate.Form.DefinitionJSON), &definition); err != nil {
		t.Fatalf("parse Form Definition: %v", err)
	}
	if len(definition.ProvidedInterfaces) != 0 {
		t.Fatalf("Form-only candidate unexpectedly claims provided Interfaces: %+v", definition.ProvidedInterfaces)
	}
	if got, want := definition.LifecycleCapabilities, []string{"create", "read", "update", "delete", "observe"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate lifecycle capabilities = %v, want explicitly scoped CRUD/observe %v", got, want)
	}
	var index formpackage.PackageIndex
	if _, err := formpackage.ValidatePackageIndex([]byte(candidate.Package.IndexJSON)); err != nil {
		t.Fatalf("Core rejected rendered package index: %v", err)
	}
	if err := json.Unmarshal([]byte(candidate.Package.IndexJSON), &index); err != nil {
		t.Fatalf("parse package index: %v", err)
	}
	formRef := index.FormRef
	if formRef.Kind != EdgeContainerHTTPCandidateFormKind || formRef.DefinitionVersion != EdgeContainerHTTPCandidateFormVersion {
		t.Fatalf("package FormRef = %+v", formRef)
	}
	if formRef.APIVersion != definition.APIVersion {
		t.Fatalf("package API version = %q, Definition = %q", formRef.APIVersion, definition.APIVersion)
	}
	if len(index.Files) != len(candidate.Package.Files) || len(index.Files) < 3 {
		t.Fatalf("package index files = %d, exported content files = %d", len(index.Files), len(candidate.Package.Files))
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, formpackage.PackageIndexFilename), []byte(candidate.Package.IndexJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, file := range candidate.Package.Files {
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
		t.Fatalf("Core could not verify exported package closure: %v", err)
	}
	if report.PackageDigest != candidate.Package.PackageDigest || report.FormRef != formRef {
		t.Fatalf("Core readback = %+v, candidate digest/ref = %s / %+v", report, candidate.Package.PackageDigest, formRef)
	}
	for _, required := range []string{
		"fixtures/negative-image.json",
		"fixtures/negative-http-port.json",
		"fixtures/negative-http-port-out-of-range.json",
		"fixtures/negative-environment.json",
		"fixtures/negative-sensitive-requirement-values-not-names.json",
		"fixtures/negative-environment-value-too-long.json",
		"fixtures/negative-registry-port-out-of-range.json",
		"fixtures/negative-health-path-empty-segment.json",
		"fixtures/negative-required-sensitive-vars.json",
	} {
		found := false
		for _, file := range candidate.Package.Files {
			if file.Path == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Core-verified package omitted negative fixture %q", required)
		}
	}
}

func TestContainerEnvironmentSecretClassificationIsCallerResponsibility(t *testing.T) {
	candidate, err := RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	schema := compileContainerCandidateDesiredSchema(t, candidate.Form.Definition.DesiredSchema)
	desired := candidate.Form.Fixtures["desired.json"]
	desired["environment"] = map[string]any{"API_TOKEN": "example-not-a-real-secret"}
	if err := schema.Validate(desired); err != nil {
		t.Fatalf("schema unexpectedly claims to detect/classify environment values: %v", err)
	}
	if !strings.Contains(candidate.Form.Definition.Description, "Callers MUST NOT put secrets in this field") ||
		!strings.Contains(candidate.Form.Definition.Description, "does not automatically detect or reject secret values") {
		t.Fatal("Form Description must state caller responsibility and lack of automatic secret detection")
	}
}

func compileContainerCandidateDesiredSchema(t *testing.T, schema map[string]any) *jsonschema.Schema {
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
	const id = "https://candidate.test/container-service-desired.json"
	if err := compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestContainerSensitiveRequirementsFailClosedWithoutHostSecretInput(t *testing.T) {
	for name, desired := range map[string]map[string]any{
		"omitted":     {},
		"empty":       {"requiredSensitiveVars": []any{}},
		"empty typed": {"requiredSensitiveVars": []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateContainerSensitiveRequirements(desired); err != nil {
				t.Fatalf("empty sensitive requirement refused: %v", err)
			}
		})
	}
	for name, values := range map[string]any{
		"one name":    []any{"API_TOKEN"},
		"typed names": []string{"API_TOKEN", "DATABASE_PASSWORD"},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateContainerSensitiveRequirements(map[string]any{"requiredSensitiveVars": values})
			if err == nil || !strings.Contains(err.Error(), "no secret-material input") {
				t.Fatalf("sensitive requirements error = %v", err)
			}
		})
	}
	if err := ValidateContainerSensitiveRequirements(map[string]any{"requiredSensitiveVars": "API_TOKEN"}); err == nil {
		t.Fatal("malformed requiredSensitiveVars was accepted")
	}
}

func TestEdgeContainerCandidatePublishesLocalLifecycleScenariosNotSupportClaims(t *testing.T) {
	candidate, err := RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]ContainerLifecycleFixture{}
	for _, fixture := range candidate.LocalLifecycleFixtures {
		if _, duplicate := fixtures[fixture.Name]; duplicate {
			t.Fatalf("duplicate local lifecycle fixture %q", fixture.Name)
		}
		fixtures[fixture.Name] = fixture
	}
	for _, name := range []string{
		"healthy-update-cuts-over",
		"health-failure-preserves-old-serving",
		"lost-ack-mismatched-readback-is-not-retried",
		"delete-stale-incarnation-is-refused",
		"restart-recovers-from-durable-operation-record",
	} {
		if _, found := fixtures[name]; !found {
			t.Errorf("missing local lifecycle scenario %q", name)
		}
	}
	if !fixtures["health-failure-preserves-old-serving"].Negative || !fixtures["lost-ack-mismatched-readback-is-not-retried"].Negative || !fixtures["delete-stale-incarnation-is-refused"].Negative {
		t.Fatal("failure and mismatch lifecycle cases must be marked negative")
	}
	if !strings.Contains(candidate.Form.Definition.Description, "Host Offering-owned capacity and placement") ||
		!strings.Contains(candidate.Form.Definition.Description, "persisted desired/observed generations") ||
		!strings.Contains(candidate.Form.Definition.Description, "container.http and its Worker Binding remain a separate unresolved contract gap") {
		t.Fatal("candidate omitted an ownership boundary or blurred the withheld HTTP contract")
	}
}
