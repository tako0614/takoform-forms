package edgeformcatalog

import (
	"fmt"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

// VectorIndexCandidate is an unpublished development-only artifact set. It is
// kept outside Forms and InterfaceDefinitions deliberately: rendering this
// value does not register a catalog member, alter the current identity set, or
// grant publisher/Host support authority. The forward WorkerVersion and
// WorkerDeployment close the candidate's exact reference graph; none of these
// artifacts is added to the registered catalogs.
type VectorIndexCandidate struct {
	Form             RenderedForm     `json:"form"`
	Interface        RenderedContract `json:"interface"`
	Binding          RenderedContract `json:"binding"`
	WorkerVersion    RenderedForm     `json:"workerVersion"`
	WorkerDeployment RenderedForm     `json:"workerDeployment"`
}

const (
	// VectorIndexCandidateFormVersion is intentionally a prerelease. It is not
	// a current or publishable identity.
	VectorIndexCandidateFormVersion = "0.1.0-dev.1"
	VectorIndexCandidateFormKind    = "VectorIndex"
	VectorIndexCandidateFormSlug    = "vector-index"

	VectorIndexCandidateInterfaceName = "edge.vector"
	// Core v1.1.0's Interface Definition and InterfaceRef schemas accept only
	// numeric SemVer (no prerelease segment). The enclosing Form therefore uses
	// the distinct dev prerelease above while this unregistered Interface uses
	// the nearest Core-valid numeric version. The future allocation under review
	// is VectorIndex@0.1.0, edge.vector@0.1.0,
	// module-worker.edge-vector@1.0.0, WorkerVersion@0.4.0, and
	// WorkerDeployment@0.3.0; none is registered by this candidate.
	VectorIndexCandidateInterfaceVersion = "0.1.0"

	vectorIndexCandidateMaxDimension       = 1536
	vectorIndexCandidateMaxIDLength        = 128
	vectorIndexCandidateMaxNamespaceLength = 128
	vectorIndexCandidateMaxBatch           = 100
	vectorIndexCandidateMaxTopK            = 100
	vectorIndexCandidateMaxMetadataBytes   = 8192
	vectorIndexCandidateMaxFilterKeys      = 8
	vectorIndexCandidateMaxMetadataProps   = 64
	vectorIndexCandidateMaxMetadataString  = 8192
	vectorIndexNUL                         = "\x00"
)

// VectorIndexCandidateForm returns the unpublished Form authoring value. It
// has no ProvidedInterfaces member because the current catalog resolver can
// only resolve registered Interfaces; RenderVectorIndexCandidate attaches the
// candidate's digest after rendering and validates the resulting Definition.
func VectorIndexCandidateForm() model.Form {
	return model.Form{
		Family:            Family,
		Kind:              VectorIndexCandidateFormKind,
		Slug:              VectorIndexCandidateFormSlug,
		Role:              model.RoleIdentity,
		DefinitionVersion: VectorIndexCandidateFormVersion,
		RequiresHostAPI:   stableHostLane,
		Title:             "Install-owned vector index",
		Description: "Unpublished install-owned vector index candidate. The resource fixes a positive vector " +
			"dimension, cosine metric, and bounded optional metadata filter-key set. Native storage, endpoints, " +
			"credentials, table names, and provider IDs are Host-private. Lifecycle is create, read, delete, and " +
			"observe only; configuration is immutable and no import or update capability is claimed by this candidate. " +
			"Deleting a bound Resource is refused. A successful unbound delete reports application-visible absence; " +
			"physical media retention or purge is operator policy, not a portable promise. Observed state reports " +
			"dimension, metric, filterKeys, and an eventual total vector count across all namespaces; status outputs " +
			"are not claimed.",
		Fields: []model.Field{
			{
				HCL: "dimension", Wire: "dimension", Kind: model.KindInteger,
				Required: true, Immutable: true,
				Min: model.I64(1), Max: model.I64(vectorIndexCandidateMaxDimension),
				Doc: "Required positive vector length. Every stored and query vector must have exactly this many " +
					"components; each component is converted to IEEE 754 binary32 before storage and a host rejects wrong " +
					"length rather than padding or truncating it.",
				Example: 3, AltExample: 768, CounterExample: 0,
			},
			{
				HCL: "metric", Wire: "metric", Kind: model.KindStringEnum,
				Required: true, Immutable: true, Enum: []string{"cosine"},
				Doc: "Required immutable similarity metric. This candidate supports cosine similarity only; scores are " +
					"higher-is-better and a host rejects another metric instead of silently substituting one.",
				Example: "cosine", CounterExample: "dotproduct",
			},
			{
				HCL: "filter_keys", Wire: "filterKeys", Kind: model.KindStringSet,
				Immutable: true, Default: []string{}, Example: []string{"spaceId"}, AltExample: []string{},
				MaxItems:    vectorIndexCandidateMaxFilterKeys,
				ItemPattern: `^[A-Za-z][A-Za-z0-9_]{0,63}$`,
				Doc: "Optional immutable set of metadata keys callers may use in exact, type-sensitive equality filters. " +
					"Omission means an empty set; a filter key not declared here is rejected. Observed keys use lexical " +
					"ordering matching canonical set defaults. Keys are simple ASCII identifiers and at most eight may be " +
					"declared.",
				CounterExample: []string{"9invalid"},
			},
		},
	}
}

// VectorIndexCandidateInterface returns the unregistered edge.vector
// Interface Definition. Its operation schemas are closed at the JSON object
// boundary and carry the runtime-dependent exact-dimension and finite-number
// checks in their descriptions; the Form supplies the configured dimension.
func VectorIndexCandidateInterface() InterfaceDefinition {
	return InterfaceDefinition{
		APIVersion: InterfaceAPIVersion,
		Kind:       "InterfaceDefinition",
		Name:       VectorIndexCandidateInterfaceName,
		Version:    VectorIndexCandidateInterfaceVersion,
		Title:      "Install-owned vector index",
		Description: "Unpublished edge.vector candidate for an install-owned vector index. The namespace is an " +
			"opaque app-selected partition: omission means the empty namespace and there is no wildcard namespace. " +
			"A key is (namespace, id), and IDs may repeat in different namespaces. Every vector component is converted " +
			"to IEEE 754 binary32 before storage; conversion to a non-finite value or a zero vector norm is " +
			"invalid_spec, and returned values are the canonical binary32 values. A successful mutation acknowledges " +
			"durable acceptance only; get and query visibility, and the observed count, are eventual. Input validation " +
			"for one batch completes before that batch's writes begin, producing zero effects for invalid_spec. Quota " +
			"or unavailable may leave a partial batch; retrying the whole call with the same canonical values and " +
			"metadata is idempotent. The data-only fixtures that exercise filtering run against a fresh VectorIndex " +
			"configured as {dimension:3, metric:'cosine', filterKeys:['spaceId']}. A missing or empty query filter " +
			"imposes no metadata restriction. Query filters apply namespace and declared metadata keys with exact, type-sensitive " +
			"equality joined with AND before approximate nearest-neighbor selection. Results are finite cosine scores in " +
			"[-1,1], sorted " +
			"descending with ties unspecified. This candidate has no insert/create-only operation and no atomic-batch or " +
			"exact-nearest-neighbor promise. After accepted mutations affecting a selected namespace and filter have " +
			"converged and no concurrent writes are present, a successful query returns exactly " +
			"min(topK, eligibleVisibleCount) matches, where eligibleVisibleCount counts unique visible records satisfying " +
			"that namespace and filter; each (namespace, id) appears at most once. Before convergence, eventual reads may " +
			"return fewer matches. Operation failures use only invalid_spec, quota, and unavailable.",
		Semantics: InterfaceSemantics{
			Consistency: "eventual",
			Pagination:  "none",
			Ordering:    "none",
		},
		Limits: map[string]int64{
			"maxDimension":       vectorIndexCandidateMaxDimension,
			"maxIdLength":        vectorIndexCandidateMaxIDLength,
			"maxNamespaceLength": vectorIndexCandidateMaxNamespaceLength,
			"maxBatch":           vectorIndexCandidateMaxBatch,
			"maxTopK":            vectorIndexCandidateMaxTopK,
			"maxMetadataBytes":   vectorIndexCandidateMaxMetadataBytes,
			"maxFilterKeys":      vectorIndexCandidateMaxFilterKeys,
		},
		Operations: []InterfaceOperation{
			vectorIndexUpsertOperation(),
			vectorIndexGetOperation(),
			vectorIndexDeleteOperation(),
			vectorIndexQueryOperation(),
		},
		Fixtures: []InterfaceFixture{
			{
				Name: "upsert-accepts-explicit-namespace",
				Steps: []InterfaceFixtureStep{
					{
						Operation: "upsert",
						Input: map[string]any{
							"namespace": "space:s",
							"vectors": []any{map[string]any{
								"id": "file:f:0", "values": []any{1, 0, 0},
								"metadata": map[string]any{"spaceId": "s", "chunkIndex": 0},
							}},
						},
						Expected: map[string]any{"ids": []any{"file:f:0"}, "count": 1},
					},
				},
			},
			{
				Name: "query-filters-declared-key",
				Steps: []InterfaceFixtureStep{
					{
						Operation: "query",
						Input: map[string]any{
							"namespace": "space:s", "values": []any{1, 0, 0}, "topK": 1,
							"filter":         map[string]any{"spaceId": "s"},
							"returnMetadata": true,
						},
						Expected: map[string]any{"matches": []any{}, "count": 0},
					},
				},
			},
			{
				Name: "empty-namespace-is-default-not-wildcard",
				Steps: []InterfaceFixtureStep{
					{
						Operation: "upsert",
						Input: map[string]any{
							"vectors": []any{map[string]any{
								"id": "default-id", "values": []any{1, 0, 0},
							}},
						},
						Expected: map[string]any{"ids": []any{"default-id"}, "count": 1},
					},
					{
						// Do not assert this read immediately after upsert: visibility is
						// eventual, and static fixtures do not poll.
						Operation: "get",
						Input:     map[string]any{"ids": []any{"default-id"}},
					},
				},
			},
			{
				Name: "delete-returns-exact-requested-namespace",
				Steps: []InterfaceFixtureStep{
					{
						Operation: "delete",
						Input: map[string]any{
							"namespace": "space:s", "ids": []any{"file:f:0"},
						},
						Expected: map[string]any{"ids": []any{"file:f:0"}, "count": 1},
					},
				},
			},
			{
				Name: "duplicate-upsert-id-is-invalid",
				Steps: []InterfaceFixtureStep{
					{
						Operation: "upsert",
						Input: map[string]any{
							"vectors": []any{
								map[string]any{"id": "same", "values": []any{1, 0, 0}},
								map[string]any{"id": "same", "values": []any{0, 1, 0}},
							},
						},
						ExpectedError: "invalid_spec",
					},
				},
			},
		},
	}
}

// renderVectorIndexCandidatePair renders and validates the unpublished Form /
// Interface pair. The candidate Interface is rendered and Core-validated first
// so the Form's providedInterfaces reference carries its exact digest without
// touching the registered InterfaceDefinitions catalog. The public renderer
// adds the forward Binding and WorkerVersion in vector_worker_candidate.go.
func renderVectorIndexCandidatePair() (VectorIndexCandidate, error) {
	ifaceDefinition := VectorIndexCandidateInterface()
	if err := ValidateInterfaceDefinitions([]InterfaceDefinition{ifaceDefinition}); err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate interface authoring: %w", err)
	}
	iface, err := renderInterfaceContract(
		ifaceDefinition.Name, ifaceDefinition.Version, ifaceDefinition,
	)
	if err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate interface: %w", err)
	}

	form := VectorIndexCandidateForm()
	if err := form.Validate(); err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate Form authoring: %w", err)
	}
	// renderForm resolves ProvidedInterfaces through the current registered
	// catalog. Render without one, then attach the candidate's exact digest and
	// re-render the complete Definition through the same Core validator.
	renderedForm, err := renderForm(form, nil)
	if err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate Form: %w", err)
	}
	renderedForm.Definition.ProvidedInterfaces = []formpackage.InterfaceRef{{
		APIVersion:   ifaceDefinition.APIVersion,
		Name:         ifaceDefinition.Name,
		Version:      ifaceDefinition.Version,
		SchemaDigest: iface.SchemaDigest,
	}}
	// The shared model derives import in its historical base lifecycle set. This
	// candidate intentionally claims only the operations that are implemented;
	// no import or update promise is emitted.
	renderedForm.Definition.LifecycleCapabilities = []string{"create", "read", "delete", "observe"}
	renderedForm.Definition.ObservedSchema = vectorIndexObservedSchema()
	renderedForm.DefinitionJSON, err = marshalIndented(renderedForm.Definition)
	if err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate Form JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(renderedForm.DefinitionJSON)); err != nil {
		return VectorIndexCandidate{}, fmt.Errorf("vector index candidate Form Core validation: %w", err)
	}
	return VectorIndexCandidate{Form: renderedForm, Interface: iface}, nil
}

func vectorIndexIDSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"minLength":   1,
		"maxLength":   vectorIndexCandidateMaxIDLength,
		"pattern":     "^[^" + vectorIndexNUL + "]+$",
		"description": "Opaque ID; maxLength counts Unicode code points and NUL is forbidden.",
	}
}

func vectorIndexObservedSchema() map[string]any {
	return map[string]any{
		"$schema":              draft2020,
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"count", "dimension", "filterKeys", "metric"},
		"properties": map[string]any{
			"dimension": map[string]any{
				"type": "integer", "minimum": 1, "maximum": vectorIndexCandidateMaxDimension,
			},
			"metric": map[string]any{"type": "string", "enum": []any{"cosine"}},
			"filterKeys": map[string]any{
				"type": "array", "maxItems": vectorIndexCandidateMaxFilterKeys,
				"uniqueItems": true,
				"items":       map[string]any{"type": "string", "pattern": `^[A-Za-z][A-Za-z0-9_]{0,63}$`},
				"description": "Declared filter keys are emitted in lexical order, matching canonical set-default ordering.",
			},
			"count": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Eventual total number of records across all namespaces; it may lag a completed mutation.",
			},
		},
	}
}

func vectorIndexNamespaceSchema() map[string]any {
	return map[string]any{
		"type":      "string",
		"pattern":   "^[^*" + vectorIndexNUL + "]*$",
		"maxLength": vectorIndexCandidateMaxNamespaceLength,
		"default":   "",
		"description": "Opaque app-selected namespace; omission is the empty namespace, never a wildcard. " +
			"maxLength counts Unicode code points; NUL and '*' are forbidden.",
	}
}

func vectorIndexValuesSchema() map[string]any {
	return map[string]any{
		"type":     "array",
		"minItems": 1,
		"maxItems": vectorIndexCandidateMaxDimension,
		"items": map[string]any{
			"type":        "number",
			"description": "Converted to IEEE 754 binary32; conversion must remain finite.",
		},
		"description": "Components are converted to IEEE 754 binary32 before storage. Conversion to a non-finite " +
			"component or a zero vector norm is invalid_spec; the vector must have exactly the Resource dimension. " +
			"Returned values are the canonical binary32 values.",
	}
}

func vectorIndexMetadataValueSchema() map[string]any {
	return map[string]any{
		"oneOf": []any{
			map[string]any{
				"type":        "string",
				"maxLength":   vectorIndexCandidateMaxMetadataString,
				"description": "String maxLength counts Unicode code points.",
			},
			map[string]any{
				"type":        "number",
				"description": "Finite JSON number; non-finite values are invalid_spec.",
			},
			map[string]any{"type": "boolean"},
			map[string]any{"type": "null"},
		},
	}
}

func vectorIndexMetadataSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"maxProperties":        vectorIndexCandidateMaxMetadataProps,
		"propertyNames":        map[string]any{"type": "string", "pattern": `^[A-Za-z][A-Za-z0-9_]{0,63}$`},
		"additionalProperties": vectorIndexMetadataValueSchema(),
		"description": "Flat scalar metadata only. Its RFC 8785 canonical JSON UTF-8 encoding must be at " +
			"most 8192 bytes; numeric values must be finite. Keys are simple ASCII identifiers and at most 64 " +
			"properties are allowed.",
		"default": map[string]any{},
	}
}

func vectorIndexOutputMetadataSchema() map[string]any {
	schema := vectorIndexMetadataSchema()
	// `default` is an input-materialization hint. Output contracts must not
	// suggest that an omitted return field is fabricated as an empty object.
	delete(schema, "default")
	return schema
}

func vectorIndexRecordSchema() map[string]any {
	return closedObject([]string{"id", "values"}, map[string]any{
		"id":       vectorIndexIDSchema(),
		"values":   vectorIndexValuesSchema(),
		"metadata": vectorIndexMetadataSchema(),
	})
}

func vectorIndexIDsSchema() map[string]any {
	return map[string]any{
		"type":        "array",
		"minItems":    1,
		"maxItems":    vectorIndexCandidateMaxBatch,
		"uniqueItems": true,
		"items":       vectorIndexIDSchema(),
	}
}

func vectorIndexFilterSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"maxProperties":        vectorIndexCandidateMaxFilterKeys,
		"propertyNames":        map[string]any{"type": "string", "pattern": `^[A-Za-z][A-Za-z0-9_]{0,63}$`},
		"additionalProperties": vectorIndexMetadataValueSchema(),
		"description": "Exact, type-sensitive equality terms joined with AND; namespace and these declared keys are " +
			"applied before approximate nearest-neighbor selection. A missing or empty filter imposes no metadata " +
			"restriction. Numeric values must be finite and every key must be declared in the Resource filterKeys set.",
	}
}

func vectorIndexMutationOutputSchema() map[string]any {
	return operationObject([]string{"ids", "count"}, map[string]any{
		"ids": func() map[string]any {
			schema := vectorIndexIDsSchema()
			schema["description"] = "IDs in the accepted request order."
			return schema
		}(),
		"count": map[string]any{
			"type": "integer", "minimum": 1, "maximum": vectorIndexCandidateMaxBatch,
			"description": "For a successful mutation, the number of accepted request records or IDs; it is not a row-existence proof.",
		},
	})
}

func vectorIndexUpsertOperation() InterfaceOperation {
	return InterfaceOperation{
		Name: "upsert",
		Description: "The vectors array contains 1..100 records and is validated in full before any effects; an " +
			"invalid_spec therefore has zero effects. Duplicate IDs in one upsert are invalid_spec. Each key is " +
			"(namespace, id); an existing key is replaced with " +
			"the complete values and metadata after every component is converted to canonical IEEE 754 binary32. " +
			"Omitted metadata is treated as {} and therefore fully clears prior metadata rather than merging it. A " +
			"successful response returns IDs exactly in input order with count equal to len(vectors), and all records " +
			"are durably accepted. Quota or unavailable may leave a partial batch; retrying the whole call with the same " +
			"canonical binary32 values and metadata is idempotent. No cross-provider atomic-batch promise is made.",
		InputSchema: operationObject([]string{"vectors"}, map[string]any{
			"namespace": vectorIndexNamespaceSchema(),
			"vectors": map[string]any{
				"type": "array", "minItems": 1, "maxItems": vectorIndexCandidateMaxBatch,
				"items": vectorIndexRecordSchema(),
			},
		}),
		OutputSchema: vectorIndexMutationOutputSchema(),
		Errors:       vectorIndexErrors(),
		Idempotent:   true,
	}
}

func vectorIndexGetOperation() InterfaceOperation {
	return InterfaceOperation{
		Name: "get",
		Description: "The IDs array contains 1..100 unique IDs in one namespace. Reads are eventual after mutation: " +
			"records that are visible are returned in request ID order and missing IDs are omitted. An omitted namespace " +
			"means only the empty namespace and never all namespaces. Returned records include namespace, canonical " +
			"binary32 values, and metadata; a recently upserted record may be absent until it becomes visible and a " +
			"recently deleted record may remain visible temporarily.",
		InputSchema: operationObject([]string{"ids"}, map[string]any{
			"namespace": vectorIndexNamespaceSchema(),
			"ids":       vectorIndexIDsSchema(),
		}),
		OutputSchema: operationObject([]string{"vectors"}, map[string]any{
			"vectors": map[string]any{
				"type": "array", "maxItems": vectorIndexCandidateMaxBatch,
				"items": closedObject([]string{"id", "namespace", "values", "metadata"}, map[string]any{
					"id": vectorIndexIDSchema(), "namespace": vectorIndexNamespaceSchema(),
					"values": vectorIndexValuesSchema(), "metadata": vectorIndexOutputMetadataSchema(),
				}),
			},
		}),
		Errors:     vectorIndexErrors(),
		Idempotent: true,
	}
}

func vectorIndexDeleteOperation() InterfaceOperation {
	return InterfaceOperation{
		Name: "delete",
		Description: "The IDs array contains 1..100 unique IDs in one namespace. The operation is idempotent and " +
			"unknown IDs are a no-op. A successful response returns the exact accepted request IDs in request order, " +
			"including unknown IDs, with count equal to len(ids); neither IDs nor count is an existence proof. Omitted " +
			"namespace addresses only the empty namespace. A successful unbound Resource delete separately reports " +
			"application-visible absence; physical media retention is operator policy. Quota or unavailable may leave a " +
			"partial batch; retrying the whole call with the same IDs is safe and idempotent.",
		InputSchema: operationObject([]string{"ids"}, map[string]any{
			"namespace": vectorIndexNamespaceSchema(),
			"ids":       vectorIndexIDsSchema(),
		}),
		OutputSchema: vectorIndexMutationOutputSchema(),
		Errors:       vectorIndexErrors(),
		Idempotent:   true,
	}
}

func vectorIndexMatchSchema() map[string]any {
	return closedObject([]string{"id", "namespace", "score"}, map[string]any{
		"id":        vectorIndexIDSchema(),
		"namespace": vectorIndexNamespaceSchema(),
		"score": map[string]any{
			"type":        "number",
			"minimum":     -1,
			"maximum":     1,
			"description": "Finite cosine similarity in [-1,1].",
		},
		"metadata": func() map[string]any {
			schema := vectorIndexOutputMetadataSchema()
			schema["description"] = "Present on every match only when returnMetadata is true; absent when false or omitted."
			return schema
		}(),
		"values": func() map[string]any {
			schema := vectorIndexValuesSchema()
			schema["description"] = "Present on every match only when returnValues is true; absent when false or omitted."
			return schema
		}(),
	})
}

func vectorIndexQueryOperation() InterfaceOperation {
	return InterfaceOperation{
		Name: "query",
		Description: "Query one namespace with a configured-dimension vector converted to finite, nonzero-norm IEEE " +
			"754 binary32. Namespace and declared metadata filter keys use exact, type-sensitive AND semantics before " +
			"approximate nearest-neighbor selection. A missing or empty filter imposes no metadata restriction. The " +
			"provider returns at most topK matches with finite cosine scores in [-1,1], sorted descending; ties are " +
			"unspecified. Query visibility is eventual after mutation, and deleted records eventually become absent from query " +
			"results. After accepted mutations for the selected namespace and filter have converged and no concurrent writes " +
			"are present, the result contains exactly min(topK, eligibleVisibleCount) unique matches, where " +
			"eligibleVisibleCount counts visible records satisfying the namespace and filter; before convergence it may contain " +
			"fewer. If returnMetadata or returnValues is true, that field appears on every match; when false or omitted, the " +
			"field is absent from every match.",
		InputSchema: operationObject([]string{"values", "topK"}, map[string]any{
			"namespace":      vectorIndexNamespaceSchema(),
			"values":         vectorIndexValuesSchema(),
			"topK":           map[string]any{"type": "integer", "minimum": 1, "maximum": vectorIndexCandidateMaxTopK},
			"filter":         vectorIndexFilterSchema(),
			"returnMetadata": map[string]any{"type": "boolean", "default": false},
			"returnValues":   map[string]any{"type": "boolean", "default": false},
		}),
		OutputSchema: operationObject([]string{"matches", "count"}, map[string]any{
			"matches": map[string]any{
				"type": "array", "maxItems": vectorIndexCandidateMaxTopK,
				"description": "After accepted mutations affecting the selected namespace and filter have converged and no " +
					"concurrent writes are present, query cardinality is exactly min(topK, eligibleVisibleCount); each " +
					"(namespace, id) appears at most once. Before convergence, eventual visibility may return fewer matches.",
				"items": vectorIndexMatchSchema(),
			},
			"count": map[string]any{
				"type": "integer", "minimum": 0, "maximum": vectorIndexCandidateMaxTopK,
				"description": "Exactly len(matches), never greater than topK.",
			},
		}),
		Errors:     vectorIndexErrors(),
		Idempotent: true,
	}
}

func vectorIndexErrors() []string {
	return []string{"invalid_spec", "quota", "unavailable"}
}
