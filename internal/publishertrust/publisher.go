// Package publishertrust prepares and verifies the official Edge Form
// publisher's public signing closure. Takoform Core owns every package,
// publisher-policy, Sigstore, and revocation check performed here; this package
// only fixes the publisher's file layout and create-only release identity.
package publishertrust

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/tako0614/takoform/formpackage"
	"github.com/tako0614/takoform/trust"
)

const (
	Family                    = "edge.forms.takoform.com"
	ExpectedPackageCount      = 16
	CoreVersion               = "v1.1.0"
	SigningRequiredStatus     = "signing-required"
	VerifiedPublicationStatus = "verified"
	PublisherRepository       = "https://github.com/tako0614/takoform-forms"
	PublisherWorkflow         = PublisherRepository + "/.github/workflows/form-package-signing.yml"
	PublisherRef              = "refs/heads/main"
	PublisherPolicyPath       = "publisher-policy.json"
	TrustedRootPath           = "trusted-root.json"
	RevocationCheckpointPath  = "revocations/checkpoint.json"
	RevocationBundlePath      = "revocations/checkpoint.sigstore.json"
	PackageIndexName          = "package-index.json"
	PackageBundleName         = "package-index.sigstore.json"
	TrustSetsRelativePath     = "forms/trust/sets"
	setTagPrefix              = "forms/sets/"
)

const (
	publisherPolicySource = "forms/trust/publisher-policy.json"
	trustedRootSource     = "forms/trust/trusted-root.json"
	candidateSetSource    = "forms/candidates/edge.forms.takoform.com/candidate-set.json"
)

type SigningSubject struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type PreparationReport struct {
	Status       string           `json:"status"`
	CoreVersion  string           `json:"coreVersion"`
	Family       string           `json:"family"`
	PackageCount int              `json:"packageCount"`
	Output       string           `json:"output"`
	Subjects     []SigningSubject `json:"subjects"`
}

type PackageVerification struct {
	Kind          string                         `json:"kind"`
	FormRef       formpackage.FormRef            `json:"formRef"`
	PackageDigest string                         `json:"packageDigest"`
	Locator       formpackage.PublicationLocator `json:"locator"`
	Bundle        trust.BundleVerification       `json:"bundle"`
}

type VerificationReport struct {
	Status            string                                 `json:"status"`
	CoreVersion       string                                 `json:"coreVersion"`
	Family            string                                 `json:"family"`
	SetID             string                                 `json:"setId"`
	SetTag            string                                 `json:"setTag"`
	PackageCount      int                                    `json:"packageCount"`
	PublisherIdentity string                                 `json:"publisherIdentity"`
	SourceCommit      string                                 `json:"sourceCommit"`
	WorkflowCommit    string                                 `json:"workflowCommit"`
	BuildConfigCommit string                                 `json:"buildConfigCommit"`
	Checkpoint        trust.RevocationCheckpointVerification `json:"checkpoint"`
	Packages          []PackageVerification                  `json:"packages"`
}

type packageCandidate struct {
	Kind          string              `json:"kind"`
	Role          string              `json:"role"`
	Path          string              `json:"path"`
	FormRef       formpackage.FormRef `json:"formRef"`
	PackageDigest string              `json:"packageDigest"`
}

type candidateSet struct {
	Format            string             `json:"format"`
	Family            string             `json:"family"`
	FormMaturity      string             `json:"formMaturity"`
	PackageAPIVersion string             `json:"packageApiVersion"`
	PublicationStatus string             `json:"publicationStatus"`
	AuthoringSource   string             `json:"authoringSource"`
	AuthoringPolicy   string             `json:"authoringPolicy"`
	Forms             []packageCandidate `json:"forms"`
}

type verifiedCandidate struct {
	candidate      packageCandidate
	locator        formpackage.PublicationLocator
	releaseRoot    string
	canonicalIndex []byte
}

// PrepareSigningRequest creates a closed, external signing request. It emits
// only public policy/root bytes, exact Core-canonical package-index subjects,
// and the exact Core v1 revocation genesis. It never creates a key or bundle.
func PrepareSigningRequest(repositoryRoot, output string) (PreparationReport, error) {
	if err := requireReleasedCore(); err != nil {
		return PreparationReport{}, err
	}
	repositoryRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return PreparationReport{}, fmt.Errorf("resolve repository root: %w", err)
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return PreparationReport{}, fmt.Errorf("resolve signing output: %w", err)
	}
	if _, err := os.Lstat(output); err == nil {
		return PreparationReport{}, fmt.Errorf("refusing to replace existing signing request %s", output)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return PreparationReport{}, fmt.Errorf("inspect signing output: %w", err)
	}

	packages, err := discoverPackages(repositoryRoot)
	if err != nil {
		return PreparationReport{}, err
	}
	policyRaw, policy, err := readPublisherPolicy(repositoryRoot)
	if err != nil {
		return PreparationReport{}, err
	}
	if err := validatePublisherPolicy(policy); err != nil {
		return PreparationReport{}, err
	}
	rootRaw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(trustedRootSource)), "publisher trusted root")
	if err != nil {
		return PreparationReport{}, err
	}
	if _, err := formpackage.Canonicalize(rootRaw); err != nil {
		return PreparationReport{}, fmt.Errorf("publisher trusted root is not I-JSON: %w", err)
	}
	genesis, err := canonicalGenesisBytes()
	if err != nil {
		return PreparationReport{}, err
	}

	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return PreparationReport{}, fmt.Errorf("create signing output parent: %w", err)
	}
	// Claim the public identity before writing any member. A crash can leave a
	// visibly incomplete request, but neither this process nor a concurrent one
	// can replace an existing request in place.
	if err := os.Mkdir(output, 0o755); err != nil {
		return PreparationReport{}, fmt.Errorf("create signing output create-only: %w", err)
	}

	subjects := make([]SigningSubject, 0, len(packages)+1)
	if err := writePublicFile(output, PublisherPolicyPath, policyRaw); err != nil {
		return PreparationReport{}, err
	}
	if err := writePublicFile(output, TrustedRootPath, rootRaw); err != nil {
		return PreparationReport{}, err
	}
	if err := writePublicFile(output, RevocationCheckpointPath, genesis); err != nil {
		return PreparationReport{}, err
	}
	subjects = append(subjects, SigningSubject{
		Role: "revocation-checkpoint", Path: RevocationCheckpointPath, Digest: formpackage.DigestBytes(genesis),
	})
	for _, packageValue := range packages {
		relative := packageSubjectPath(packageValue.locator)
		if err := writePublicFile(output, relative, packageValue.canonicalIndex); err != nil {
			return PreparationReport{}, err
		}
		subjects = append(subjects, SigningSubject{
			Role: "package-index", Path: relative, Digest: packageValue.candidate.PackageDigest,
		})
	}
	sort.Slice(subjects, func(left, right int) bool { return subjects[left].Path < subjects[right].Path })
	return PreparationReport{
		Status: SigningRequiredStatus, CoreVersion: CoreVersion, Family: Family,
		PackageCount: len(packages), Output: output, Subjects: subjects,
	}, nil
}

// VerifySigningRequest authenticates every exact subject and the signed
// genesis through released Core. A serialized report is never accepted as an
// input or capability.
func VerifySigningRequest(repositoryRoot, evidenceRoot, expectedSourceCommit string) (VerificationReport, error) {
	if err := requireReleasedCore(); err != nil {
		return VerificationReport{}, err
	}
	packages, err := discoverPackages(repositoryRoot)
	if err != nil {
		return VerificationReport{}, err
	}
	return verifyEvidence(repositoryRoot, evidenceRoot, expectedSourceCommit, true, packages)
}

// VerifyPublishedSet reruns the same cryptographic checks over one installed
// create-only set. Package subjects are re-derived from the release closures;
// the set contains no serialized verification shortcut.
func VerifyPublishedSet(repositoryRoot, setRoot string) (VerificationReport, error) {
	if err := requireReleasedCore(); err != nil {
		return VerificationReport{}, err
	}
	setRoot, err := filepath.Abs(setRoot)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("resolve trust set: %w", err)
	}
	setID := filepath.Base(setRoot)
	if !validCommit(setID) {
		return VerificationReport{}, fmt.Errorf("trust set directory %q is not an exact source commit", setID)
	}
	packages, err := discoverPublishedSetPackages(repositoryRoot, setRoot)
	if err != nil {
		return VerificationReport{}, err
	}
	return verifyEvidence(repositoryRoot, setRoot, setID, false, packages)
}

// InstallSigningRequest verifies a complete external request before copying
// only its public evidence into the source tree. The Core-authenticated source
// commit is the set identity, and an existing identity is never replaced.
func InstallSigningRequest(repositoryRoot, evidenceRoot, expectedSourceCommit string) (VerificationReport, string, error) {
	report, err := VerifySigningRequest(repositoryRoot, evidenceRoot, expectedSourceCommit)
	if err != nil {
		return VerificationReport{}, "", err
	}
	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return VerificationReport{}, "", fmt.Errorf("resolve repository root: %w", err)
	}
	target := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath), report.SetID)
	if _, err := os.Lstat(target); err == nil {
		return VerificationReport{}, "", fmt.Errorf("refusing to replace existing trust set %s", target)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return VerificationReport{}, "", fmt.Errorf("inspect trust set target: %w", err)
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return VerificationReport{}, "", fmt.Errorf("create trust set parent: %w", err)
	}
	// Create the final identity atomically before writing any member. If a
	// process dies mid-install the incomplete identity remains fail-closed and
	// cannot be retried or overwritten in place.
	if err := os.Mkdir(target, 0o755); err != nil {
		return VerificationReport{}, "", fmt.Errorf("create trust set identity create-only: %w", err)
	}

	for _, relative := range installedEvidencePaths(report) {
		raw, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(relative)), relative)
		if err != nil {
			return VerificationReport{}, "", err
		}
		if err := writePublicFile(target, relative, raw); err != nil {
			return VerificationReport{}, "", err
		}
	}
	installed, err := VerifyPublishedSet(repositoryRoot, target)
	if err != nil {
		return VerificationReport{}, "", fmt.Errorf("verify installed trust set: %w", err)
	}
	return installed, target, nil
}

// CheckPublishedSets verifies every installed set. An empty directory is a
// valid pre-signing source state, but it never makes deploy publication-ready.
func CheckPublishedSets(repositoryRoot string) ([]VerificationReport, error) {
	if err := requireReleasedCore(); err != nil {
		return nil, err
	}
	setsRoot := filepath.Join(repositoryRoot, filepath.FromSlash(TrustSetsRelativePath))
	entries, err := os.ReadDir(setsRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return []VerificationReport{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read trust sets: %w", err)
	}
	reports := make([]VerificationReport, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return nil, fmt.Errorf("trust set entry %s is not a regular directory", entry.Name())
		}
		report, err := VerifyPublishedSet(repositoryRoot, filepath.Join(setsRoot, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("verify trust set %s: %w", entry.Name(), err)
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(left, right int) bool { return reports[left].SetID < reports[right].SetID })
	return reports, nil
}

func requireReleasedCore() error {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("cannot prove the compiled Takoform Core dependency")
	}
	// Go test binaries intentionally omit dependency module metadata. The
	// command integration test executes a real built publisher-trust binary;
	// unit tests cover the validator below with explicit build records.
	if strings.HasSuffix(build.Path, ".test") {
		return nil
	}
	return validateCoreBuildInfo(build)
}

func validateCoreBuildInfo(build *debug.BuildInfo) error {
	for _, dependency := range build.Deps {
		if dependency.Path != "github.com/tako0614/takoform" {
			continue
		}
		if dependency.Version != CoreVersion || dependency.Replace != nil {
			return fmt.Errorf("publisher trust requires released Takoform Core %s without a module replacement", CoreVersion)
		}
		return nil
	}
	return fmt.Errorf("compiled publisher trust binary has no Takoform Core dependency")
}

func verifyEvidence(repositoryRoot, evidenceRoot, expectedSourceCommit string, includesSubjects bool, packages []verifiedCandidate) (VerificationReport, error) {
	repositoryRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("resolve repository root: %w", err)
	}
	evidenceRoot, err = filepath.Abs(evidenceRoot)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("resolve evidence root: %w", err)
	}
	if err := verifyEvidenceInventory(evidenceRoot, packages, includesSubjects); err != nil {
		return VerificationReport{}, err
	}

	pinnedPolicyRaw, policy, err := readPublisherPolicy(repositoryRoot)
	if err != nil {
		return VerificationReport{}, err
	}
	if err := validatePublisherPolicy(policy); err != nil {
		return VerificationReport{}, err
	}
	policyRaw, err := readRegular(filepath.Join(evidenceRoot, PublisherPolicyPath), "publisher policy")
	if err != nil {
		return VerificationReport{}, err
	}
	if !bytes.Equal(policyRaw, pinnedPolicyRaw) {
		return VerificationReport{}, fmt.Errorf("evidence publisher policy differs from the repository-pinned policy")
	}
	rootRaw, err := readRegular(filepath.Join(evidenceRoot, TrustedRootPath), "trusted root")
	if err != nil {
		return VerificationReport{}, err
	}
	pinnedRootRaw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(trustedRootSource)), "repository-pinned trusted root")
	if err != nil {
		return VerificationReport{}, err
	}
	if !bytes.Equal(rootRaw, pinnedRootRaw) {
		return VerificationReport{}, fmt.Errorf("evidence trusted root differs from the repository-pinned trusted root")
	}
	checkpointRaw, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(RevocationCheckpointPath)), "revocation checkpoint")
	if err != nil {
		return VerificationReport{}, err
	}
	genesis, err := canonicalGenesisBytes()
	if err != nil {
		return VerificationReport{}, err
	}
	if !bytes.Equal(checkpointRaw, genesis) {
		return VerificationReport{}, fmt.Errorf("first publisher trust set requires the exact signed Core v1 revocation genesis")
	}
	checkpointBundle, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(RevocationBundlePath)), "revocation checkpoint signature bundle")
	if err != nil {
		return VerificationReport{}, err
	}
	checkpoint, err := trust.VerifyRevocationCheckpoint(checkpointRaw, checkpointBundle, rootRaw, policy, nil)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("Core %s revocation checkpoint verification: %w", CoreVersion, err)
	}

	packageReports := make([]PackageVerification, 0, len(packages))
	for _, packageValue := range packages {
		subject := packageValue.canonicalIndex
		if includesSubjects {
			prepared, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(packageSubjectPath(packageValue.locator))), packageValue.candidate.Kind+" package-index subject")
			if err != nil {
				return VerificationReport{}, err
			}
			if !bytes.Equal(prepared, subject) {
				return VerificationReport{}, fmt.Errorf("%s package-index subject differs from the exact Core-canonical release bytes", packageValue.candidate.Kind)
			}
		}
		bundlePath := filepath.Join(evidenceRoot, filepath.FromSlash(packageBundlePath(packageValue.locator)))
		bundleRaw, err := readRegular(bundlePath, packageValue.candidate.Kind+" signature bundle")
		if err != nil {
			return VerificationReport{}, err
		}
		bundleReport, err := trust.VerifyBundle(subject, bundleRaw, rootRaw, policy)
		if err != nil {
			return VerificationReport{}, fmt.Errorf("Core %s %s package signature verification: %w", CoreVersion, packageValue.candidate.Kind, err)
		}
		if bundleReport.SubjectDigest != packageValue.candidate.PackageDigest {
			return VerificationReport{}, fmt.Errorf("%s signed subject digest %s differs from verified package digest %s", packageValue.candidate.Kind, bundleReport.SubjectDigest, packageValue.candidate.PackageDigest)
		}
		if err := checkpoint.CheckNotRevoked(packageValue.candidate.PackageDigest, packageValue.candidate.FormRef); err != nil {
			return VerificationReport{}, fmt.Errorf("Core %s %s revocation check: %w", CoreVersion, packageValue.candidate.Kind, err)
		}
		packageReports = append(packageReports, PackageVerification{
			Kind: packageValue.candidate.Kind, FormRef: packageValue.candidate.FormRef,
			PackageDigest: packageValue.candidate.PackageDigest, Locator: packageValue.locator, Bundle: bundleReport,
		})
	}

	provenance := checkpoint.Bundle
	for _, packageReport := range packageReports {
		if err := sameProvenance(provenance, packageReport.Bundle); err != nil {
			return VerificationReport{}, fmt.Errorf("%s signature provenance: %w", packageReport.Kind, err)
		}
	}
	commits := []struct {
		role  string
		value string
	}{
		{role: "source", value: provenance.SourceCommit},
		{role: "workflow", value: provenance.WorkflowCommit},
		{role: "build config", value: provenance.BuildConfigCommit},
	}
	for _, commit := range commits {
		if !validCommit(commit.value) {
			return VerificationReport{}, fmt.Errorf("Core verification returned an invalid %s commit %q", commit.role, commit.value)
		}
		if expectedSourceCommit != "" && commit.value != expectedSourceCommit {
			return VerificationReport{}, fmt.Errorf("signed %s commit %s differs from required protected-main commit %s", commit.role, commit.value, expectedSourceCommit)
		}
	}
	return VerificationReport{
		Status: VerifiedPublicationStatus, CoreVersion: CoreVersion, Family: Family,
		SetID: provenance.SourceCommit, SetTag: setTagPrefix + provenance.SourceCommit,
		PackageCount: len(packageReports), PublisherIdentity: provenance.PublisherIdentity,
		SourceCommit: provenance.SourceCommit, WorkflowCommit: provenance.WorkflowCommit,
		BuildConfigCommit: provenance.BuildConfigCommit, Checkpoint: checkpoint, Packages: packageReports,
	}, nil
}

// discoverPublishedSetPackages derives a historical set's exact package
// membership from its create-only bundle paths, then re-verifies the retained
// immutable release closures through Core. It deliberately does not consult
// the current candidate set, which may move forward after this set is
// published.
func discoverPublishedSetPackages(repositoryRoot, setRoot string) ([]verifiedCandidate, error) {
	files, err := inventoryRegularFiles(setRoot)
	if err != nil {
		return nil, err
	}
	seenTags := map[string]struct{}{}
	verified := make([]verifiedCandidate, 0, ExpectedPackageCount)
	for _, relative := range files {
		if !strings.HasPrefix(relative, "packages/") {
			continue
		}
		parts := strings.Split(relative, "/")
		if len(parts) != 4 || parts[3] != PackageBundleName || !safeRelative(parts[1]) || !safeRelative(parts[2]) {
			return nil, fmt.Errorf("published trust set has invalid package evidence path %s", relative)
		}
		releaseID, artifactID := parts[1], parts[2]
		sourcePath := filepath.ToSlash(filepath.Join("forms", "releases", releaseID, artifactID))
		releaseRoot := filepath.Join(repositoryRoot, filepath.FromSlash(sourcePath))
		report, err := formpackage.VerifyDirectory(releaseRoot)
		if err != nil {
			return nil, fmt.Errorf("Core %s historical release verification for %s: %w", CoreVersion, relative, err)
		}
		capability, ok := report.VerifiedPackage()
		if !ok {
			return nil, fmt.Errorf("Core %s did not issue a verified package capability for %s", CoreVersion, relative)
		}
		locator, err := formpackage.PublicationLocatorFor(capability.PackageIndex(), capability.PackageDigest())
		if err != nil {
			return nil, fmt.Errorf("Core %s historical publication locator for %s: %w", CoreVersion, relative, err)
		}
		if report.FormRef.APIVersion != Family || locator.ReleaseID != releaseID || locator.ArtifactID != artifactID || locator.SourcePath != sourcePath {
			return nil, fmt.Errorf("published trust set package path %s differs from its Core locator", relative)
		}
		if _, duplicate := seenTags[locator.Tag]; duplicate {
			return nil, fmt.Errorf("duplicate published package tag %s", locator.Tag)
		}
		seenTags[locator.Tag] = struct{}{}
		indexRaw, err := readRegular(filepath.Join(releaseRoot, PackageIndexName), report.FormRef.Kind+" historical release package index")
		if err != nil {
			return nil, err
		}
		canonical, err := formpackage.Canonicalize(indexRaw)
		if err != nil {
			return nil, fmt.Errorf("Core %s canonicalize %s historical package index: %w", CoreVersion, report.FormRef.Kind, err)
		}
		if formpackage.DigestBytes(canonical) != report.PackageDigest {
			return nil, fmt.Errorf("%s historical canonical package-index digest differs from package identity", report.FormRef.Kind)
		}
		verified = append(verified, verifiedCandidate{
			candidate: packageCandidate{
				Kind:          report.FormRef.Kind,
				FormRef:       report.FormRef,
				PackageDigest: report.PackageDigest,
			},
			locator: locator, releaseRoot: releaseRoot, canonicalIndex: canonical,
		})
	}
	if len(verified) != ExpectedPackageCount {
		return nil, fmt.Errorf("published trust set must contain exactly %d package bundles; found %d", ExpectedPackageCount, len(verified))
	}
	sort.Slice(verified, func(left, right int) bool { return verified[left].locator.Tag < verified[right].locator.Tag })
	return verified, nil
}

func discoverPackages(repositoryRoot string) ([]verifiedCandidate, error) {
	raw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(candidateSetSource)), "Edge candidate set")
	if err != nil {
		return nil, err
	}
	var candidates candidateSet
	if err := decodeStrict(raw, &candidates); err != nil {
		return nil, fmt.Errorf("decode Edge candidate set: %w", err)
	}
	if candidates.Format != "takoform.form-family-candidates@v1" || candidates.Family != Family || len(candidates.Forms) != ExpectedPackageCount {
		return nil, fmt.Errorf("Edge candidate set must contain exactly %d packages for %s", ExpectedPackageCount, Family)
	}
	seenTags := map[string]struct{}{}
	verified := make([]verifiedCandidate, 0, len(candidates.Forms))
	for _, candidate := range candidates.Forms {
		if candidate.Kind == "" || candidate.Path == "" || !safeRelative(candidate.Path) || !strings.HasPrefix(candidate.Path, "forms/candidates/"+Family+"/") {
			return nil, fmt.Errorf("%s candidate path is not an exact Edge package path", candidate.Kind)
		}
		candidateRoot := filepath.Join(repositoryRoot, filepath.FromSlash(candidate.Path))
		report, err := formpackage.VerifyDirectory(candidateRoot)
		if err != nil {
			return nil, fmt.Errorf("Core %s candidate verification for %s: %w", CoreVersion, candidate.Kind, err)
		}
		capability, ok := report.VerifiedPackage()
		if !ok {
			return nil, fmt.Errorf("Core %s did not issue a verified package capability for %s", CoreVersion, candidate.Kind)
		}
		if report.PackageDigest != candidate.PackageDigest || report.FormRef != candidate.FormRef {
			return nil, fmt.Errorf("%s candidate identity differs from the candidate set", candidate.Kind)
		}
		locator, err := formpackage.PublicationLocatorFor(capability.PackageIndex(), capability.PackageDigest())
		if err != nil {
			return nil, fmt.Errorf("Core %s publication locator for %s: %w", CoreVersion, candidate.Kind, err)
		}
		if _, duplicate := seenTags[locator.Tag]; duplicate {
			return nil, fmt.Errorf("duplicate package tag %s", locator.Tag)
		}
		seenTags[locator.Tag] = struct{}{}
		releaseRoot := filepath.Join(repositoryRoot, filepath.FromSlash(locator.SourcePath))
		releaseReport, err := formpackage.VerifyDirectory(releaseRoot)
		if err != nil {
			return nil, fmt.Errorf("Core %s release verification for %s: %w", CoreVersion, candidate.Kind, err)
		}
		if releaseReport.PackageDigest != candidate.PackageDigest || releaseReport.FormRef != candidate.FormRef {
			return nil, fmt.Errorf("%s release identity differs from its candidate", candidate.Kind)
		}
		indexRaw, err := readRegular(filepath.Join(releaseRoot, PackageIndexName), candidate.Kind+" release package index")
		if err != nil {
			return nil, err
		}
		canonical, err := formpackage.Canonicalize(indexRaw)
		if err != nil {
			return nil, fmt.Errorf("Core %s canonicalize %s package index: %w", CoreVersion, candidate.Kind, err)
		}
		if formpackage.DigestBytes(canonical) != candidate.PackageDigest {
			return nil, fmt.Errorf("%s canonical package-index digest differs from package identity", candidate.Kind)
		}
		verified = append(verified, verifiedCandidate{candidate: candidate, locator: locator, releaseRoot: releaseRoot, canonicalIndex: canonical})
	}
	sort.Slice(verified, func(left, right int) bool { return verified[left].locator.Tag < verified[right].locator.Tag })
	return verified, nil
}

func readPublisherPolicy(repositoryRoot string) ([]byte, trust.PublisherPolicy, error) {
	raw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(publisherPolicySource)), "publisher policy")
	if err != nil {
		return nil, trust.PublisherPolicy{}, err
	}
	policy, err := trust.ParsePublisherPolicy(raw)
	if err != nil {
		return nil, trust.PublisherPolicy{}, err
	}
	return raw, policy, nil
}

func validatePublisherPolicy(policy trust.PublisherPolicy) error {
	if policy.SourceRepository != PublisherRepository || policy.Workflow != PublisherWorkflow || policy.Ref != PublisherRef {
		return fmt.Errorf("publisher policy must pin %s at %s on %s", PublisherRepository, PublisherWorkflow, PublisherRef)
	}
	return nil
}

func canonicalGenesisBytes() ([]byte, error) {
	checkpoint := formpackage.RevocationCheckpoint{
		APIVersion: formpackage.CurrentTrustAPIVersion, Kind: formpackage.RevocationCheckpointKind,
		CheckpointVersion: "0.0.0", Sequence: 0, PreviousCheckpointDigest: nil,
		Entries: []formpackage.RevocationCheckpointEntry{},
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("marshal Core v1 revocation genesis: %w", err)
	}
	canonical, err := formpackage.Canonicalize(raw)
	if err != nil {
		return nil, fmt.Errorf("canonicalize Core v1 revocation genesis: %w", err)
	}
	if _, err := formpackage.AdvanceRevocationCheckpoint(nil, canonical); err != nil {
		return nil, fmt.Errorf("validate Core v1 revocation genesis: %w", err)
	}
	return canonical, nil
}

func verifyEvidenceInventory(root string, packages []verifiedCandidate, includesSubjects bool) error {
	allowed := map[string]struct{}{
		PublisherPolicyPath: {}, TrustedRootPath: {}, RevocationCheckpointPath: {}, RevocationBundlePath: {},
	}
	requiredBundles := make([]string, 0, len(packages)+1)
	requiredBundles = append(requiredBundles, RevocationBundlePath)
	for _, packageValue := range packages {
		if includesSubjects {
			allowed[packageSubjectPath(packageValue.locator)] = struct{}{}
		}
		bundle := packageBundlePath(packageValue.locator)
		allowed[bundle] = struct{}{}
		requiredBundles = append(requiredBundles, bundle)
	}
	actual, err := inventoryRegularFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range actual {
		if _, ok := allowed[relative]; !ok {
			return fmt.Errorf("unexpected evidence file %s", relative)
		}
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, relative := range actual {
		actualSet[relative] = struct{}{}
	}
	for _, required := range []string{PublisherPolicyPath, TrustedRootPath, RevocationCheckpointPath} {
		if _, ok := actualSet[required]; !ok {
			return fmt.Errorf("required evidence file %s is missing", required)
		}
	}
	if includesSubjects {
		for _, packageValue := range packages {
			required := packageSubjectPath(packageValue.locator)
			if _, ok := actualSet[required]; !ok {
				return fmt.Errorf("required package-index subject %s is missing", required)
			}
		}
	}
	sort.Strings(requiredBundles)
	for _, required := range requiredBundles {
		if _, ok := actualSet[required]; !ok {
			return fmt.Errorf("signature bundle is missing: %s", required)
		}
	}
	return nil
}

func installedEvidencePaths(report VerificationReport) []string {
	paths := []string{PublisherPolicyPath, TrustedRootPath, RevocationCheckpointPath, RevocationBundlePath}
	for _, packageReport := range report.Packages {
		paths = append(paths, packageBundlePath(packageReport.Locator))
	}
	sort.Strings(paths)
	return paths
}

func packageSubjectPath(locator formpackage.PublicationLocator) string {
	return filepath.ToSlash(filepath.Join("packages", locator.ReleaseID, locator.ArtifactID, PackageIndexName))
}

func packageBundlePath(locator formpackage.PublicationLocator) string {
	return filepath.ToSlash(filepath.Join("packages", locator.ReleaseID, locator.ArtifactID, PackageBundleName))
}

func sameProvenance(expected, actual trust.BundleVerification) error {
	if expected.OIDCIssuer != actual.OIDCIssuer || expected.SourceRepository != actual.SourceRepository ||
		expected.Workflow != actual.Workflow || expected.Ref != actual.Ref || expected.PublisherIdentity != actual.PublisherIdentity ||
		expected.SourceCommit != actual.SourceCommit || expected.WorkflowCommit != actual.WorkflowCommit ||
		expected.BuildConfigCommit != actual.BuildConfigCommit || expected.TrustedRootDigest != actual.TrustedRootDigest {
		return fmt.Errorf("does not equal the checkpoint publisher/root/source provenance")
	}
	return nil
}

func readRegular(path, label string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return raw, nil
}

func writePublicFile(root, relative string, raw []byte) error {
	if !safeRelative(relative) {
		return fmt.Errorf("unsafe evidence path %q", relative)
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("write evidence file %s: %w", relative, err)
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return fmt.Errorf("write evidence file %s: %w", relative, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close evidence file %s: %w", relative, err)
	}
	return nil
}

func inventoryRegularFiles(root string) ([]string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect evidence root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("evidence root is not a regular directory")
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is forbidden in evidence: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported evidence entry: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func safeRelative(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || (character > '9' && character < 'a') || character > 'f' {
			return false
		}
	}
	return value != strings.Repeat("0", 40)
}
