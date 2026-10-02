package publishertrust

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/tako0614/takoform/formpackage"
	"github.com/tako0614/takoform/trust"
)

const canonicalGenesis = `{"apiVersion":"trust.forms.takoform.com/v1","checkpointVersion":"0.0.0","entries":[],"kind":"FormPackageRevocationCheckpoint","previousCheckpointDigest":null,"sequence":0}`

func TestPublisherTrustUsesExactReleasedCore(t *testing.T) {
	t.Parallel()
	exact := &debug.BuildInfo{Deps: []*debug.Module{{
		Path:    "github.com/tako0614/takoform",
		Version: CoreVersion,
	}}}
	if err := validateCoreBuildInfo(exact); err != nil {
		t.Fatalf("exact released Core was rejected: %v", err)
	}
	for name, build := range map[string]*debug.BuildInfo{
		"missing": {},
		"wrong version": {Deps: []*debug.Module{{
			Path:    "github.com/tako0614/takoform",
			Version: "v1.0.1",
		}}},
		"replacement": {Deps: []*debug.Module{{
			Path:    "github.com/tako0614/takoform",
			Version: CoreVersion,
			Replace: &debug.Module{Path: "../takoform", Version: "(devel)"},
		}}},
	} {
		if err := validateCoreBuildInfo(build); err == nil {
			t.Fatalf("%s Core build unexpectedly passed", name)
		}
	}
}

func TestReadAbandonedPrepublicationRequiresTheExactSingletonRecord(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	record, err := ReadAbandonedPrepublication(repositoryRoot)
	if err != nil {
		t.Fatalf("read abandoned prepublication record: %v", err)
	}
	if record.Format != AbandonedPrepublicationFormat ||
		record.Family != Family ||
		record.SetID != AbandonedPrepublicationSetID ||
		record.Disposition != AbandonedPrepublicationDisposition ||
		len(record.EvidenceOnlyPackages) != 3 {
		t.Fatalf("unexpected abandoned prepublication record: %+v", record)
	}
	if record.EvidenceOnlyPackages[0].FormRef.Kind != "ObjectBucket" {
		t.Fatalf("first evidence-only package = %+v, want ObjectBucket", record.EvidenceOnlyPackages[0])
	}

	fixture := t.TempDir()
	manifestPath := filepath.Join(fixture, filepath.FromSlash(AbandonedPrepublicationPath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(AbandonedPrepublicationPath)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["evidenceOnlyPackages"] = append(manifest["evidenceOnlyPackages"].([]any), map[string]any{})
	mutated, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAbandonedPrepublication(fixture); err == nil || !strings.Contains(err.Error(), "exactly 3 evidence-only") {
		t.Fatalf("singleton expansion error = %v, want exact-count refusal", err)
	}
}

func TestVerifyPublishedSetClassifiesAbandonedEvidenceOnlyPackages(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	setRoot := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), AbandonedPrepublicationSetID)
	report, err := VerifyPublishedSet(repositoryRoot, setRoot)
	if err != nil {
		t.Fatalf("verify abandoned publisher set: %v", err)
	}
	if report.Disposition != AbandonedPrepublicationDisposition {
		t.Fatalf("disposition = %q, want %q", report.Disposition, AbandonedPrepublicationDisposition)
	}
	if len(report.EvidenceOnlyPackages) != 3 {
		t.Fatalf("evidence-only package count = %d, want 3", len(report.EvidenceOnlyPackages))
	}
	for _, entry := range report.EvidenceOnlyPackages {
		if entry.FormRef.Kind != "ObjectBucket" && entry.FormRef.Kind != "WorkerDeployment" && entry.FormRef.Kind != "WorkerVersion" {
			t.Fatalf("unexpected evidence-only package: %+v", entry)
		}
	}
}

func TestPrepareSigningRequestEmitsExactCoreSubjectsAndRefusesOverwrite(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	output := filepath.Join(t.TempDir(), "request")

	report, err := PrepareSigningRequest(repositoryRoot, output)
	if err != nil {
		t.Fatalf("prepare signing request: %v", err)
	}
	if report.Status != SigningRequiredStatus || report.PackageCount != 17 {
		t.Fatalf("unexpected preparation report: %+v", report)
	}
	if len(report.Subjects) != report.PackageCount+1 {
		t.Fatalf("subject count = %d, want %d", len(report.Subjects), report.PackageCount+1)
	}

	genesis, err := os.ReadFile(filepath.Join(output, RevocationCheckpointPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(genesis) != canonicalGenesis {
		t.Fatalf("genesis bytes = %q, want %q", genesis, canonicalGenesis)
	}

	packageSubject := filepath.Join(
		output,
		"packages",
		"k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2nn5shk3dfk5xxe23foi",
		"sha256-931eda33c673a640530b81779a5821ed27b9244c9f13dec9660867173aa69405",
		PackageIndexName,
	)
	actual, err := os.ReadFile(packageSubject)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(
		repositoryRoot,
		"forms",
		"releases",
		"k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2nn5shk3dfk5xxe23foi",
		"sha256-931eda33c673a640530b81779a5821ed27b9244c9f13dec9660867173aa69405",
		PackageIndexName,
	))
	if err != nil {
		t.Fatal(err)
	}
	want, err := formpackage.Canonicalize(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatal("prepared package-index subject is not the exact Core canonical bytes")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(packageSubject), PackageBundleName)); !os.IsNotExist(err) {
		t.Fatalf("prepare unexpectedly created a signature bundle: %v", err)
	}

	if _, err := PrepareSigningRequest(repositoryRoot, output); err == nil || !strings.Contains(err.Error(), "refusing to replace") {
		t.Fatalf("second prepare error = %v, want create-only refusal", err)
	}
}

func TestPrepareContinuationKeepsSignedCheckpointAndSignsExplicitActiveRoster(t *testing.T) {
	t.Parallel()
	repositoryRoot, paths, _ := syntheticActiveRoster(t, 2, "")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	output := filepath.Join(t.TempDir(), "continuation")
	abandoned := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), AbandonedPrepublicationSetID)
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, abandoned, paths, filepath.Join(t.TempDir(), "abandoned")); err == nil || !strings.Contains(err.Error(), "evidence-only predecessor") {
		t.Fatalf("abandoned predecessor error = %v", err)
	}
	duplicate := append([]string(nil), paths...)
	duplicate[len(duplicate)-1] = paths[0]
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, duplicate, filepath.Join(t.TempDir(), "duplicate")); err == nil || !strings.Contains(err.Error(), "repeats package identity") {
		t.Fatalf("duplicate successor package error = %v", err)
	}
	report, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, output)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != ContinuationMode || report.PreviousSetID != "e7f8a39311dd011b8467e97e7f300cabb9a6b06c" || report.PackageCount != 19 || report.RevocationTag != "" || len(report.Subjects) != 20 {
		t.Fatalf("unexpected continuation report: %+v", report)
	}
	for _, relative := range []string{RevocationCheckpointPath, RevocationBundlePath} {
		want, err := os.ReadFile(filepath.Join(previousSet, relative))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(output, relative))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("inherited %s changed", relative)
		}
	}
	for _, subject := range report.Subjects {
		if subject.Role == "revocation-checkpoint" {
			t.Fatal("continuation attempted to re-sign checkpoint")
		}
	}
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, output); err == nil || !strings.Contains(err.Error(), "refusing to replace") {
		t.Fatalf("second continuation preparation error = %v", err)
	}
}

func TestPrepareContinuationRejectsCallerInventedActiveRoster(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	current, err := discoverPackages(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(current))
	for _, value := range current {
		paths = append(paths, value.locator.SourcePath)
	}
	abandoned, err := ReadAbandonedPrepublication(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := readRetainedPackageEntries(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	substitution := append([]string(nil), paths...)
	for index, value := range current {
		if value.candidate.FormRef.Kind == "WorkerDeployment" {
			substitution[index] = retained[0].SourcePath
			break
		}
	}
	for name, roster := range map[string][]string{
		"abandoned-only": {abandoned.EvidenceOnlyPackages[0].SourcePath},
		"omission":       paths[:len(paths)-1],
		"substitution":   substitution,
	} {
		if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, roster, filepath.Join(t.TempDir(), name)); err == nil || (!strings.Contains(err.Error(), "current-family index") && !strings.Contains(err.Error(), "abandoned evidence-only")) {
			t.Fatalf("%s caller-invented roster error = %v, want publisher selection refusal", name, err)
		}
	}
}

func TestPrepareContinuationRequiresPinnedSelectedCandidateProjection(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*testing.T, string){
		"candidate-set digest fork": func(t *testing.T, repositoryRoot string) {
			path := filepath.Join(repositoryRoot, filepath.FromSlash(candidateSetSource))
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"index count fork": func(t *testing.T, repositoryRoot string) {
			path := filepath.Join(repositoryRoot, filepath.FromSlash(currentFamilyIndexSource))
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var index currentFamilyIndex
			if err := json.Unmarshal(raw, &index); err != nil {
				t.Fatal(err)
			}
			index.Families[0].FormCount--
			raw, err = json.Marshal(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			repositoryRoot, paths, _ := syntheticActiveRoster(t, 0, "")
			mutate(t, repositoryRoot)
			previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), originalPublishedSetID)
			if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, filepath.Join(t.TempDir(), "request")); err == nil || !strings.Contains(err.Error(), "current-family index") {
				t.Fatalf("forked publisher selection error = %v", err)
			}
		})
	}
}

func TestPrepareContinuationRejectsTwoSelectedActiveVersionsOfOneKind(t *testing.T) {
	t.Parallel()
	repositoryRoot, _, replacement := syntheticActiveRoster(t, 0, "ModuleWorker")
	current, err := discoverActivePackages(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), originalPublishedSetID)
	previous, err := discoverPublishedSetPackages(repositoryRoot, previousSet)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range previous {
		if value.candidate.Kind == "ModuleWorker" {
			current = append(current, value)
			break
		}
	}
	projectSelectedCandidates(t, repositoryRoot, current)
	paths := make([]string, 0, len(current))
	for _, value := range current {
		paths = append(paths, value.locator.SourcePath)
	}
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, filepath.Join(t.TempDir(), "request")); err == nil || !strings.Contains(err.Error(), "repeats active Form kind ModuleWorker") {
		t.Fatalf("two active versions of ModuleWorker error = %v", err)
	}
	if len(replacement) != 1 || replacement[0].candidate.FormRef.DefinitionVersion != "0.2.0" {
		t.Fatal("fixture lacks the second active ModuleWorker version")
	}
}

func TestContinuationCanReplaceActiveDefinitionVersionWithoutChangingHistory(t *testing.T) {
	t.Parallel()
	repositoryRoot, paths, replacement := syntheticActiveRoster(t, 0, "ModuleWorker")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	previousPackages, err := discoverPublishedSetPackages(repositoryRoot, previousSet)
	if err != nil {
		t.Fatal(err)
	}
	var removed verifiedCandidate
	for _, value := range previousPackages {
		if value.candidate.FormRef.Kind == "ModuleWorker" {
			removed = value
		}
	}
	if removed.locator.Tag == "" {
		t.Fatal("fixture has no previous ModuleWorker")
	}
	oldBundle, err := os.ReadFile(filepath.Join(previousSet, filepath.FromSlash(packageBundlePath(removed.locator))))
	if err != nil {
		t.Fatal(err)
	}
	oldSetFiles, err := inventoryRegularFiles(previousSet)
	if err != nil {
		t.Fatal(err)
	}
	oldSetBytes := make(map[string][]byte, len(oldSetFiles))
	for _, relative := range oldSetFiles {
		oldSetBytes[relative], err = os.ReadFile(filepath.Join(previousSet, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "version-replacement")
	report, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, output)
	if err != nil {
		t.Fatalf("prepare active definition-version replacement: %v", err)
	}
	if report.Mode != ContinuationMode || report.PackageCount != len(previousPackages) || len(report.Subjects) != len(previousPackages)+1 {
		t.Fatalf("replacement did not retain exact active roster size: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(packageSubjectPath(removed.locator)))); !os.IsNotExist(err) {
		t.Fatalf("replaced version remained in active signed set: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(previousSet, filepath.FromSlash(packageBundlePath(removed.locator)))); err != nil || !bytes.Equal(got, oldBundle) {
		t.Fatalf("previous published package bundle changed: %v", err)
	}
	for relative, want := range oldSetBytes {
		got, err := os.ReadFile(filepath.Join(previousSet, filepath.FromSlash(relative)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("previous signed set file %s changed: %v", relative, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(output, RevocationCheckpointPath)); err != nil || string(got) != canonicalGenesis {
		t.Fatalf("replacement changed inherited checkpoint: %v", err)
	}
	const replacementCommit = "dddddddddddddddddddddddddddddddddddddddd"
	for _, subject := range report.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(output, filepath.FromSlash(bundle)), []byte("synthetic-version-bundle"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	active := make([]verifiedCandidate, 0, len(paths))
	for _, sourcePath := range paths {
		value, err := verifyReleaseCandidate(filepath.Join(repositoryRoot, filepath.FromSlash(sourcePath)), sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		active = append(active, value)
	}
	synthetic := func(subject, bundle, root []byte, policy trust.PublisherPolicy) (trust.BundleVerification, error) {
		if string(bundle) != "synthetic-version-bundle" {
			return trust.BundleVerification{}, fmt.Errorf("unexpected synthetic bundle")
		}
		return trust.BundleVerification{Status: trust.VerifiedStatus, SubjectDigest: formpackage.DigestBytes(subject), BundleDigest: formpackage.DigestBytes(bundle), TrustedRootDigest: formpackage.DigestBytes(root), OIDCIssuer: policy.OIDCIssuer, SourceRepository: policy.SourceRepository, Workflow: policy.Workflow, Ref: policy.Ref, PublisherIdentity: policy.Identity(), SourceCommit: replacementCommit, WorkflowCommit: replacementCommit, BuildConfigCommit: replacementCommit, TransparencyLogVerified: true, TransparencyLogThreshold: 1}, nil
	}
	verified, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, replacementCommit, true, active, synthetic)
	if err != nil || verified.report.PackageCount != 17 || verified.report.PreviousSetID != filepath.Base(previousSet) || len(verified.report.SetHistory) != 2 {
		t.Fatalf("verify explicit active version replacement with test-only signature seam: report=%+v error=%v", verified.report, err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, replacementCommit); err == nil {
		t.Fatal("public Core verifier accepted synthetic version-replacement signatures")
	}
	for _, relative := range []string{LineageSubjectPath, LineageBundlePath} {
		if err := os.Remove(filepath.Join(output, relative)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, replacementCommit, true, active, synthetic); err == nil {
		t.Fatal("same-count version replacement without signed lineage was accepted as a legacy set")
	}
	if report.PackageCount != 17 || len(replacement) != 1 || replacement[0].candidate.FormRef.DefinitionVersion != "0.2.0" {
		t.Fatal("fixture did not select the replacement Form version")
	}
	if _, err := VerifyPublishedSet(repositoryRoot, filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), AbandonedPrepublicationSetID)); err != nil {
		t.Fatalf("new active version reclassified old evidence-only history: %v", err)
	}
}

func TestPublishedContinuationRejectsSetLineageCycleBeforeSignatureReplay(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	previousPackages, err := discoverPublishedSetPackages(repositoryRoot, previousSet)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(previousPackages))
	for _, value := range previousPackages {
		paths = append(paths, value.locator.SourcePath)
	}
	request := filepath.Join(t.TempDir(), "request")
	report, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range report.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(request, filepath.FromSlash(bundle)), []byte("synthetic-cycle-bundle"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fixture := t.TempDir()
	for _, source := range []string{"forms/releases", "forms/trust/publisher-policy.json", "forms/trust/trusted-root.json", "forms/revocations/README.md"} {
		copyTestTree(t, filepath.Join(repositoryRoot, filepath.FromSlash(source)), filepath.Join(fixture, filepath.FromSlash(source)))
	}
	const firstID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secondID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, pair := range [][2]string{{firstID, secondID}, {secondID, firstID}} {
		setRoot := filepath.Join(fixture, filepath.FromSlash(TrustSetsRelativePath), pair[0])
		copyTestTree(t, request, setRoot)
		manifestRaw, err := os.ReadFile(filepath.Join(setRoot, LineageSubjectPath))
		if err != nil {
			t.Fatal(err)
		}
		var manifest SetLineageSubject
		if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.PreviousSetID = pair[1]
		encoded, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := formpackage.Canonicalize(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(setRoot, LineageSubjectPath), canonical, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	firstSet := filepath.Join(fixture, filepath.FromSlash(TrustSetsRelativePath), firstID)
	if _, err := VerifyPublishedSet(fixture, firstSet); err == nil || !strings.Contains(err.Error(), "continuation signature verification") {
		t.Fatalf("public verifier traversed unsigned cyclic ancestry: %v", err)
	}
	var verifyCycle func(string, string, map[string]struct{}) (verifiedEvidence, error)
	verifyCycle = func(repositoryRoot, setRoot string, ancestors map[string]struct{}) (verifiedEvidence, error) {
		setID := filepath.Base(setRoot)
		packages, err := discoverPublishedSetPackages(repositoryRoot, setRoot)
		if err != nil {
			return verifiedEvidence{}, err
		}
		synthetic := func(subject, bundle, root []byte, policy trust.PublisherPolicy) (trust.BundleVerification, error) {
			if string(bundle) != "synthetic-cycle-bundle" {
				return trust.BundleVerification{}, fmt.Errorf("unexpected synthetic cycle bundle")
			}
			return trust.BundleVerification{Status: trust.VerifiedStatus, SubjectDigest: formpackage.DigestBytes(subject), BundleDigest: formpackage.DigestBytes(bundle), TrustedRootDigest: formpackage.DigestBytes(root), OIDCIssuer: policy.OIDCIssuer, SourceRepository: policy.SourceRepository, Workflow: policy.Workflow, Ref: policy.Ref, PublisherIdentity: policy.Identity(), SourceCommit: setID, WorkflowCommit: setID, BuildConfigCommit: setID, TransparencyLogVerified: true, TransparencyLogThreshold: 1}, nil
		}
		return verifyEvidenceWithPredecessorVerifierWithAncestors(repositoryRoot, setRoot, setID, false, packages, synthetic, ancestors, verifyCycle)
	}
	if _, err := verifyCycle(fixture, firstSet, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "lineage contains a cycle") {
		t.Fatalf("synthetically signed cyclic publisher-set lineage error = %v", err)
	}
}

func TestPublishedContinuationBoundsAcyclicPredecessorReplay(t *testing.T) {
	t.Parallel()
	ancestors := make(map[string]struct{}, maxPublisherSetLineageDepth)
	for index := 0; index < maxPublisherSetLineageDepth; index++ {
		ancestors[fmt.Sprintf("%040x", index+1)] = struct{}{}
	}
	if _, err := verifyPublishedSetEvidenceWithAncestors(".", "unread-set", ancestors); err == nil || !strings.Contains(err.Error(), "lineage exceeds replay depth") {
		t.Fatalf("deep published-set replay error = %v, want early depth refusal", err)
	}
	if _, err := verifyEvidenceWithPredecessorVerifierWithAncestors(".", "unread-request", strings.Repeat("a", 40), true, nil, nil, ancestors, nil); err == nil || !strings.Contains(err.Error(), "lineage exceeds replay depth") {
		t.Fatalf("deep signing-request replay error = %v, want early depth refusal", err)
	}
}

func TestContinuationRejectsUnauthenticatedLineageBeforePredecessorTraversal(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), originalPublishedSetID)
	packages, err := discoverPackages(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(packages))
	for _, value := range packages {
		paths = append(paths, value.locator.SourcePath)
	}
	for name, badSubject := range map[string]bool{"malformed signed structure": true, "invalid signature": false} {
		t.Run(name, func(t *testing.T) {
			request := filepath.Join(t.TempDir(), "request")
			if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, request); err != nil {
				t.Fatal(err)
			}
			if badSubject {
				if err := os.WriteFile(filepath.Join(request, LineageSubjectPath), []byte(`{"previousSetId":"untrusted"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(request, LineageBundlePath), []byte(`{}`), 0o644); err != nil {
				t.Fatal(err)
			}
			predecessorVisited := false
			_, err := verifyEvidenceWithPredecessorVerifier(repositoryRoot, request, strings.Repeat("a", 40), true, packages,
				func([]byte, []byte, []byte, trust.PublisherPolicy) (trust.BundleVerification, error) {
					return trust.BundleVerification{}, fmt.Errorf("invalid test-only signature")
				},
				func(string, string, map[string]struct{}) (verifiedEvidence, error) {
					predecessorVisited = true
					return verifiedEvidence{}, fmt.Errorf("unexpected predecessor traversal")
				})
			if err == nil || predecessorVisited {
				t.Fatalf("unauthenticated lineage error = %v, predecessor visited = %t", err, predecessorVisited)
			}
			if badSubject && !strings.Contains(err.Error(), "invalid continuation predecessor identity") {
				t.Fatalf("malformed lineage error = %v", err)
			}
			if !badSubject && !strings.Contains(err.Error(), "continuation signature verification") {
				t.Fatalf("invalid lineage signature error = %v", err)
			}
		})
	}
}

func TestSyntheticContinuationVerifiesInheritedCoreEvidenceAndNewProvenance(t *testing.T) {
	repositoryRoot, paths, newPackages := syntheticActiveRoster(t, 2, "")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	old, err := discoverPublishedSetPackages(repositoryRoot, previousSet)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "continuation")
	report, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, paths, output)
	if err != nil {
		t.Fatal(err)
	}
	const newCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, subject := range report.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(output, filepath.FromSlash(bundle)), []byte("synthetic-new-bundle"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packages := append([]verifiedCandidate(nil), old...)
	packages = append(packages, newPackages...)
	verifySynthetic := func(subject, bundle, root []byte, policy trust.PublisherPolicy) (trust.BundleVerification, error) {
		if string(bundle) != "synthetic-new-bundle" {
			return trust.BundleVerification{}, fmt.Errorf("unexpected synthetic bundle")
		}
		return trust.BundleVerification{Status: trust.VerifiedStatus, SubjectDigest: formpackage.DigestBytes(subject), BundleDigest: formpackage.DigestBytes(bundle), TrustedRootDigest: formpackage.DigestBytes(root), OIDCIssuer: policy.OIDCIssuer, SourceRepository: policy.SourceRepository, Workflow: policy.Workflow, Ref: policy.Ref, PublisherIdentity: policy.Identity(), SourceCommit: newCommit, WorkflowCommit: newCommit, BuildConfigCommit: newCommit, TransparencyLogVerified: true, TransparencyLogThreshold: 1}, nil
	}
	verified, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic)
	if err != nil {
		t.Fatalf("verify synthetic continuation: %v", err)
	}
	if verified.report.SetID != newCommit || verified.report.PreviousSetID != "e7f8a39311dd011b8467e97e7f300cabb9a6b06c" || verified.report.Checkpoint.Pin.Sequence != 0 || len(verified.report.Packages) != 19 || len(verified.report.RevocationTags) != 0 || len(verified.report.SetHistory) != 2 || verified.report.SetHistory[0].SetID != "e7f8a39311dd011b8467e97e7f300cabb9a6b06c" || verified.report.SetHistory[1].SetID != newCommit {
		t.Fatalf("unexpected verified continuation: %+v", verified.report)
	}
	futureRepo := t.TempDir()
	copyTestTree(t, filepath.Join(repositoryRoot, "forms"), filepath.Join(futureRepo, "forms"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "trust", "publisher-policy.json"), filepath.Join(futureRepo, "forms", "trust", "publisher-policy.json"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "trust", "trusted-root.json"), filepath.Join(futureRepo, "forms", "trust", "trusted-root.json"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "revocations", "README.md"), filepath.Join(futureRepo, "forms", "revocations", "README.md"))
	futureSet := filepath.Join(futureRepo, filepath.FromSlash(TrustSetsRelativePath), newCommit)
	copyTestTree(t, output, futureSet)
	repeatedRequest := filepath.Join(t.TempDir(), "repeated-no-revocation")
	repeated, err := prepareContinuationFromVerifiedPredecessor(futureRepo, futureSet, paths, repeatedRequest, verified)
	if err != nil {
		t.Fatalf("prepare repeated no-revocation successor: %v", err)
	}
	if repeated.Mode != ContinuationMode || repeated.PreviousSetID != newCommit || repeated.Sequence != 0 || repeated.PackageCount != 19 || len(repeated.Subjects) != 20 || repeated.RevocationTag != "" {
		t.Fatalf("repeated continuation changed checkpoint or roster: %+v", repeated)
	}
	for _, relative := range []string{RevocationCheckpointPath, RevocationBundlePath} {
		want, err := os.ReadFile(filepath.Join(futureSet, relative))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(repeatedRequest, relative))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("repeated continuation changed inherited %s", relative)
		}
	}
	for _, subject := range repeated.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(repeatedRequest, filepath.FromSlash(bundle)), []byte("synthetic-new-bundle"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const repeatedCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	verifyRepeated := func(subject, bundle, root []byte, policy trust.PublisherPolicy) (trust.BundleVerification, error) {
		result, err := verifySynthetic(subject, bundle, root, policy)
		if err != nil {
			return result, err
		}
		result.SourceCommit, result.WorkflowCommit, result.BuildConfigCommit = repeatedCommit, repeatedCommit, repeatedCommit
		return result, nil
	}
	verifiedRepeated, err := verifyEvidenceWithPredecessorVerifier(futureRepo, repeatedRequest, repeatedCommit, true, packages, verifyRepeated, func(_ string, setRoot string, _ map[string]struct{}) (verifiedEvidence, error) {
		if filepath.Base(setRoot) != newCommit {
			return verifiedEvidence{}, fmt.Errorf("wrong synthetic predecessor %s", setRoot)
		}
		return verified, nil
	})
	if err != nil || verifiedRepeated.report.Mode != ContinuationMode || verifiedRepeated.report.PreviousSetID != newCommit || verifiedRepeated.report.Checkpoint.Pin != verified.report.Checkpoint.Pin || len(verifiedRepeated.report.SetHistory) != 3 {
		t.Fatalf("verify repeated C0 continuation with test-only signatures: report=%+v error=%v", verifiedRepeated.report, err)
	}
	abandoned, err := ReadAbandonedPrepublication(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	revokedSource := abandoned.EvidenceOnlyPackages[0].SourcePath
	revokedPackage, err := verifyReleaseCandidate(filepath.Join(repositoryRoot, filepath.FromSlash(revokedSource)), revokedSource)
	if err != nil {
		t.Fatal(err)
	}
	statement := canonicalStatement(t, revokedPackage, 1, "1.0.0")
	checkpointRaw := checkpointForStatement(t, verified.report.Checkpoint.Pin, nil, statement)
	for relative, raw := range map[string][]byte{"forms/revocations/1.0.0.json": statement, "forms/revocations/checkpoints/1.0.0.json": checkpointRaw} {
		path := filepath.Join(futureRepo, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	futureRequest := filepath.Join(t.TempDir(), "revocation-successor")
	future, err := prepareRevocationFromVerifiedPredecessor(futureRepo, futureSet, "1.0.0", futureRequest, verified)
	if err != nil {
		t.Fatalf("prepare real checkpoint extension after continuation: %v", err)
	}
	if future.Mode != AdvancementMode || future.PreviousSetID != newCommit || future.Sequence != 1 || len(future.Subjects) != 21 || future.RevocationTag != "forms/revocations/v1.0.0" {
		t.Fatalf("unexpected post-continuation revocation request: %+v", future)
	}
	futureLineageRaw, err := os.ReadFile(filepath.Join(futureRequest, LineageSubjectPath))
	if err != nil {
		t.Fatal(err)
	}
	var futureLineage SetLineageSubject
	if err := json.Unmarshal(futureLineageRaw, &futureLineage); err != nil {
		t.Fatal(err)
	}
	if futureLineage.PreviousSetID != newCommit || futureLineage.PreviousCheckpointPin.Digest != verified.report.Checkpoint.Pin.Digest || futureLineage.CurrentCheckpointPin.Sequence != 1 || futureLineage.RevocationTag != "forms/revocations/v1.0.0" || futureLineage.StatementDigest == "" {
		t.Fatalf("future lineage confuses immediate set predecessor with checkpoint signer: %+v", futureLineage)
	}
	if verified.report.Checkpoint.Bundle.SourceCommit == futureLineage.PreviousSetID {
		t.Fatal("fixture failed to separate set predecessor from checkpoint signer")
	}
	for _, subject := range future.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "revocation-checkpoint" {
			bundle = RevocationBundlePath
		}
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		path := filepath.Join(futureRequest, filepath.FromSlash(bundle))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifySigningRequest(futureRepo, futureRequest, "cccccccccccccccccccccccccccccccccccccccc"); err == nil || !strings.Contains(err.Error(), "checkpoint history sequence 1 verification") {
		t.Fatalf("fake real-revocation successor error = %v, want actual Core checkpoint signature refusal", err)
	}
	// S2 has a new checkpoint but its immediate set predecessor is S1, not
	// the S0 signer of C0. A later real revocation must keep the same 19
	// package identities and extend C1 rather than falling back to current17.
	const secondCommit = "cccccccccccccccccccccccccccccccccccccccc"
	secondSet := filepath.Join(futureRepo, filepath.FromSlash(TrustSetsRelativePath), secondCommit)
	copyTestTree(t, futureRequest, secondSet)
	firstAdvancement, err := validateRevocationAdvancement(verified.report.Checkpoint.Pin, statement, checkpointRaw)
	if err != nil {
		t.Fatal(err)
	}
	secondPackages := append([]PackageVerification(nil), verified.report.Packages...)
	second := verifiedEvidence{
		report: VerificationReport{SetID: secondCommit, Mode: AdvancementMode, Packages: secondPackages, Checkpoint: trust.RevocationCheckpointVerification{Pin: firstAdvancement.Pin}},
		revocations: verifiedRevocationChain{
			statements:  append(append([]verifiedRevocationStatement(nil), verified.revocations.statements...), verifiedRevocationStatement{raw: statement, verification: RevocationStatementVerification{Sequence: 1, StatementVersion: "1.0.0", StatementDigest: firstAdvancement.Entry.StatementDigest, Tag: firstAdvancement.Tag}}),
			checkpoints: append(append([]verifiedRevocationCheckpoint(nil), verified.revocations.checkpoints...), verifiedRevocationCheckpoint{raw: checkpointRaw, bundle: []byte(`{}`), verification: trust.RevocationCheckpointVerification{Pin: firstAdvancement.Pin, CheckpointVersion: "1.0.0"}}),
		},
	}
	afterRevocationRequest := filepath.Join(t.TempDir(), "no-revocation-after-c1")
	// The source-only C1 fixture has no real Sigstore proof. This private seam
	// checks the exact validated statement entries; the public path still calls
	// Core CheckNotRevoked and rejects this fake predecessor signature.
	checkpointValue, err := formpackage.ValidateRevocationCheckpoint(checkpointRaw)
	if err != nil {
		t.Fatal(err)
	}
	afterRevocation, err := prepareContinuationWithRevocationCheck(futureRepo, secondSet, paths, afterRevocationRequest, second, func(_ trust.RevocationCheckpointVerification, digest string, formRef formpackage.FormRef) error {
		for _, entry := range checkpointValue.Entries {
			if entry.PackageDigest == digest && entry.FormRef == formRef {
				return fmt.Errorf("synthetic active package was revoked")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("prepare no-revocation successor after real C1: %v", err)
	}
	if afterRevocation.Mode != ContinuationMode || afterRevocation.PreviousSetID != secondCommit || afterRevocation.Sequence != 1 || afterRevocation.PackageCount != 19 || afterRevocation.RevocationTag != "" || len(afterRevocation.Subjects) != 20 {
		t.Fatalf("post-C1 continuation changed checkpoint or roster: %+v", afterRevocation)
	}
	for _, relative := range []string{RevocationCheckpointPath, RevocationBundlePath, revocationHistoryCheckpointPath("0.0.0"), revocationHistoryBundlePath("0.0.0"), revocationStatementEvidencePath("1.0.0")} {
		want, err := os.ReadFile(filepath.Join(secondSet, relative))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(afterRevocationRequest, relative))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("post-C1 continuation changed inherited %s", relative)
		}
	}
	secondRevokedSource := abandoned.EvidenceOnlyPackages[1].SourcePath
	secondRevokedPackage, err := verifyReleaseCandidate(filepath.Join(repositoryRoot, filepath.FromSlash(secondRevokedSource)), secondRevokedSource)
	if err != nil {
		t.Fatal(err)
	}
	secondStatement := canonicalStatement(t, secondRevokedPackage, 2, "1.1.0")
	secondCheckpointRaw := checkpointForStatement(t, firstAdvancement.Pin, []formpackage.RevocationCheckpointEntry{firstAdvancement.Entry}, secondStatement)
	for relative, raw := range map[string][]byte{"forms/revocations/1.1.0.json": secondStatement, "forms/revocations/checkpoints/1.1.0.json": secondCheckpointRaw} {
		path := filepath.Join(futureRepo, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wrongSet := second
	wrongSet.report.SetID = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	if _, err := prepareRevocationFromVerifiedPredecessor(futureRepo, secondSet, "1.1.0", filepath.Join(t.TempDir(), "wrong-set"), wrongSet); err == nil || !strings.Contains(err.Error(), "set ID differs") {
		t.Fatalf("wrong immediate set predecessor error = %v", err)
	}
	wrongRoster := second
	wrongRoster.report.Packages = append([]PackageVerification(nil), second.report.Packages...)
	wrongRoster.report.Packages[0].PackageDigest = "sha256:" + strings.Repeat("f", 64)
	if _, err := prepareRevocationFromVerifiedPredecessor(futureRepo, secondSet, "1.1.0", filepath.Join(t.TempDir(), "wrong-roster"), wrongRoster); err == nil || !strings.Contains(err.Error(), "package inventory differs") {
		t.Fatalf("changed predecessor roster error = %v", err)
	}
	secondCheckpointPath := filepath.Join(futureRepo, "forms", "revocations", "checkpoints", "1.1.0.json")
	forkedSecondCheckpoint := bytes.Replace(secondCheckpointRaw, []byte(firstAdvancement.Pin.Digest), []byte("sha256:"+strings.Repeat("f", 64)), 1)
	if err := os.WriteFile(secondCheckpointPath, forkedSecondCheckpoint, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareRevocationFromVerifiedPredecessor(futureRepo, secondSet, "1.1.0", filepath.Join(t.TempDir(), "forked-checkpoint"), second); err == nil || !strings.Contains(err.Error(), "pinned digest") {
		t.Fatalf("wrong predecessor checkpoint error = %v", err)
	}
	if err := os.WriteFile(secondCheckpointPath, secondCheckpointRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	thirdRequest := filepath.Join(t.TempDir(), "second-revocation-successor")
	third, err := prepareRevocationFromVerifiedPredecessor(futureRepo, secondSet, "1.1.0", thirdRequest, second)
	if err != nil {
		t.Fatalf("prepare second real checkpoint extension after continuation: %v", err)
	}
	if third.Mode != AdvancementMode || third.PreviousSetID != secondCommit || third.Sequence != 2 || third.PackageCount != 19 || len(third.Subjects) != 21 {
		t.Fatalf("second advancement changed immediate predecessor or package closure: %+v", third)
	}
	thirdLineageRaw, err := os.ReadFile(filepath.Join(thirdRequest, LineageSubjectPath))
	if err != nil {
		t.Fatal(err)
	}
	var thirdLineage SetLineageSubject
	if err := json.Unmarshal(thirdLineageRaw, &thirdLineage); err != nil {
		t.Fatal(err)
	}
	if thirdLineage.PreviousSetID != secondCommit || thirdLineage.PreviousCheckpointPin != firstAdvancement.Pin || len(thirdLineage.Subjects) != 19 {
		t.Fatalf("second advancement lineage is not the exact S2/C1 successor: %+v", thirdLineage)
	}
	for _, subject := range third.Subjects {
		bundle := LineageBundlePath
		if subject.Role == "revocation-checkpoint" {
			bundle = RevocationBundlePath
		}
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(thirdRequest, filepath.FromSlash(bundle)), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifySigningRequest(futureRepo, thirdRequest, "dddddddddddddddddddddddddddddddddddddddd"); err == nil || !strings.Contains(err.Error(), "checkpoint history sequence 1 verification") {
		t.Fatalf("fake second real-revocation successor error = %v, want actual Core checkpoint signature refusal", err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, newCommit); err == nil || !strings.Contains(err.Error(), "continuation signature verification") {
		t.Fatalf("public verifier error = %v, want actual Core signature refusal", err)
	}
	mixedCommit := func(subject, bundle, root []byte, policy trust.PublisherPolicy) (trust.BundleVerification, error) {
		result, err := verifySynthetic(subject, bundle, root, policy)
		if err != nil {
			return result, err
		}
		if formpackage.DigestBytes(subject) == newPackages[0].candidate.PackageDigest {
			result.SourceCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			result.WorkflowCommit = result.SourceCommit
			result.BuildConfigCommit = result.SourceCommit
		}
		return result, nil
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, mixedCommit); err == nil || !strings.Contains(err.Error(), "new package provenance") {
		t.Fatalf("mixed package commit error = %v, want provenance refusal", err)
	}

	oldCheckpoint, err := os.ReadFile(filepath.Join(output, RevocationCheckpointPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, RevocationCheckpointPath), []byte(canonicalGenesis+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil {
		t.Fatal("changed checkpoint raw was accepted")
	}
	if err := os.WriteFile(filepath.Join(output, RevocationCheckpointPath), oldCheckpoint, 0o644); err != nil {
		t.Fatal(err)
	}
	checkpointBundlePath := filepath.Join(output, RevocationBundlePath)
	oldCheckpointBundle, err := os.ReadFile(checkpointBundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checkpointBundlePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil {
		t.Fatal("changed inherited checkpoint bundle was accepted")
	}
	if err := os.WriteFile(checkpointBundlePath, oldCheckpointBundle, 0o644); err != nil {
		t.Fatal(err)
	}
	oldBundlePath := filepath.Join(output, filepath.FromSlash(packageBundlePath(old[0].locator)))
	oldBundleRaw, err := os.ReadFile(oldBundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldBundlePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil {
		t.Fatal("changed package bundle was accepted")
	}
	if err := os.WriteFile(oldBundlePath, oldBundleRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{PublisherPolicyPath, TrustedRootPath} {
		path := filepath.Join(output, relative)
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil {
			t.Fatalf("changed %s was accepted", relative)
		}
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	manifestRaw, err := os.ReadFile(filepath.Join(output, LineageSubjectPath))
	if err != nil {
		t.Fatal(err)
	}
	var manifest SetLineageSubject
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Subjects[0].Digest = "sha256:" + strings.Repeat("f", 64)
	altered, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	altered, err = formpackage.Canonicalize(altered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, LineageSubjectPath), altered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil || !strings.Contains(err.Error(), "signed package inventory") {
		t.Fatalf("false manifest inventory error = %v", err)
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.CurrentCheckpointPin.Digest = "sha256:" + strings.Repeat("f", 64)
	altered, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	altered, err = formpackage.Canonicalize(altered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, LineageSubjectPath), altered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil || !strings.Contains(err.Error(), "checkpoint pin") {
		t.Fatalf("changed checkpoint pin error = %v", err)
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.PreviousSetID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	altered, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	altered, err = formpackage.Canonicalize(altered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, LineageSubjectPath), altered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyEvidenceWithNewBundleVerifier(repositoryRoot, output, newCommit, true, packages, verifySynthetic); err == nil {
		t.Fatal("stale or fabricated predecessor was accepted")
	}
	if err := os.Remove(filepath.Join(output, LineageSubjectPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, newCommit); err == nil {
		t.Fatal("19-package evidence without a signed lineage subject was accepted")
	}
}

func copyTestTree(t *testing.T, source, target string) {
	t.Helper()
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			copyTestTree(t, filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name()))
		}
		return
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// These fixtures model a future publisher-owned selection without mutating the
// checked-in 17-package index, candidate set, release roots, or signed history.
func syntheticActiveRoster(t *testing.T, extraKinds int, replaceKind string) (string, []string, []verifiedCandidate) {
	t.Helper()
	source := filepath.Join("..", "..")
	fixture := t.TempDir()
	copyTestTree(t, filepath.Join(source, "forms"), filepath.Join(fixture, "forms"))
	current, err := discoverPackages(fixture)
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]verifiedCandidate, 0, len(current)+extraKinds)
	var base verifiedCandidate
	for _, value := range current {
		if value.candidate.Kind == "ModuleWorker" {
			base = value
		}
		if value.candidate.Kind != replaceKind {
			selected = append(selected, value)
		}
	}
	if base.locator.Tag == "" {
		t.Fatal("fixture lacks ModuleWorker base package")
	}
	var added []verifiedCandidate
	if replaceKind != "" {
		added = append(added, synthesizeRelease(t, fixture, base, replaceKind, "0.2.0"))
	}
	for i := 0; i < extraKinds; i++ {
		added = append(added, synthesizeRelease(t, fixture, base, fmt.Sprintf("SyntheticContainer%d", i+1), "0.1.0"))
	}
	selected = append(selected, added...)
	projectSelectedCandidates(t, fixture, selected)
	paths := make([]string, 0, len(selected))
	for _, value := range selected {
		paths = append(paths, value.locator.SourcePath)
	}
	return fixture, paths, added
}

func synthesizeRelease(t *testing.T, repositoryRoot string, base verifiedCandidate, kind, definitionVersion string) verifiedCandidate {
	t.Helper()
	staging := filepath.Join(t.TempDir(), "package")
	copyTestTree(t, base.releaseRoot, staging)
	definitionPath := filepath.Join(staging, "definition.json")
	definitionRaw, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	var definition map[string]any
	if err := json.Unmarshal(definitionRaw, &definition); err != nil {
		t.Fatal(err)
	}
	definition["kind"] = kind
	definition["definitionVersion"] = definitionVersion
	encoded, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	definitionRaw, err = formpackage.Canonicalize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionPath, definitionRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(staging, PackageIndexName)
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index formpackage.PackageIndex
	if err := json.Unmarshal(indexRaw, &index); err != nil {
		t.Fatal(err)
	}
	index.FormRef.Kind = kind
	index.FormRef.DefinitionVersion = definitionVersion
	index.FormRef.SchemaDigest = formpackage.DigestBytes(definitionRaw)
	for i := range index.Files {
		if index.Files[i].Path == index.DefinitionPath {
			index.Files[i].Size = int64(len(definitionRaw))
			index.Files[i].Digest = formpackage.DigestBytes(definitionRaw)
		}
	}
	encoded, err = json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	indexRaw, err = formpackage.Canonicalize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, indexRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	locator, err := formpackage.PublicationLocatorFor(index, formpackage.DigestBytes(indexRaw))
	if err != nil {
		t.Fatal(err)
	}
	copyTestTree(t, staging, filepath.Join(repositoryRoot, filepath.FromSlash(locator.SourcePath)))
	verified, err := verifyReleaseCandidate(filepath.Join(repositoryRoot, filepath.FromSlash(locator.SourcePath)), locator.SourcePath)
	if err != nil {
		t.Fatalf("synthetic %s release is not Core-valid: %v", kind, err)
	}
	return verified
}

func projectSelectedCandidates(t *testing.T, repositoryRoot string, selected []verifiedCandidate) {
	t.Helper()
	setPath := filepath.Join(repositoryRoot, filepath.FromSlash(candidateSetSource))
	raw, err := os.ReadFile(setPath)
	if err != nil {
		t.Fatal(err)
	}
	var set candidateSet
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	set.Forms = nil
	for i, value := range selected {
		candidatePath := fmt.Sprintf("forms/candidates/%s/synthetic-selected-%02d", Family, i)
		copyTestTree(t, value.releaseRoot, filepath.Join(repositoryRoot, filepath.FromSlash(candidatePath)))
		set.Forms = append(set.Forms, packageCandidate{Kind: value.candidate.Kind, Role: value.candidate.Role, Path: candidatePath, FormRef: value.candidate.FormRef, PackageDigest: value.candidate.PackageDigest})
	}
	raw, err = json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(setPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(repositoryRoot, filepath.FromSlash(currentFamilyIndexSource))
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index currentFamilyIndex
	if err := json.Unmarshal(indexRaw, &index); err != nil {
		t.Fatal(err)
	}
	index.Families[0].FormCount = len(selected)
	index.Families[0].SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	indexRaw, err = json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, indexRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySigningRequestRequiresCryptographicBundlesNotSerializedClaims(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	output := filepath.Join(t.TempDir(), "request")
	if _, err := PrepareSigningRequest(repositoryRoot, output); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(output, "verification.json"), []byte(`{"status":"verified"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, ""); err == nil || !strings.Contains(err.Error(), "unexpected evidence file verification.json") {
		t.Fatalf("serialized claim error = %v, want exact-closure refusal", err)
	}
	if err := os.Remove(filepath.Join(output, "verification.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, ""); err == nil || !strings.Contains(err.Error(), "signature bundle is missing") {
		t.Fatalf("unsigned request error = %v, want missing-bundle refusal", err)
	}
}

func TestVerifySigningRequestRejectsEvidenceControlledTrustedRoot(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	output := filepath.Join(t.TempDir(), "request")
	report, err := PrepareSigningRequest(repositoryRoot, output)
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range report.Subjects {
		bundle := RevocationBundlePath
		if subject.Role == "package-index" {
			bundle = strings.TrimSuffix(subject.Path, PackageIndexName) + PackageBundleName
		}
		if err := os.WriteFile(filepath.Join(output, filepath.FromSlash(bundle)), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(output, TrustedRootPath), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySigningRequest(repositoryRoot, output, ""); err == nil || !strings.Contains(err.Error(), "differs from the repository-pinned trusted root") {
		t.Fatalf("untrusted root error = %v, want repository pin refusal", err)
	}
}

func TestPublishedSetPackageMembershipDoesNotFollowCurrentCandidates(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	current, err := discoverPackages(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	historicalRepository := t.TempDir()
	if err := os.CopyFS(
		filepath.Join(historicalRepository, "forms", "releases"),
		os.DirFS(filepath.Join(repositoryRoot, "forms", "releases")),
	); err != nil {
		t.Fatal(err)
	}
	setRoot := filepath.Join(t.TempDir(), "set")
	for _, packageValue := range current {
		bundle := filepath.Join(setRoot, filepath.FromSlash(packageBundlePath(packageValue.locator)))
		if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bundle, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(historicalRepository, filepath.FromSlash(candidateSetSource))); !os.IsNotExist(err) {
		t.Fatalf("historical test unexpectedly has a current candidate set: %v", err)
	}
	historical, err := discoverPublishedSetPackages(historicalRepository, setRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(historical) != len(current) {
		t.Fatalf("historical package count = %d, want %d", len(historical), len(current))
	}
	for index := range current {
		if historical[index].locator != current[index].locator || historical[index].candidate.PackageDigest != current[index].candidate.PackageDigest {
			t.Fatalf("historical package %d differs: got %+v, want %+v", index, historical[index], current[index])
		}
	}
}

func TestRevocationAdvancementExtendsTheExactCorePinAndRefusesRollbackForkAndPrefixRewrite(t *testing.T) {
	t.Parallel()
	genesis, err := canonicalGenesisBytes()
	if err != nil {
		t.Fatal(err)
	}
	genesisPin, err := formpackage.AdvanceRevocationCheckpoint(nil, genesis)
	if err != nil {
		t.Fatal(err)
	}
	packages, err := discoverPackages(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	firstStatement := canonicalStatement(t, packages[0], 1, "1.0.0")
	firstCheckpoint := checkpointForStatement(t, genesisPin, nil, firstStatement)
	first, err := validateRevocationAdvancement(genesisPin, firstStatement, firstCheckpoint)
	if err != nil {
		t.Fatalf("validate first advancement: %v", err)
	}
	if first.Pin.Sequence != 1 || first.Statement.StatementVersion != "1.0.0" ||
		first.Tag != "forms/revocations/v1.0.0" {
		t.Fatalf("unexpected first advancement: %+v", first)
	}
	prettyCheckpoint := append([]byte("\n"), firstCheckpoint...)
	if _, err := validateRevocationAdvancement(genesisPin, firstStatement, prettyCheckpoint); err == nil || !strings.Contains(err.Error(), "checkpoint bytes must be RFC 8785 canonical JSON") {
		t.Fatalf("noncanonical checkpoint error = %v, want canonical-byte refusal", err)
	}

	if _, err := validateRevocationAdvancement(first.Pin, firstStatement, genesis); err == nil || !strings.Contains(err.Error(), "sequence") {
		t.Fatalf("rollback error = %v, want sequence refusal", err)
	}
	forked := append([]byte(nil), firstCheckpoint...)
	forked = bytes.Replace(forked, []byte(genesisPin.Digest), []byte("sha256:"+strings.Repeat("f", 64)), 1)
	if _, err := validateRevocationAdvancement(genesisPin, firstStatement, forked); err == nil || !strings.Contains(err.Error(), "pinned digest") {
		t.Fatalf("fork error = %v, want predecessor refusal", err)
	}

	secondStatement := canonicalStatement(t, packages[1], 2, "1.1.0")
	secondCheckpoint := checkpointForStatement(t, first.Pin, []formpackage.RevocationCheckpointEntry{first.Entry}, secondStatement)
	second, err := validateRevocationAdvancement(first.Pin, secondStatement, secondCheckpoint)
	if err != nil {
		t.Fatalf("validate second advancement: %v", err)
	}
	if second.Pin.Sequence != 2 || second.Tag != "forms/revocations/v1.1.0" {
		t.Fatalf("unexpected second advancement: %+v", second)
	}
	rewritten := append([]byte(nil), secondCheckpoint...)
	rewritten = bytes.Replace(rewritten, []byte(first.Entry.PackageDigest), []byte("sha256:"+strings.Repeat("e", 64)), 1)
	if _, err := validateRevocationAdvancement(first.Pin, secondStatement, rewritten); err == nil || !strings.Contains(err.Error(), "pinned cumulative entries") {
		t.Fatalf("prefix rewrite error = %v, want cumulative-prefix refusal", err)
	}
}

func canonicalStatement(t *testing.T, packageValue verifiedCandidate, sequence uint64, version string) []byte {
	t.Helper()
	raw, err := json.Marshal(formpackage.RevocationStatement{
		APIVersion:       formpackage.CurrentTrustAPIVersion,
		Kind:             formpackage.RevocationKind,
		Sequence:         sequence,
		StatementVersion: version,
		PackageDigest:    packageValue.candidate.PackageDigest,
		FormRef:          packageValue.candidate.FormRef,
		ReasonCode:       "signature-invalid",
		Summary:          "The retained signature cannot be validated.",
		IssuedAt:         time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Effects: formpackage.RevocationEffects{
			BlockNewCreateOrUpdate:         true,
			BlockActivation:                true,
			RetainBytesForObserveAndDelete: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := formpackage.Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func checkpointForStatement(
	t *testing.T,
	previous formpackage.RevocationCheckpointPin,
	prefix []formpackage.RevocationCheckpointEntry,
	statement []byte,
) []byte {
	t.Helper()
	entry, err := formpackage.RevocationCheckpointEntryForStatement(statement)
	if err != nil {
		t.Fatal(err)
	}
	entries := append(append([]formpackage.RevocationCheckpointEntry(nil), prefix...), entry)
	raw, err := json.Marshal(formpackage.RevocationCheckpoint{
		APIVersion:               formpackage.CurrentTrustAPIVersion,
		Kind:                     formpackage.RevocationCheckpointKind,
		CheckpointVersion:        entry.StatementVersion,
		Sequence:                 previous.Sequence + 1,
		PreviousCheckpointDigest: &previous.Digest,
		Entries:                  entries,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := formpackage.Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
