package publishertrust

import (
	"bytes"
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

func TestPrepareContinuationKeepsSignedCheckpointAndAddsOnlyTwoExplicitSubjects(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	retained, err := readRetainedPackageEntries(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{retained[0].SourcePath, retained[1].SourcePath}
	output := filepath.Join(t.TempDir(), "continuation")
	abandoned := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), AbandonedPrepublicationSetID)
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, abandoned, paths, filepath.Join(t.TempDir(), "abandoned")); err == nil || !strings.Contains(err.Error(), "evidence-only predecessor") {
		t.Fatalf("abandoned predecessor error = %v", err)
	}
	if _, err := PrepareContinuationSigningRequest(repositoryRoot, previousSet, []string{paths[0], paths[0]}, filepath.Join(t.TempDir(), "duplicate")); err == nil || !strings.Contains(err.Error(), "repeats package identity") {
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

func TestSyntheticContinuationVerifiesInheritedCoreEvidenceAndNewProvenance(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	previousSet := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), "e7f8a39311dd011b8467e97e7f300cabb9a6b06c")
	retained, err := readRetainedPackageEntries(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{retained[0].SourcePath, retained[1].SourcePath}
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
	old, err := discoverPublishedSetPackages(repositoryRoot, previousSet)
	if err != nil {
		t.Fatal(err)
	}
	packages := append([]verifiedCandidate(nil), old...)
	for _, path := range paths {
		value, err := verifyReleaseCandidate(filepath.Join(repositoryRoot, filepath.FromSlash(path)), path)
		if err != nil {
			t.Fatal(err)
		}
		packages = append(packages, value)
	}
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
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "releases"), filepath.Join(futureRepo, "forms", "releases"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "trust", "publisher-policy.json"), filepath.Join(futureRepo, "forms", "trust", "publisher-policy.json"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "trust", "trusted-root.json"), filepath.Join(futureRepo, "forms", "trust", "trusted-root.json"))
	copyTestTree(t, filepath.Join(repositoryRoot, "forms", "revocations", "README.md"), filepath.Join(futureRepo, "forms", "revocations", "README.md"))
	futureSet := filepath.Join(futureRepo, filepath.FromSlash(TrustSetsRelativePath), newCommit)
	copyTestTree(t, output, futureSet)
	statement := canonicalStatement(t, packages[0], 1, "1.0.0")
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
	secondStatement := canonicalStatement(t, packages[1], 2, "1.1.0")
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
		if formpackage.DigestBytes(subject) == retained[0].PackageDigest {
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
