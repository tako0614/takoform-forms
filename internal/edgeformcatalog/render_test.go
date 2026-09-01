package edgeformcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// TestConstraintsSurviveRenderedDefinitionRoundTrip pins the whole
// authoring boundary: the rich model renders every kind-specific member, the
// embedded current schema validates those JSON bytes, and typed decoding
// returns the same constraint list. A field omitted by either renderer or the
// Form Package model is therefore observable here instead of becoming an
// unenforced host promise.
func TestConstraintsSurviveRenderedDefinitionRoundTrip(t *testing.T) {
	t.Parallel()
	wantModel := []model.Constraint{
		{Kind: model.ConstraintOrderedPair, References: []string{"/minimum", "/maximum"}},
		{Kind: model.ConstraintUniqueBy, List: "/indexes", Member: "name"},
		{Kind: model.ConstraintAcyclic, Reference: "/deadLetter/queue"},
		{Kind: model.ConstraintDistinctPair, References: []string{"/primary", "/alternate"}},
		{Kind: model.ConstraintUniquePair, References: []string{"/topic", "/primary"}},
		{
			Kind:   model.ConstraintSameResolvedTarget,
			Anchor: "/function", Members: "/versions/*/functionVersion", Through: "/function",
		},
	}
	ref := func(hcl, wire, kind string) model.Field {
		return model.Field{
			HCL: hcl, Wire: wire, Kind: model.KindResourceRef, Required: true,
			Doc:     "Exact resolved-UID constraint participant.",
			Example: map[string]any{"apiVersion": Family.APIVersion(), "kind": kind, "name": hcl},
			ResourceTarget: &model.ResourceTarget{
				Group: Family.APIVersion(), Kind: kind, Contract: model.TargetContract{ExactForm: true},
			},
		}
	}
	form := model.Form{
		Family: Family, Kind: "ConstraintProbe", Slug: "constraint-probe",
		Role: model.RoleIdentity, RequiresHostAPI: "forms.takoform.com/v1",
		DefinitionVersion: "0.1.0", Title: "Constraint probe", Description: "Round-trip probe.",
		Fields: []model.Field{
			{HCL: "minimum", Wire: "minimum", Kind: model.KindInteger, Required: true, Min: model.I64(0), Max: model.I64(100), Doc: "Lower bound.", Example: 1},
			{HCL: "maximum", Wire: "maximum", Kind: model.KindInteger, Required: true, Min: model.I64(0), Max: model.I64(100), Doc: "Upper bound.", Example: 10},
			{HCL: "indexes", Wire: "indexes", Kind: model.KindObjectList, Required: true, MinItems: 1, MaxItems: 8, Doc: "Named indexes.",
				Example: []any{map[string]any{"name": "by-name"}}, Fields: []model.Field{{
					HCL: "name", Wire: "name", Kind: model.KindString, Required: true,
					Pattern: model.PatternResourceName, MaxLength: model.ResourceNameMaxLength, Doc: "Index name.", Example: "by-name",
				}}},
			{HCL: "dead_letter", Wire: "deadLetter", Kind: model.KindObject, Required: true, Doc: "Dead-letter target.",
				Example: map[string]any{"queue": map[string]any{"apiVersion": Family.APIVersion(), "kind": "AtLeastOnceQueue", "name": "queue"}},
				Fields:  []model.Field{ref("queue", "queue", "AtLeastOnceQueue")}},
			ref("primary", "primary", "AtLeastOnceQueue"),
			ref("alternate", "alternate", "AtLeastOnceQueue"),
			ref("topic", "topic", "AtLeastOnceQueue"),
			ref("function", "function", "ModuleWorker"),
			{HCL: "versions", Wire: "versions", Kind: model.KindObjectList, Required: true, MinItems: 1, MaxItems: 8,
				Doc: "Selected versions.", Example: []any{map[string]any{"functionVersion": map[string]any{
					"apiVersion": Family.APIVersion(), "kind": "WorkerVersion", "name": "version-a",
				}}}, Fields: []model.Field{ref("function_version", "functionVersion", "WorkerVersion")}},
		},
		StructuralConstraints:  wantModel[:2],
		ResolvedUIDConstraints: wantModel[2:],
	}
	rendered, err := renderForm(form, constraintProbeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := []formpackage.FormConstraint{
		{Kind: "orderedPair", References: []string{"/minimum", "/maximum"}},
		{Kind: "uniqueBy", List: "/indexes", Member: "name"},
		{Kind: "acyclic", Reference: "/deadLetter/queue"},
		{Kind: "distinctPair", References: []string{"/primary", "/alternate"}},
		{Kind: "uniquePair", References: []string{"/topic", "/primary"}},
		{Kind: "sameResolvedTarget", Anchor: "/function", Members: "/versions/*/functionVersion", Through: "/function"},
	}
	wantConstraints, _ := json.Marshal(want)
	gotConstraints, _ := json.Marshal(decoded.Constraints)
	if string(gotConstraints) != string(wantConstraints) {
		t.Fatalf("decoded constraints = %s, want literal %s", gotConstraints, wantConstraints)
	}
	// Compare canonical JSON rather than Go map representation: JSON numbers in
	// decoded schemas legitimately use a different Go numeric type, while the
	// contract is equality of the rendered Definition document.
	wantRaw, err := json.Marshal(rendered.Definition)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, err := formpackage.Canonicalize(wantRaw)
	if err != nil {
		t.Fatal(err)
	}
	gotCanonical, err := formpackage.Canonicalize(gotRaw)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCanonical) != string(wantCanonical) {
		t.Fatalf("render -> JSON -> validate -> decode changed the Definition\ngot  %s\nwant %s", gotCanonical, wantCanonical)
	}
}

func TestTaggedObjectAnnotationSurvivesRuntimeDefinitionValidation(t *testing.T) {
	t.Parallel()
	form := model.Form{
		Family: Family, Kind: "TaggedProbe", Slug: "tagged-probe",
		Role: model.RoleIdentity, RequiresHostAPI: "forms.takoform.com/v1",
		DefinitionVersion: "0.1.0", Title: "Tagged probe", Description: "Closed tagged-object probe.",
		Fields: []model.Field{{
			HCL: "delivery", Wire: "delivery", Kind: model.KindTaggedObject, Required: true,
			Doc: "Exactly one closed delivery variant.", Discriminator: "type",
			Variants: []model.TaggedObjectVariant{
				{Tag: "direct", Fields: []model.Field{{
					HCL: "address", Wire: "address", Kind: model.KindString, Required: true,
					Pattern: `^[a-z][a-z0-9-]{0,31}$`, MaxLength: 32, Doc: "Portable address.", Example: "primary",
				}}},
				{Tag: "discard", Fields: []model.Field{{
					HCL: "reason", Wire: "reason", Kind: model.KindString, Required: true,
					Pattern: `^[a-z][a-z0-9-]{0,31}$`, MaxLength: 32, Doc: "Portable reason.", Example: "expired",
				}}},
			},
			Example: map[string]any{"type": "direct", "address": "primary"},
		}},
	}
	rendered, err := renderForm(form, constraintProbeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := formpackage.ValidateDefinition([]byte(rendered.DefinitionJSON))
	if err != nil {
		t.Fatal(err)
	}
	properties := decoded.DesiredSchema["properties"].(map[string]any)
	delivery := properties["delivery"].(map[string]any)
	if delivery[model.TaggedObjectDiscriminatorAnnotationKey] != "type" {
		t.Fatalf("tagged discriminator annotation = %#v", delivery[model.TaggedObjectDiscriminatorAnnotationKey])
	}
}

type constraintProbeResolver struct{}

func (constraintProbeResolver) ResolveResourceTarget(target model.ResourceTarget) (model.ResolvedResourceTarget, error) {
	return model.ResolvedResourceTarget{
		ResourceNamePattern: model.PatternResourceName,
		TargetFormRefs: []model.TargetFormRef{{
			APIVersion: target.Group, Kind: target.Kind, DefinitionVersion: "0.1.0",
			SchemaDigest: "sha256:" + strings.Repeat("a", 64),
		}},
	}, nil
}

func (constraintProbeResolver) ResolveExactFormRelations(model.TargetFormRef) ([]model.Relation, error) {
	return []model.Relation{{
		Pointer: "/function", TargetAPIVersion: Family.APIVersion(), TargetKind: "ModuleWorker",
	}}, nil
}

// TestRenderedFormsVerifyAsV1Alpha5Packages proves the complete authoring
// pipeline end to end: every catalog Form renders to a definition whose
// staged package — definition, canonical fixture, and negative fixtures —
// passes formpackage.VerifyDirectory under the v1alpha5 index profile.
func TestRenderedFormsVerifyAsV1Alpha5Packages(t *testing.T) {
	t.Parallel()
	forms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != len(Forms) {
		t.Fatalf("rendered %d forms, want %d", len(forms), len(Forms))
	}
	for _, form := range forms {
		form := form
		t.Run(form.Slug, func(t *testing.T) {
			t.Parallel()
			if len(form.Definition.NegativeFixtures) == 0 {
				t.Fatal("every Form must carry at least one negative fixture")
			}
			root := t.TempDir()
			definitionRaw := []byte(form.DefinitionJSON)
			writeFile(t, filepath.Join(root, "definition.json"), definitionRaw)
			payloadPaths := []string{"definition.json"}
			payloads := map[string][]byte{"definition.json": definitionRaw}
			for name, document := range form.Fixtures {
				raw, err := marshalIndented(document)
				if err != nil {
					t.Fatal(err)
				}
				relative := "fixtures/" + name
				writeFile(t, filepath.Join(root, "fixtures", name), []byte(raw))
				payloadPaths = append(payloadPaths, relative)
				payloads[relative] = []byte(raw)
			}
			slices.Sort(payloadPaths)
			files := make([]any, 0, len(payloadPaths))
			for _, relative := range payloadPaths {
				mediaType := "application/json"
				if relative == "definition.json" {
					mediaType = formpackage.DefinitionMediaType
				}
				files = append(files, map[string]any{
					"path":      relative,
					"mediaType": mediaType,
					"size":      len(payloads[relative]),
					"digest":    formpackage.DigestBytes(payloads[relative]),
				})
			}
			schemaDigest, err := formpackage.DigestCanonicalJSON(definitionRaw)
			if err != nil {
				t.Fatal(err)
			}
			index := map[string]any{
				"apiVersion": formpackage.VersionlessFamilyPackageAPIVersion,
				"kind":       formpackage.PackageKind,
				"formRef": map[string]any{
					"apiVersion":        Family.APIVersion(),
					"kind":              form.Kind,
					"definitionVersion": form.Definition.DefinitionVersion,
					"schemaDigest":      schemaDigest,
				},
				"definitionPath": "definition.json",
				"files":          files,
			}
			indexRaw, err := marshalIndented(index)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, formpackage.PackageIndexFilename), []byte(indexRaw))
			report, err := formpackage.VerifyDirectory(root)
			if err != nil {
				t.Fatal(err)
			}
			if report.FormRef.Kind != form.Kind || report.FormRef.APIVersion != Family.APIVersion() {
				t.Fatalf("verified FormRef = %+v", report.FormRef)
			}
		})
	}
}

// TestWorkerBundleWireSpecIsOnlyTheManifestDigest asserts the collapse onto
// one source of truth against the bytes that actually ship: the GENERATED
// candidate definition, not the in-memory catalog. A WorkerBundle's desired
// state is exactly manifestDigest — the artifact manifest, not the Form,
// describes the modules, so a second spelling of the same bytes must not
// reappear in the wire spec.
func TestWorkerBundleWireSpecIsOnlyTheManifestDigest(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "forms", "candidates", "edge.forms.takoform.com", "worker-bundle", "definition.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	definition, err := formpackage.ValidateDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	properties, ok := definition.DesiredSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("the generated WorkerBundle definition has no desired properties object")
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"manifestDigest"}) {
		t.Fatalf("WorkerBundle wire spec = %v, want exactly [manifestDigest]", names)
	}
	digest, _ := properties["manifestDigest"].(map[string]any)
	if digest["type"] != "string" || digest["pattern"] != model.PatternCanonicalSHA256 {
		t.Fatalf("manifestDigest is not a canonical sha256 string: %v", digest)
	}
	required, _ := definition.DesiredSchema["required"].([]any)
	if len(required) != 1 || required[0] != "manifestDigest" {
		t.Fatalf("WorkerBundle required = %v, want exactly [manifestDigest]", required)
	}
	if !slices.Equal(definition.ImmutableFields, []string{"/manifestDigest"}) {
		t.Fatalf("WorkerBundle immutableFields = %v", definition.ImmutableFields)
	}
	if closed, _ := definition.DesiredSchema["additionalProperties"].(bool); closed {
		t.Fatal("the WorkerBundle desired schema is not closed")
	}
}

func TestRenderedDefinitionsOmitNameAndEnvelopeFields(t *testing.T) {
	t.Parallel()
	forms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range forms {
		properties, ok := form.Definition.DesiredSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s desired schema has no properties object", form.Kind)
		}
		for _, forbidden := range []string{"name", "id", "generation", "ready", "portability"} {
			if _, present := properties[forbidden]; present {
				t.Errorf("%s desired schema declares envelope field %q", form.Kind, forbidden)
			}
		}
		if form.Definition.ObservedSchema != nil {
			t.Errorf("%s declares an observed schema; MVP forms declare none", form.Kind)
		}
	}
}

// TestOutputSchemasArePinnedAndClosed states, once and by hand, which Forms
// publish an output contract at all.
//
// The list matters more than the shape. `status.outputs` is REQUIRED on the
// wire exactly for a Form whose Definition declares an outputSchema and
// OMITTED for every other (the public Core's embedded host-api-wire-v1beta1.schema.json), so
// a Form that gained or lost one silently would change what every conforming
// host must return for it. Writing the set out means that change cannot happen
// without an edit here.
func TestOutputSchemasArePinnedAndClosed(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"WorkerEndpoint": {"hostname", "url"},
	}
	forms, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range forms {
		declared, publishes := want[form.Kind]
		if !publishes {
			if form.Definition.OutputSchema != nil {
				t.Errorf("%s declares an output schema that the pinned set does not list", form.Kind)
			}
			continue
		}
		schema := form.Definition.OutputSchema
		if schema == nil {
			t.Fatalf("%s must declare an output schema", form.Kind)
		}
		if closed, _ := schema["additionalProperties"].(bool); closed {
			t.Errorf("%s output schema is not closed", form.Kind)
		}
		properties, _ := schema["properties"].(map[string]any)
		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}
		slices.Sort(names)
		if !slices.Equal(names, declared) {
			t.Errorf("%s outputs = %v, want %v", form.Kind, names, declared)
		}
		required, _ := schema["required"].([]string)
		if !slices.Equal(required, declared) {
			t.Errorf("%s required outputs = %v, want every declared output %v", form.Kind, required, declared)
		}
	}
}

// TestContractDefinitionsSatisfyNormativeSchemas validates every rendered
// Interface and Binding Definition through Core's embedded normative schemas.
func TestContractDefinitionsSatisfyNormativeSchemas(t *testing.T) {
	t.Parallel()
	interfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(interfaces) != 8 {
		t.Fatalf("interface catalog has %d entries, want 8", len(interfaces))
	}
	for _, contract := range interfaces {
		if err := formpackage.ValidateInterfaceDefinition([]byte(contract.DefinitionJSON)); err != nil {
			t.Errorf("interface %s does not satisfy the normative schema: %v", contract.Name, err)
		}
		digest, err := formpackage.DigestCanonicalJSON([]byte(contract.DefinitionJSON))
		if err != nil {
			t.Fatal(err)
		}
		if digest != contract.SchemaDigest {
			t.Errorf("interface %s digest drifted", contract.Name)
		}
	}

	bindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 7 {
		t.Fatalf("binding catalog has %d entries, want 7", len(bindings))
	}
	interfaceDigests := map[string]string{}
	for _, contract := range interfaces {
		interfaceDigests[contract.Name] = contract.SchemaDigest
	}
	definitions, err := BindingDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	for index, contract := range bindings {
		if err := formpackage.ValidateBindingDefinition([]byte(contract.DefinitionJSON)); err != nil {
			t.Errorf("binding %s does not satisfy the normative schema: %v", contract.Name, err)
		}
		target := definitions[index].TargetInterface
		if interfaceDigests[target.Name] != target.SchemaDigest {
			t.Errorf("binding %s embeds a stale digest for interface %s", contract.Name, target.Name)
		}
	}
}

func TestQueueProducerProjectsOnlySubmission(t *testing.T) {
	t.Parallel()
	definitions, err := BindingDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if definition.Name != "module-worker.queue-producer" {
			continue
		}
		operations := definition.RuntimeProjection.Operations
		if len(operations) != 2 || operations[0] != "send" || operations[1] != "sendBatch" {
			t.Fatalf("queue-producer projects %v", operations)
		}
		return
	}
	t.Fatal("module-worker.queue-producer is not in the catalog")
}

func TestObjectBucketBindingUsesLengthAwareStreamingABI(t *testing.T) {
	t.Parallel()
	definitions, err := BindingDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if definition.Name != "module-worker.object-bucket" {
			continue
		}
		if definition.Version != "1.1.0" {
			t.Fatalf("object bucket binding version = %s, want 1.1.0", definition.Version)
		}
		description := definition.Description
		positive := []string{
			"put(key, body, options?)",
			"uploadPart(key, uploadId, partNumber, body, options?)",
			"body is a ReadableStream<Uint8Array>",
			"contentLength may be omitted",
			"MUST equal the intrinsic byte length",
			"ReadableStream, options is required and contentLength is required",
			"streams bytes with backpressure",
			"MUST NOT buffer the body merely to discover its length",
			"list accepts prefix, cursor, and delimiter strings plus integer limit from 1 through 1000",
			"`maxLength` counts Unicode code points",
			"only key/prefix count UTF-8 bytes",
			"createMultipartUpload accepts only contentType",
			"completeMultipartUpload parts is an ordered array of 1 through 10000 closed {partNumber, etag} objects",
			"A list resolves to {objects, prefixes, truncated, cursor?}",
			"values that cannot form a valid Interface input document reject with TypeError",
			"once the caller supplies structurally valid input, a schema-valid Interface operation fails with the exact error name declared by that operation",
			"key outside the Interface UTF-8 byte budget is invalid_key",
			"schema-valid cursor the host does not recognize is invalid_cursor",
			"object-relative range that cannot be served is range_not_satisfiable",
			"unmet validator is precondition_failed",
			"undersized non-final parts is invalid_part",
			"unknown upload is upload_not_found",
			"Absent ReadableStream contentLength",
			"numeric contentLength that is non-finite, non-integral, negative, unsafe, over the Interface limit, or mismatched",
			"Present non-number contentLength",
		}
		for _, phrase := range positive {
			if !strings.Contains(description, phrase) {
				t.Errorf("object bucket ABI description omits positive rule %q", phrase)
			}
		}
		negative := []string{
			"Extra positional arguments, non-plain-object options, unknown option members, and invalid input types reject with TypeError",
			"reject pre-consumption with invalid_body",
			"other invalid body/option types, and unknown option members reject with TypeError",
			"stream delivering a different count rejects with invalid_body and stores nothing",
			"`bodyStream` is host-internal wire framing and is never exposed in or accepted by this JavaScript result",
		}
		for _, phrase := range negative {
			if !strings.Contains(description, phrase) {
				t.Errorf("object bucket ABI description omits negative rule %q", phrase)
			}
		}
		if strings.Contains(description, "{body, bodyStream") {
			t.Error("object bucket ABI must not expose host-internal bodyStream in the JavaScript get result")
		}
		if strings.Contains(description, "Well-typed values outside those semantic bounds reject with the exact Interface error name") {
			t.Error("object bucket ABI must not promise an Interface error for a bound that has no declared operation error")
		}
		if strings.Contains(description, "prefixes?") {
			t.Error("object bucket ABI must normalize an absent prefix set to a required empty array")
		}
		return
	}
	t.Fatal("module-worker.object-bucket is not in the catalog")
}

func TestEdgeObjectsDefersMultipartMinimumSizeValidationUntilCompletion(t *testing.T) {
	t.Parallel()
	definitions := InterfaceDefinitions()
	for _, definition := range definitions {
		if definition.Name != "edge.objects" {
			continue
		}
		var put, uploadPart, completeMultipartUpload *InterfaceOperation
		for index := range definition.Operations {
			operation := &definition.Operations[index]
			switch operation.Name {
			case "put":
				put = operation
			case "uploadPart":
				uploadPart = operation
			case "completeMultipartUpload":
				completeMultipartUpload = operation
			}
		}
		if put == nil || uploadPart == nil || completeMultipartUpload == nil {
			t.Fatal("edge.objects omits put or multipart operations")
		}
		if !strings.Contains(put.Description, "above maxSinglePutBytes fails with value_too_large") ||
			!slices.Contains(put.Errors, "value_too_large") {
			t.Error("put must route a schema-valid object above maxSinglePutBytes to value_too_large")
		}
		if strings.Contains(uploadPart.Description, "except the highest-numbered one") {
			t.Error("uploadPart cannot know which part will be highest in the later completion set")
		}
		if !strings.Contains(uploadPart.Description, "A part may be shorter than 5242880 bytes here") {
			t.Error("uploadPart must explicitly allow a short part until the later completion request identifies the final part")
		}
		if slices.Contains(uploadPart.Errors, "invalid_part") {
			t.Error("uploadPart declares invalid_part without a schema-valid trigger")
		}
		if slices.Contains(uploadPart.Errors, "value_too_large") {
			t.Error("uploadPart declares value_too_large even though contentLength is schema-bounded by maxObjectBytes")
		}
		for _, phrase := range []string{
			"Every part except the highest-numbered part in this completion request MUST be at least 5242880 bytes",
			"fails with invalid_part and assembles nothing",
		} {
			if !strings.Contains(completeMultipartUpload.Description, phrase) {
				t.Errorf("completeMultipartUpload omits enforceable multipart rule %q", phrase)
			}
		}
		if !slices.Contains(completeMultipartUpload.Errors, "invalid_part") {
			t.Error("completeMultipartUpload does not declare invalid_part")
		}
		if slices.Contains(completeMultipartUpload.Errors, "precondition_failed") {
			t.Error("completeMultipartUpload declares precondition_failed without a conditional input")
		}
		return
	}
	t.Fatal("edge.objects is not in the catalog")
}

func TestEdgeObjectsOperationErrorsAreExact(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"head":                    {"invalid_key", "not_found", "backend_unavailable"},
		"get":                     {"invalid_key", "not_found", "precondition_failed", "range_not_satisfiable", "backend_unavailable"},
		"put":                     {"invalid_key", "invalid_body", "value_too_large", "precondition_failed", "backend_unavailable"},
		"delete":                  {"invalid_key", "backend_unavailable"},
		"list":                    {"invalid_cursor", "backend_unavailable"},
		"createMultipartUpload":   {"invalid_key", "backend_unavailable"},
		"uploadPart":              {"invalid_key", "invalid_body", "upload_not_found", "backend_unavailable"},
		"completeMultipartUpload": {"invalid_key", "invalid_part", "upload_not_found", "value_too_large", "backend_unavailable"},
		"abortMultipartUpload":    {"invalid_key", "upload_not_found", "backend_unavailable"},
	}
	for _, definition := range InterfaceDefinitions() {
		if definition.Name != "edge.objects" {
			continue
		}
		if len(definition.Operations) != len(want) {
			t.Fatalf("edge.objects operations = %d, want %d", len(definition.Operations), len(want))
		}
		for _, operation := range definition.Operations {
			errors, ok := want[operation.Name]
			if !ok {
				t.Errorf("unexpected edge.objects operation %q", operation.Name)
				continue
			}
			if !slices.Equal(operation.Errors, errors) {
				t.Errorf("%s errors = %v, want %v", operation.Name, operation.Errors, errors)
			}
		}
		return
	}
	t.Fatal("edge.objects is not in the catalog")
}

func writeFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
