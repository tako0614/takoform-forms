package edgeformcatalog

import (
	"fmt"
	"sort"
	"strings"

	model "github.com/tako0614/takoform-forms/internal/currentformmodel"
	"github.com/tako0614/takoform/formpackage"
)

const (
	EdgeContainerHTTPCandidateFormKind    = "ContainerService"
	EdgeContainerHTTPCandidateFormVersion = "0.1.0"

	containerEnvironmentMaxEntries     = 64
	containerEnvironmentMaxValueLength = 4096
	containerSensitiveNameMaxItems     = 64
	containerHealthPathMaxLength       = 2048
	containerRevisionMaxLength         = 128
)

const (
	containerImageDigestPattern = `^(?:[a-z0-9]+(?:[.-][a-z0-9]+)*(?::(?:[1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]))?/)?[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?@sha256:[0-9a-f]{64}$`
	containerHealthPathPattern  = `^/(?:[A-Za-z0-9._~!$&'()*+,;=:@-]|%[0-9A-Fa-f]{2})+(?:/(?:[A-Za-z0-9._~!$&'()*+,;=:@-]|%[0-9A-Fa-f]{2})+)*$|^/$`
	containerRevisionPattern    = `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`
)

// RenderedPackageFile is one exact regular payload file in a Form Package
// closure. Content is the verbatim UTF-8 file body, so a consumer can assemble
// and Core-verify the local candidate without re-authoring or re-serializing
// any package member.
type RenderedPackageFile struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
	Content   string `json:"content"`
}

// RenderedFormPackage carries the exact unpublished package index and its
// listed payloads. The package digest is content identity only; this renderer
// does not create a publication tag or trust statement.
type RenderedFormPackage struct {
	IndexJSON     string                `json:"packageIndexJson"`
	PackageDigest string                `json:"packageDigest"`
	Files         []RenderedPackageFile `json:"files"`
}

// ContainerLifecycleFixture is a local integration scenario emitted beside
// the exact package closure. These scenarios are not Form-package conformance
// documents or a claim that a Host has implemented the lifecycle.
type ContainerLifecycleFixture struct {
	Name     string `json:"name"`
	Negative bool   `json:"negative"`
	Scenario string `json:"scenario"`
	Expected string `json:"expected"`
}

// EdgeContainerServiceFormCandidate is a local-only Form package export. The
// separate container.http Interface and Binding are withheld until their
// semantics fit released Core's mandatory consistency vocabulary.
type EdgeContainerServiceFormCandidate struct {
	PublicationStatus      string                      `json:"publicationStatus"`
	ContractClosure        string                      `json:"contractClosure"`
	Form                   RenderedForm                `json:"form"`
	Package                RenderedFormPackage         `json:"package"`
	LocalLifecycleFixtures []ContainerLifecycleFixture `json:"localLifecycleFixtures"`
}

// RenderEdgeContainerServiceFormCandidate renders the unpublished
// ContainerService Form package and local lifecycle scenarios without
// registering it. Interface/Binding emission is intentionally held until the
// mandatory Core consistency field can be satisfied without claiming a false
// state guarantee.
func RenderEdgeContainerServiceFormCandidate() (EdgeContainerServiceFormCandidate, error) {
	formModel := containerServiceCandidateForm()
	if err := formModel.Validate(); err != nil {
		return EdgeContainerServiceFormCandidate{}, fmt.Errorf("ContainerService candidate authoring: %w", err)
	}
	form, err := renderForm(formModel, newTargetContractResolver())
	if err != nil {
		return EdgeContainerServiceFormCandidate{}, fmt.Errorf("ContainerService candidate Form: %w", err)
	}
	// The shared identity model includes historical import capability by
	// default. Container import/adoption has no defined identity or fencing
	// semantics in this local slice, so claim only its bounded CRUD/observe set.
	form.Definition.LifecycleCapabilities = []string{"create", "read", "update", "delete", "observe"}
	if err := appendContainerNegativeFixture(&form, "sensitive-requirement-values-not-names", map[string]any{
		"image":                 containerImageExample(),
		"httpPort":              8080,
		"healthPath":            "/health",
		"environment":           map[string]any{"APP_MODE": "production"},
		"workloadRevision":      "revision-1",
		"outboundInternet":      false,
		"requiredSensitiveVars": []any{"API_TOKEN=plaintext"},
	}); err != nil {
		return EdgeContainerServiceFormCandidate{}, err
	}
	if err := appendContainerNegativeFixture(&form, "http-port-out-of-range", map[string]any{
		"image":                 containerImageExample(),
		"httpPort":              65536,
		"healthPath":            "/health",
		"environment":           map[string]any{"APP_MODE": "production"},
		"workloadRevision":      "revision-1",
		"outboundInternet":      false,
		"requiredSensitiveVars": []any{},
	}); err != nil {
		return EdgeContainerServiceFormCandidate{}, err
	}
	if err := appendContainerNegativeFixture(&form, "environment-value-too-long", map[string]any{
		"image":                 containerImageExample(),
		"httpPort":              8080,
		"healthPath":            "/health",
		"environment":           map[string]any{"APP_MODE": strings.Repeat("x", containerEnvironmentMaxValueLength+1)},
		"workloadRevision":      "revision-1",
		"outboundInternet":      false,
		"requiredSensitiveVars": []any{},
	}); err != nil {
		return EdgeContainerServiceFormCandidate{}, err
	}
	if err := appendContainerNegativeFixture(&form, "registry-port-out-of-range", map[string]any{
		"image":                 "registry.example.com:99999/app@sha256:" + strings.Repeat("a", 64),
		"httpPort":              8080,
		"healthPath":            "/health",
		"environment":           map[string]any{"APP_MODE": "production"},
		"workloadRevision":      "revision-1",
		"outboundInternet":      false,
		"requiredSensitiveVars": []any{},
	}); err != nil {
		return EdgeContainerServiceFormCandidate{}, err
	}
	if err := appendContainerNegativeFixture(&form, "health-path-empty-segment", map[string]any{
		"image":                 containerImageExample(),
		"httpPort":              8080,
		"healthPath":            "//health",
		"environment":           map[string]any{"APP_MODE": "production"},
		"workloadRevision":      "revision-1",
		"outboundInternet":      false,
		"requiredSensitiveVars": []any{},
	}); err != nil {
		return EdgeContainerServiceFormCandidate{}, err
	}
	form.DefinitionJSON, err = marshalIndented(form.Definition)
	if err != nil {
		return EdgeContainerServiceFormCandidate{}, fmt.Errorf("ContainerService candidate Definition JSON: %w", err)
	}
	if _, err := formpackage.ValidateDefinition([]byte(form.DefinitionJSON)); err != nil {
		return EdgeContainerServiceFormCandidate{}, fmt.Errorf("ContainerService candidate Core validation: %w", err)
	}

	formPackage, err := renderContainerFormPackage(form)
	if err != nil {
		return EdgeContainerServiceFormCandidate{}, fmt.Errorf("ContainerService candidate package closure: %w", err)
	}
	return EdgeContainerServiceFormCandidate{
		PublicationStatus: "UNPUBLISHED",
		ContractClosure:   "form-only; container.http Interface and module-worker.container-http Binding are withheld",
		Form:              form,
		Package:           formPackage,
		LocalLifecycleFixtures: []ContainerLifecycleFixture{
			{
				Name: "healthy-update-cuts-over", Scenario: "A requested workload revision starts and its configured health path succeeds.",
				Expected: "Only then does the exact requested revision become serving.",
			},
			{
				Name: "health-failure-preserves-old-serving", Negative: true,
				Scenario: "The new workload revision fails startup or its configured health check.",
				Expected: "The new revision never receives serving traffic and the previous serving execution revision of the same Resource UID remains selected.",
			},
			{
				Name: "lost-ack-mismatched-readback-is-not-retried", Negative: true,
				Scenario: "An update acknowledgement is lost and readback does not match the same UID, desired generation, and operation identity.",
				Expected: "Do not adopt or retry the mutation; preserve the prior serving incarnation and report unresolved outcome.",
			},
			{
				Name: "delete-stale-incarnation-is-refused", Negative: true,
				Scenario: "A delete observes an incarnation UID that differs from the exact incarnation fenced by the request.",
				Expected: "Refuse deletion and leave the newer incarnation untouched.",
			},
			{
				Name: "restart-recovers-from-durable-operation-record", Scenario: "The API process restarts while a workload operation is unsettled.",
				Expected: "Recover by reconciling durable desired/observed generations and operation identity; do not control a child process through an API request.",
			},
		},
	}, nil
}

// ValidateContainerSensitiveRequirements is the initial local execution
// slice's fail-closed preflight. It deliberately accepts names only as input
// metadata and refuses every nonempty list because the current Host API cannot
// provide secret material. It does not inspect or classify environment values;
// the field's nonsecret classification and caller responsibility remain separate.
func ValidateContainerSensitiveRequirements(desired map[string]any) error {
	value, found := desired["requiredSensitiveVars"]
	if !found || value == nil {
		return nil
	}
	var count int
	switch names := value.(type) {
	case []any:
		count = len(names)
	case []string:
		count = len(names)
	default:
		return fmt.Errorf("ContainerService requiredSensitiveVars must be an array of names")
	}
	if count > 0 {
		return fmt.Errorf("ContainerService execution refuses %d requiredSensitiveVars: current Host API has no secret-material input", count)
	}
	return nil
}

func containerServiceCandidateForm() model.Form {
	return model.Form{
		Family: model.Family{Group: Family.Group},
		Kind:   EdgeContainerHTTPCandidateFormKind, Slug: "container-service",
		Role: model.RoleIdentity, RequiresHostAPI: stableHostLane,
		DefinitionVersion: EdgeContainerHTTPCandidateFormVersion,
		Title:             "Container Service",
		Description: "One logical private HTTP Container service identified by an OCI image pinned to a sha256 " +
			"manifest digest. The Host provisions and operates exact execution revisions using Host Offering-owned " +
			"capacity and placement; those are never customer desired fields. This Form-only local candidate does " +
			"not publish caller-facing HTTP invocation; container.http and its Worker Binding remain a separate " +
			"unresolved contract gap. The service exposes the declared HTTP port and health path. An update starts " +
			"the exact requested workload revision, verifies health before moving serving traffic, and preserves the " +
			"previous serving execution revision of the same Resource incarnation if startup or health fails. A lost operation " +
			"acknowledgement is resolved by readback of the same resource UID, desired generation, and operation " +
			"identity before retry; a mismatched identity is never adopted. Delete is fenced by the exact observed " +
			"incarnation. Durable restart recovery reconciles persisted desired/observed generations and operation " +
			"identity; it is not process-control through the request API. `requiredSensitiveVars` contains names only, " +
			"never secret values. This initial local executable slice refuses any nonempty requiredSensitiveVars " +
			"because the current Host API cannot supply secret material. `environment` is classified as ordinary " +
			"nonsecret desired-state configuration and is persisted/read back as part of ResourceSpec. Callers MUST NOT " +
			"put secrets in this field; the schema does not automatically detect or reject secret values. A separate " +
			"sealed, generation-bound secret transport is not available. `outboundInternet` is explicitly false when " +
			"omitted and becomes true only when requested.",
		Fields: []model.Field{
			{HCL: "image", Wire: "image", Kind: model.KindString, Required: true, Immutable: false,
				Pattern: containerImageDigestPattern, MaxLength: 512,
				Doc:     "OCI image reference including its immutable sha256 manifest digest; mutable tags without a digest are refused.",
				Example: containerImageExample(), AltExample: "ghcr.io/example/app:v2@sha256:" + strings.Repeat("b", 64),
				CounterExample: "ghcr.io/example/app:latest"},
			{HCL: "http_port", Wire: "httpPort", Kind: model.KindInteger, Required: true,
				Min: model.I64(1), Max: model.I64(65535),
				Doc:     "The private HTTP listener port inside the image; public port mapping and placement are Host-owned.",
				Example: 8080, AltExample: 9000, CounterExample: 0},
			{HCL: "health_path", Wire: "healthPath", Kind: model.KindString, Required: true,
				Pattern: containerHealthPathPattern, MaxLength: containerHealthPathMaxLength,
				Doc:     "Absolute path used by the Host to establish readiness before an update is made serving.",
				Example: "/health", AltExample: "/ready", CounterExample: "health"},
			{HCL: "environment", Wire: "environment", Kind: model.KindStringMap, Required: false,
				Default: map[string]any{}, MaxProperties: containerEnvironmentMaxEntries,
				ItemPattern: `^[\s\S]*$`, MaxLength: containerEnvironmentMaxValueLength,
				Doc:            "At most 64 ordinary bounded desired-state strings persisted/read back as ResourceSpec. Callers MUST NOT put secrets here; automatic secret-value detection/rejection is not provided.",
				Example:        map[string]any{"APP.MODE": "production", "APP_MODE": "standard"},
				CounterExample: containerEnvironmentOverflow()},
			{HCL: "workload_revision", Wire: "workloadRevision", Kind: model.KindString, Required: true,
				Pattern: containerRevisionPattern, MaxLength: containerRevisionMaxLength,
				Doc:     "Author-declared bounded revision identity for this desired workload; it is part of the exact operation fence, not a Host UID or generation.",
				Example: "revision-1", AltExample: "revision-2", CounterExample: strings.Repeat("r", containerRevisionMaxLength+1)},
			{HCL: "outbound_internet", Wire: "outboundInternet", Kind: model.KindBoolean, Required: false,
				Default: false,
				Doc:     "Whether this workload may initiate outbound Internet connections. Omission means false.",
				Example: false, AltExample: true},
			{HCL: "required_sensitive_vars", Wire: "requiredSensitiveVars", Kind: model.KindStringSet, Required: false,
				Default: []any{}, ItemPattern: model.PortableMapKeyPattern,
				MaxItems: containerSensitiveNameMaxItems,
				Doc:      "Names of required sensitive process variables only; values are supplied outside portable desired state. This initial executable local slice refuses a nonempty set.",
				Example:  []any{}, CounterExample: []any{"not/a/portable-name"}},
		},
	}
}

func containerImageExample() string {
	return "ghcr.io/example/app@sha256:" + strings.Repeat("a", 64)
}

func containerEnvironmentOverflow() map[string]any {
	values := make(map[string]any, containerEnvironmentMaxEntries+1)
	for index := 0; index <= containerEnvironmentMaxEntries; index++ {
		values[fmt.Sprintf("VAR_%02d", index)] = "bounded"
	}
	return values
}

func appendContainerNegativeFixture(form *RenderedForm, name string, desired map[string]any) error {
	path := "fixtures/negative-" + name + ".json"
	for _, existing := range form.Definition.NegativeFixtures {
		if existing.InputPath == path {
			return fmt.Errorf("ContainerService candidate repeats negative fixture %q", name)
		}
	}
	form.Definition.NegativeFixtures = append(form.Definition.NegativeFixtures, formpackage.NegativeFixture{
		Name: "reject-" + name, Stage: "desired", InputPath: path,
		ExpectedFailure: "schema_validation_failed",
	})
	form.Fixtures["negative-"+name+".json"] = desired
	return nil
}

func renderContainerFormPackage(form RenderedForm) (RenderedFormPackage, error) {
	fixtureNames := make([]string, 0, len(form.Fixtures))
	for name := range form.Fixtures {
		fixtureNames = append(fixtureNames, name)
	}
	sort.Strings(fixtureNames)
	payloads := make(map[string][]byte, len(fixtureNames)+1)
	payloads["definition.json"] = []byte(form.DefinitionJSON)
	for _, name := range fixtureNames {
		raw, err := marshalIndented(form.Fixtures[name])
		if err != nil {
			return RenderedFormPackage{}, fmt.Errorf("fixture %s: %w", name, err)
		}
		payloads["fixtures/"+name] = []byte(raw)
	}
	paths := make([]string, 0, len(payloads))
	for path := range payloads {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := make([]formpackage.PackageFile, 0, len(paths))
	for _, path := range paths {
		raw := payloads[path]
		mediaType := "application/json"
		if path == "definition.json" {
			mediaType = formpackage.DefinitionMediaType
		}
		files = append(files, formpackage.PackageFile{
			Path: path, MediaType: mediaType, Size: int64(len(raw)), Digest: formpackage.DigestBytes(raw),
		})
	}
	schemaDigest, err := formpackage.DigestCanonicalJSON([]byte(form.DefinitionJSON))
	if err != nil {
		return RenderedFormPackage{}, fmt.Errorf("Form Definition schema digest: %w", err)
	}
	formRef := formpackage.FormRef{
		APIVersion:        form.Definition.APIVersion,
		Kind:              form.Definition.Kind,
		DefinitionVersion: form.Definition.DefinitionVersion,
		SchemaDigest:      schemaDigest,
	}
	index := formpackage.PackageIndex{
		APIVersion:     formpackage.VersionlessFamilyPackageAPIVersion,
		Kind:           formpackage.PackageKind,
		FormRef:        formRef,
		DefinitionPath: "definition.json",
		Files:          files,
	}
	indexJSON, err := marshalIndented(index)
	if err != nil {
		return RenderedFormPackage{}, fmt.Errorf("package index: %w", err)
	}
	if _, err := formpackage.ValidatePackageIndex([]byte(indexJSON)); err != nil {
		return RenderedFormPackage{}, fmt.Errorf("package index Core validation: %w", err)
	}
	packageDigest, err := formpackage.DigestCanonicalJSON([]byte(indexJSON))
	if err != nil {
		return RenderedFormPackage{}, fmt.Errorf("package index digest: %w", err)
	}
	fileClosure := make([]RenderedPackageFile, 0, len(paths))
	for _, file := range files {
		fileClosure = append(fileClosure, RenderedPackageFile{
			Path: file.Path, MediaType: file.MediaType, Size: file.Size, Digest: file.Digest,
			Content: string(payloads[file.Path]),
		})
	}
	return RenderedFormPackage{IndexJSON: indexJSON, PackageDigest: packageDigest, Files: fileClosure}, nil
}
