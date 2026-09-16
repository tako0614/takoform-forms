package edgeformcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tako0614/takoform/formpackage"
)

func TestVectorIndexCandidateRendersAndValidatesWithReleasedCore(t *testing.T) {
	t.Parallel()
	candidate, err := RenderVectorIndexCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(candidate.Form.DefinitionJSON)); err != nil {
		t.Fatalf("candidate Form rejected by released Core: %v", err)
	}
	if err := formpackage.ValidateInterfaceDefinition([]byte(candidate.Interface.DefinitionJSON)); err != nil {
		t.Fatalf("candidate Interface rejected by released Core: %v", err)
	}
	if candidate.Form.Kind != VectorIndexCandidateFormKind || candidate.Form.Slug != VectorIndexCandidateFormSlug {
		t.Fatalf("candidate Form identity = %s/%s", candidate.Form.Kind, candidate.Form.Slug)
	}
	if got := candidate.Form.Definition.DefinitionVersion; got != VectorIndexCandidateFormVersion {
		t.Fatalf("Form definitionVersion = %q, want %q", got, VectorIndexCandidateFormVersion)
	}
	if got := candidate.Interface.Name; got != VectorIndexCandidateInterfaceName {
		t.Fatalf("Interface name = %q, want %q", got, VectorIndexCandidateInterfaceName)
	}
	if got := candidate.Interface.Version; got != VectorIndexCandidateInterfaceVersion {
		t.Fatalf("Interface version = %q, want %q", got, VectorIndexCandidateInterfaceVersion)
	}
	if got, want := candidate.Form.Definition.LifecycleCapabilities, []string{"create", "read", "delete", "observe"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycleCapabilities = %#v, want %#v", got, want)
	}
	if candidate.Form.Definition.OutputSchema != nil {
		t.Fatalf("candidate claims status outputs: %#v", candidate.Form.Definition.OutputSchema)
	}
	if candidate.Form.Definition.ObservedSchema == nil {
		t.Fatal("candidate has no observed schema")
	}
	observedProperties, _ := candidate.Form.Definition.ObservedSchema["properties"].(map[string]any)
	for _, name := range []string{"dimension", "metric", "filterKeys", "count"} {
		if _, ok := observedProperties[name]; !ok {
			t.Fatalf("observed schema is missing %s: %#v", name, candidate.Form.Definition.ObservedSchema)
		}
	}
	if len(candidate.Form.Definition.AcceptedBindings) != 0 {
		t.Fatalf("candidate claims bindings: %#v", candidate.Form.Definition.AcceptedBindings)
	}
	if got := len(candidate.Interface.DefinitionJSON); got == 0 {
		t.Fatal("candidate Interface has no rendered definition")
	}
	if got := candidate.Form.Definition.ProvidedInterfaces; len(got) != 1 {
		t.Fatalf("providedInterfaces = %#v, want one candidate Interface", got)
	} else {
		ref := got[0]
		if ref.APIVersion != InterfaceAPIVersion || ref.Name != candidate.Interface.Name ||
			ref.Version != candidate.Interface.Version || ref.SchemaDigest != candidate.Interface.SchemaDigest {
			t.Fatalf("provided InterfaceRef = %#v, want exact rendered candidate digest", ref)
		}
	}
	if _, known := ByKind(VectorIndexCandidateFormKind); known {
		t.Fatal("candidate Form was registered in current Forms")
	}
	for _, definition := range InterfaceDefinitions() {
		if definition.Name == VectorIndexCandidateInterfaceName {
			t.Fatal("candidate Interface was registered in current InterfaceDefinitions")
		}
	}
}

func TestVectorIndexCandidateLeavesCurrentRenderedCatalogUnchanged(t *testing.T) {
	t.Parallel()
	beforeForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	beforeInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(beforeForms), 17; got != want {
		t.Fatalf("current rendered Form count = %d, want %d", got, want)
	}
	if got, want := len(beforeInterfaces), 8; got != want {
		t.Fatalf("current rendered Interface count = %d, want %d", got, want)
	}
	if _, err := RenderVectorIndexCandidate(); err != nil {
		t.Fatal(err)
	}
	afterForms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	afterInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeForms, afterForms) {
		t.Fatal("rendering the unpublished candidate changed current rendered Forms")
	}
	if !reflect.DeepEqual(beforeInterfaces, afterInterfaces) {
		t.Fatal("rendering the unpublished candidate changed current rendered Interfaces")
	}
}

func TestVectorIndexCandidateFormSchemaFixtures(t *testing.T) {
	t.Parallel()
	candidate, err := RenderVectorIndexCandidate()
	if err != nil {
		t.Fatal(err)
	}
	schema := compileVectorIndexSchema(t, candidate.Form.Definition.DesiredSchema)
	valid := candidate.Form.Fixtures["desired.json"]
	if want := map[string]any{
		"dimension":  3,
		"metric":     "cosine",
		"filterKeys": []any{"spaceId"},
	}; !reflect.DeepEqual(valid, want) {
		t.Fatalf("canonical desired fixture = %#v, want fresh fixture filter key spaceId: %#v", valid, want)
	}
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("canonical desired fixture rejected: %v", err)
	}
	for name, invalid := range map[string]map[string]any{
		"dimension below minimum":  {"dimension": 0, "metric": "cosine", "filterKeys": []any{}},
		"dimension above maximum":  {"dimension": 1537, "metric": "cosine", "filterKeys": []any{}},
		"dimension wrong type":     {"dimension": "3", "metric": "cosine", "filterKeys": []any{}},
		"metric outside enum":      {"dimension": 3, "metric": "dotproduct", "filterKeys": []any{}},
		"filter key wrong shape":   {"dimension": 3, "metric": "cosine", "filterKeys": "spaceId"},
		"filter key wrong grammar": {"dimension": 3, "metric": "cosine", "filterKeys": []any{"9bad"}},
		"filter key count": {
			"dimension": 3, "metric": "cosine",
			"filterKeys": []any{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		},
	} {
		if err := schema.Validate(invalid); err == nil {
			t.Errorf("%s was accepted: %#v", name, invalid)
		}
	}
	negativeFixtureNames := []string{}
	for name := range candidate.Form.Fixtures {
		if strings.HasPrefix(name, "negative-") {
			negativeFixtureNames = append(negativeFixtureNames, name)
		}
	}
	if len(negativeFixtureNames) < 4 {
		t.Fatalf("generated only %d Form negative fixtures, want dimension/metric/filter coverage", len(negativeFixtureNames))
	}
}

func TestVectorIndexCandidateInterfaceOperationSchemas(t *testing.T) {
	t.Parallel()
	candidate, err := RenderVectorIndexCandidate()
	if err != nil {
		t.Fatal(err)
	}
	var renderedDefinition InterfaceDefinition
	if err := json.Unmarshal([]byte(candidate.Interface.DefinitionJSON), &renderedDefinition); err != nil {
		t.Fatal(err)
	}
	if got, want := len(renderedDefinition.Operations), 4; got != want {
		t.Fatalf("candidate operation count = %d, want %d (no insert/create-only operation)", got, want)
	}
	operations := map[string]InterfaceOperation{}
	for _, operation := range VectorIndexCandidateInterface().Operations {
		operations[operation.Name] = operation
	}
	allowedErrors := map[string]bool{
		"invalid_spec": true, "quota": true, "unavailable": true,
	}
	for name, operation := range operations {
		if len(operation.Errors) != len(allowedErrors) {
			t.Fatalf("operation %s errors = %#v, want the three-code closed vocabulary", name, operation.Errors)
		}
		for _, code := range operation.Errors {
			if !allowedErrors[code] {
				t.Fatalf("operation %s carries invented error code %q", name, code)
			}
		}
	}
	valid := map[string]map[string]any{
		"upsert": {
			"namespace": "space:s",
			"vectors": []any{map[string]any{
				"id": "file:f:0", "values": []any{1, 0, 0},
				"metadata": map[string]any{"spaceId": "s", "chunkIndex": 0},
			}},
		},
		"get":    {"ids": []any{"file:f:0"}},
		"delete": {"namespace": "space:s", "ids": []any{"file:f:0"}},
		"query": {
			"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 1,
			"filter": map[string]any{"spaceId": "s"}, "returnMetadata": true, "returnValues": true,
		},
	}
	for name, input := range valid {
		operation, ok := operations[name]
		if !ok {
			t.Fatalf("operation %s missing", name)
		}
		schema := compileVectorIndexSchema(t, operation.InputSchema)
		if err := schema.Validate(input); err != nil {
			t.Errorf("valid %s input rejected: %v", name, err)
		}
	}
	if got, want := operations["upsert"].Errors, []string{"invalid_spec", "quota", "unavailable"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("upsert errors = %#v, want %#v", got, want)
	}
	for _, name := range []string{"get", "delete", "query"} {
		if got, want := operations[name].Errors, []string{"invalid_spec", "quota", "unavailable"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s errors = %#v, want %#v", name, got, want)
		}
	}
	for _, phrase := range []string{
		"IEEE 754 binary32", "non-finite", "zero vector norm", "canonical binary32",
		"durable acceptance", "observed count", "zero effects", "partial batch",
		"whole call", "fresh VectorIndex", "filterKeys:['spaceId']",
		"before approximate nearest-neighbor selection", "ties unspecified",
		"After accepted mutations", "no concurrent writes", "min(topK, eligibleVisibleCount)",
		"each (namespace, id) appears at most once",
	} {
		if !strings.Contains(VectorIndexCandidateInterface().Description, phrase) {
			t.Errorf("Interface description is missing %q: %s", phrase, VectorIndexCandidateInterface().Description)
		}
	}
	for _, name := range []string{"upsert", "get", "delete", "query"} {
		if strings.Contains(operations[name].Description, "not_found") || strings.Contains(operations[name].Description, "conflict") {
			t.Errorf("%s description carries withdrawn error vocabulary: %s", name, operations[name].Description)
		}
	}
	queryDescription := operations["query"].Description
	for _, phrase := range []string{
		"before approximate nearest-neighbor selection", "finite cosine scores in [-1,1]",
		"ties are unspecified", "missing or empty filter", "returnMetadata", "returnValues", "absent from every match",
		"After accepted mutations", "no concurrent writes", "min(topK, eligibleVisibleCount)", "unique matches",
	} {
		if !strings.Contains(queryDescription, phrase) {
			t.Errorf("query description is missing %q: %s", phrase, queryDescription)
		}
	}
	queryOutputProperties, _ := operations["query"].OutputSchema["properties"].(map[string]any)
	queryCount, _ := queryOutputProperties["count"].(map[string]any)
	if got := queryCount["description"]; got != "Exactly len(matches), never greater than topK." {
		t.Errorf("query count description = %#v", got)
	}
	upsertOutputProperties, _ := operations["upsert"].OutputSchema["properties"].(map[string]any)
	mutationCount, _ := upsertOutputProperties["count"].(map[string]any)
	if got := mutationCount["minimum"]; got != 1 {
		t.Errorf("successful mutation count minimum = %#v, want 1", got)
	}
	if got, _ := mutationCount["description"].(string); !strings.Contains(got, "accepted request") {
		t.Errorf("successful mutation count description = %#v", mutationCount["description"])
	}
	queryMatches, _ := queryOutputProperties["matches"].(map[string]any)
	if got, _ := queryMatches["description"].(string); !strings.Contains(got, "min(topK, eligibleVisibleCount)") ||
		!strings.Contains(got, "each (namespace, id) appears at most once") {
		t.Fatalf("query matches schema omits settled cardinality/uniqueness contract: %q", got)
	}
	queryMatchItems, _ := queryMatches["items"].(map[string]any)
	queryMatchProperties, _ := queryMatchItems["properties"].(map[string]any)
	for name, phrase := range map[string]string{
		"metadata": "Present on every match only when returnMetadata is true; absent when false or omitted.",
		"values":   "Present on every match only when returnValues is true; absent when false or omitted.",
	} {
		property, _ := queryMatchProperties[name].(map[string]any)
		if got := property["description"]; got != phrase {
			t.Errorf("query match %s description = %#v, want %q", name, got, phrase)
		}
	}
	if _, ok := queryMatchProperties["metadata"].(map[string]any)["default"]; ok {
		t.Fatal("query metadata output schema carries an input default and could fabricate an omitted field")
	}
	metadataValue, _ := vectorIndexMetadataValueSchema()["oneOf"].([]any)
	if len(metadataValue) < 2 {
		t.Fatal("metadata scalar schema lost its numeric branch")
	}
	numericMetadata, _ := metadataValue[1].(map[string]any)
	if got := numericMetadata["description"]; got != "Finite JSON number; non-finite values are invalid_spec." {
		t.Errorf("numeric metadata description = %#v", got)
	}
	if !strings.Contains(vectorIndexMetadataSchema()["description"].(string), "RFC 8785 canonical JSON UTF-8") {
		t.Fatal("metadata schema does not state the RFC 8785 UTF-8 byte measurement")
	}
	querySchema := compileVectorIndexSchema(t, operations["query"].InputSchema)
	for name, invalid := range map[string]map[string]any{
		"namespace wildcard":         {"namespace": "*", "values": []any{1, 0, 0}, "topK": 1},
		"namespace too long":         {"namespace": strings.Repeat("n", 129), "values": []any{1, 0, 0}, "topK": 1},
		"query wrong values shape":   {"namespace": "space:s", "values": "1,0,0", "topK": 1},
		"query topK zero":            {"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 0},
		"query return shape":         {"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 1, "returnValues": "yes"},
		"query return metadata all":  {"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 1, "returnMetadata": "all"},
		"query return metadata none": {"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 1, "returnMetadata": "none"},
		"query namespace NUL":        {"namespace": "space\x00s", "values": []any{1, 0, 0}, "topK": 1},
		"query empty values":         {"namespace": "space:s", "values": []any{}, "topK": 1},
	} {
		if err := querySchema.Validate(invalid); err == nil {
			t.Errorf("%s was accepted: %#v", name, invalid)
		}
	}
	upsertSchema := compileVectorIndexSchema(t, operations["upsert"].InputSchema)
	if err := upsertSchema.Validate(map[string]any{"vectors": []any{}}); err == nil {
		t.Fatal("empty upsert vectors were accepted")
	}
	if err := upsertSchema.Validate(map[string]any{"vectors": []any{map[string]any{"id": "x", "values": []any{}}}}); err == nil {
		t.Fatal("empty vector values were accepted")
	}
	if err := upsertSchema.Validate(map[string]any{
		"vectors": []any{map[string]any{"id": "x", "values": []any{1}, "metadata": map[string]any{"nested": map[string]any{}}}},
	}); err == nil {
		t.Fatal("nested metadata object was accepted despite flat scalar contract")
	}
	if err := upsertSchema.Validate(map[string]any{
		"vectors": []any{map[string]any{
			"id": "x", "values": []any{1},
			"metadata": map[string]any{"constructor": "plain-json-safe"},
		}},
	}); err != nil {
		t.Fatalf("constructor metadata key was rejected despite the simple identifier grammar: %v", err)
	}
	getSchema := compileVectorIndexSchema(t, operations["get"].InputSchema)
	if err := getSchema.Validate(map[string]any{"ids": []any{}}); err == nil {
		t.Fatal("empty get IDs were accepted")
	}
	if err := getSchema.Validate(map[string]any{"ids": []any{strings.Repeat("i", 129)}}); err == nil {
		t.Fatal("129-character ID was accepted")
	}
	if err := getSchema.Validate(map[string]any{"ids": []any{"bad\x00id"}}); err == nil {
		t.Fatal("NUL-containing ID was accepted")
	}
	deleteSchema := compileVectorIndexSchema(t, operations["delete"].InputSchema)
	if err := deleteSchema.Validate(map[string]any{"ids": []any{}}); err == nil {
		t.Fatal("empty delete IDs were accepted")
	}
	for _, name := range []string{"upsert", "get", "delete"} {
		operation := operations[name]
		if name == "upsert" {
			vectors, _ := operation.InputSchema["properties"].(map[string]any)
			batch, _ := vectors["vectors"].(map[string]any)
			if got := batch["minItems"]; got != 1 {
				t.Errorf("upsert vectors minItems = %#v, want 1", got)
			}
		} else {
			properties, _ := operation.InputSchema["properties"].(map[string]any)
			ids, _ := properties["ids"].(map[string]any)
			if got := ids["minItems"]; got != 1 {
				t.Errorf("%s IDs minItems = %#v, want 1", name, got)
			}
		}
	}
	unicodeID := strings.Repeat("😀", 128)
	if err := getSchema.Validate(map[string]any{"ids": []any{unicodeID}}); err != nil {
		t.Fatalf("128-code-point Unicode ID was rejected: %v", err)
	}
	if err := getSchema.Validate(map[string]any{"ids": []any{unicodeID + "😀"}}); err == nil {
		t.Fatal("129-code-point Unicode ID was accepted")
	}
	unicodeNamespace := strings.Repeat("😀", 128)
	if err := querySchema.Validate(map[string]any{
		"namespace": unicodeNamespace, "values": []any{1}, "topK": 1,
	}); err != nil {
		t.Fatalf("128-code-point Unicode namespace was rejected: %v", err)
	}
	if err := querySchema.Validate(map[string]any{
		"namespace": unicodeNamespace + "😀", "values": []any{1}, "topK": 1,
	}); err == nil {
		t.Fatal("129-code-point Unicode namespace was accepted")
	}
	duplicateFixture := false
	for _, fixture := range VectorIndexCandidateInterface().Fixtures {
		if fixture.Name != "duplicate-upsert-id-is-invalid" {
			continue
		}
		duplicateFixture = true
		if len(fixture.Steps) != 1 || fixture.Steps[0].ExpectedError != "invalid_spec" {
			t.Fatalf("duplicate-ID fixture = %#v, want one invalid_spec step", fixture)
		}
	}
	if !duplicateFixture {
		t.Fatal("candidate does not carry the duplicate-ID invalid_spec fixture")
	}
	if !strings.Contains(operations["upsert"].Description, "fully clears prior metadata") {
		t.Fatalf("upsert description does not make omitted metadata replacement explicit: %s", operations["upsert"].Description)
	}
	if !strings.Contains(operations["delete"].Description, "partial batch") ||
		!strings.Contains(operations["delete"].Description, "whole call") {
		t.Fatalf("delete description does not make partial whole-call retry semantics explicit: %s", operations["delete"].Description)
	}
	observedProperties := candidate.Form.Definition.ObservedSchema["properties"].(map[string]any)
	observedCount := observedProperties["count"].(map[string]any)
	if got, _ := observedCount["description"].(string); !strings.Contains(got, "across all namespaces") {
		t.Fatalf("observed count description = %#v", observedCount["description"])
	}
	observedFilterKeys := observedProperties["filterKeys"].(map[string]any)
	if got := observedFilterKeys["description"]; got != "Declared filter keys are emitted in lexical order, matching canonical set-default ordering." {
		t.Fatalf("observed filterKeys description = %#v", got)
	}
	for _, fixture := range VectorIndexCandidateInterface().Fixtures {
		if fixture.Name == "upsert-accepts-explicit-namespace" || fixture.Name == "delete-returns-exact-requested-namespace" {
			if len(fixture.Steps) != 1 || len(fixture.Steps[0].Expected) == 0 {
				t.Fatalf("fixture %s has no deterministic expected mutation output: %#v", fixture.Name, fixture)
			}
		}
	}
}

func TestVectorIndexCandidateDoesNotFakeEventualQueryVisibility(t *testing.T) {
	t.Parallel()
	definition := VectorIndexCandidateInterface()
	querySteps := 0
	for _, fixture := range definition.Fixtures {
		for _, step := range fixture.Steps {
			if step.Operation != "query" {
				continue
			}
			querySteps++
			// Core's InterfaceFixtureStep has no settled-precondition or polling
			// member. The only static query trace therefore stays in a fresh scope
			// and proves an empty result without claiming immediate post-upsert
			// visibility; positive cardinality is a Host conformance obligation.
			if fixture.Name != "query-filters-declared-key" {
				t.Fatalf("query fixture %q is not the fresh-scope trace", fixture.Name)
			}
			matches, ok := step.Expected["matches"].([]any)
			if !ok || len(matches) != 0 {
				t.Fatalf("fresh-scope query fixture claims nonempty matches: %#v", step.Expected)
			}
			if count, ok := step.Expected["count"].(int); !ok || count != 0 {
				t.Fatalf("fresh-scope query fixture count = %#v, want zero", step.Expected["count"])
			}
		}
	}
	if querySteps != 1 {
		t.Fatalf("candidate has %d static query steps; expected only the fresh-scope trace", querySteps)
	}
}

func TestVectorIndexCandidateFixturesMatchOperationSchemas(t *testing.T) {
	t.Parallel()
	definition := VectorIndexCandidateInterface()
	operations := make(map[string]InterfaceOperation, len(definition.Operations))
	for _, operation := range definition.Operations {
		operations[operation.Name] = operation
	}
	for _, fixture := range definition.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			for stepIndex, step := range fixture.Steps {
				operation, ok := operations[step.Operation]
				if !ok {
					t.Fatalf("fixture step %d names undeclared operation %q", stepIndex, step.Operation)
				}
				inputSchema := compileVectorIndexSchema(t, operation.InputSchema)
				if err := inputSchema.Validate(step.Input); err != nil {
					t.Fatalf("fixture step %d %s input rejected by its operation schema: %v\ninput=%#v", stepIndex, step.Operation, err, step.Input)
				}
				// A nil Expected means the fixture deliberately makes no static
				// output claim (for example, an eventual read). Error steps also
				// have no output document to validate.
				if step.ExpectedError != "" || step.Expected == nil {
					continue
				}
				outputSchema := compileVectorIndexSchema(t, operation.OutputSchema)
				if err := outputSchema.Validate(step.Expected); err != nil {
					t.Fatalf("fixture step %d %s expected output rejected by its operation schema: %v\nexpected=%#v", stepIndex, step.Operation, err, step.Expected)
				}
			}
		})
	}
}

func compileVectorIndexSchema(t *testing.T, schema map[string]any) *jsonschema.Schema {
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
	const id = "https://candidate.test/vector-index-schema.json"
	if err := compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}
