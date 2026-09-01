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
	"slices"
	"sort"
	"strings"

	"github.com/tako0614/takoform/formpackage"
	"github.com/tako0614/takoform/trust"
)

const (
	Family                             = "edge.forms.takoform.com"
	ExpectedPackageCount               = 17
	CoreVersion                        = "v1.1.0"
	SigningRequiredStatus              = "signing-required"
	VerifiedPublicationStatus          = "verified"
	GenesisMode                        = "genesis"
	AdvancementMode                    = "advancement"
	PublisherRepository                = "https://github.com/tako0614/takoform-forms"
	PublisherWorkflow                  = PublisherRepository + "/.github/workflows/form-package-signing.yml"
	PublisherRef                       = "refs/heads/main"
	PublisherPolicyPath                = "publisher-policy.json"
	TrustedRootPath                    = "trusted-root.json"
	RevocationCheckpointPath           = "revocations/checkpoint.json"
	RevocationBundlePath               = "revocations/checkpoint.sigstore.json"
	RevocationStatementsPath           = "revocations/statements"
	RevocationHistoryPath              = "revocations/history/checkpoints"
	PackageIndexName                   = "package-index.json"
	PackageBundleName                  = "package-index.sigstore.json"
	TrustSetsRelativePath              = "forms/trust/sets"
	AbandonedPrepublicationPath        = "forms/trust/abandoned-prepublication.json"
	AbandonedPrepublicationFormat      = "takoform.abandoned-prepublication@v1"
	AbandonedPrepublicationDisposition = "evidence-only"
	AbandonedPrepublicationSetID       = "cdd30b711e2c6857b1b4d247b1471f5676904933"
	AbandonedPrepublicationSetTag      = "forms/sets/cdd30b711e2c6857b1b4d247b1471f5676904933"
	MaxRevocationSequence              = 1024
	setTagPrefix                       = "forms/sets/"
	revocationTagPrefix                = "forms/revocations/v"
)

const (
	publisherPolicySource          = "forms/trust/publisher-policy.json"
	trustedRootSource              = "forms/trust/trusted-root.json"
	candidateSetSource             = "forms/candidates/edge.forms.takoform.com/candidate-set.json"
	retainedPackageInventorySource = "forms/retained-packages.json"
	abandonedPrepublicationSource  = AbandonedPrepublicationPath
	revocationSourceRoot           = "forms/revocations"
	revocationSourceCheckpoints    = "forms/revocations/checkpoints"
)

const expectedRetainedPackageCount = 2

type retainedPackageInventory struct {
	Format   string                 `json:"format"`
	Family   string                 `json:"family"`
	Packages []retainedPackageEntry `json:"packages"`
}

type retainedPackageEntry struct {
	FormRef       formpackage.FormRef `json:"formRef"`
	PackageDigest string              `json:"packageDigest"`
	ReleaseID     string              `json:"releaseId"`
	ArtifactID    string              `json:"artifactId"`
	Tag           string              `json:"tag"`
	SourcePath    string              `json:"sourcePath"`
}

// AbandonedPrepublicationRecord is the one-time recovery declaration for a
// cryptographically verified set that is retained only as audit evidence.
// It is intentionally a singleton format: adding another abandoned set
// requires a new format and explicit architecture decision.
type AbandonedPrepublicationRecord struct {
	Format               string                           `json:"format"`
	Family               string                           `json:"family"`
	SetID                string                           `json:"setId"`
	SetTag               string                           `json:"setTag"`
	Disposition          string                           `json:"disposition"`
	EvidenceOnlyPackages []AbandonedPrepublicationPackage `json:"evidenceOnlyPackages"`
}

type AbandonedPrepublicationPackage struct {
	FormRef       formpackage.FormRef `json:"formRef"`
	PackageDigest string              `json:"packageDigest"`
	ReleaseID     string              `json:"releaseId"`
	ArtifactID    string              `json:"artifactId"`
	Tag           string              `json:"tag"`
	SourcePath    string              `json:"sourcePath"`
}

type SigningSubject struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type PreparationReport struct {
	Status            string           `json:"status"`
	CoreVersion       string           `json:"coreVersion"`
	Family            string           `json:"family"`
	Mode              string           `json:"mode"`
	Sequence          uint64           `json:"sequence"`
	CheckpointVersion string           `json:"checkpointVersion"`
	PreviousSetID     string           `json:"previousSetId,omitempty"`
	RevocationTag     string           `json:"revocationTag,omitempty"`
	PackageCount      int              `json:"packageCount"`
	Output            string           `json:"output"`
	Subjects          []SigningSubject `json:"subjects"`
}

type PackageVerification struct {
	Kind          string                         `json:"kind"`
	FormRef       formpackage.FormRef            `json:"formRef"`
	PackageDigest string                         `json:"packageDigest"`
	Locator       formpackage.PublicationLocator `json:"locator"`
	Bundle        trust.BundleVerification       `json:"bundle"`
}

type VerificationReport struct {
	Status               string                                 `json:"status"`
	CoreVersion          string                                 `json:"coreVersion"`
	Family               string                                 `json:"family"`
	SetID                string                                 `json:"setId"`
	SetTag               string                                 `json:"setTag"`
	Disposition          string                                 `json:"disposition,omitempty"`
	EvidenceOnlyPackages []AbandonedPrepublicationPackage       `json:"evidenceOnlyPackages,omitempty"`
	PackageCount         int                                    `json:"packageCount"`
	PublisherIdentity    string                                 `json:"publisherIdentity"`
	SourceCommit         string                                 `json:"sourceCommit"`
	WorkflowCommit       string                                 `json:"workflowCommit"`
	BuildConfigCommit    string                                 `json:"buildConfigCommit"`
	Checkpoint           trust.RevocationCheckpointVerification `json:"checkpoint"`
	CheckpointHistory    []PreviousCheckpointVerification       `json:"checkpointHistory"`
	PreviousCheckpoint   *PreviousCheckpointVerification        `json:"previousCheckpoint,omitempty"`
	RevocationTag        string                                 `json:"revocationTag,omitempty"`
	RevocationTags       []string                               `json:"revocationTags"`
	Statements           []RevocationStatementVerification      `json:"statements"`
	Packages             []PackageVerification                  `json:"packages"`
}

type PreviousCheckpointVerification struct {
	SetID             string                              `json:"setId"`
	SetTag            string                              `json:"setTag"`
	CheckpointVersion string                              `json:"checkpointVersion"`
	Pin               formpackage.RevocationCheckpointPin `json:"pin"`
}

type RevocationStatementVerification struct {
	Sequence         uint64              `json:"sequence"`
	StatementVersion string              `json:"statementVersion"`
	StatementDigest  string              `json:"statementDigest"`
	PackageDigest    string              `json:"packageDigest"`
	FormRef          formpackage.FormRef `json:"formRef"`
	SourcePath       string              `json:"sourcePath"`
	Tag              string              `json:"tag"`
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

type revocationAdvancement struct {
	Statement formpackage.RevocationStatement
	Entry     formpackage.RevocationCheckpointEntry
	Pin       formpackage.RevocationCheckpointPin
	Tag       string
}

type verifiedRevocationStatement struct {
	raw          []byte
	verification RevocationStatementVerification
}

type verifiedRevocationCheckpoint struct {
	raw          []byte
	bundle       []byte
	verification trust.RevocationCheckpointVerification
}

type verifiedRevocationChain struct {
	statements  []verifiedRevocationStatement
	checkpoints []verifiedRevocationCheckpoint
}

type verifiedEvidence struct {
	report      VerificationReport
	revocations verifiedRevocationChain
}

// ReadAbandonedPrepublication validates the source-controlled one-time
// recovery record. Every package locator is compared with an exact immutable
// identity; a Core-valid historical root alone is not enough authority to be
// classified as evidence-only.
func ReadAbandonedPrepublication(repositoryRoot string) (AbandonedPrepublicationRecord, error) {
	raw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(abandonedPrepublicationSource)), "abandoned prepublication manifest")
	if err != nil {
		return AbandonedPrepublicationRecord{}, err
	}
	var record AbandonedPrepublicationRecord
	if err := decodeStrict(raw, &record); err != nil {
		return AbandonedPrepublicationRecord{}, fmt.Errorf("decode abandoned prepublication manifest: %w", err)
	}
	if record.Format != AbandonedPrepublicationFormat || record.Family != Family ||
		record.SetID != AbandonedPrepublicationSetID ||
		record.SetTag != AbandonedPrepublicationSetTag ||
		record.Disposition != AbandonedPrepublicationDisposition ||
		len(record.EvidenceOnlyPackages) != len(expectedAbandonedPrepublicationPackages()) {
		return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication manifest must contain exactly %d evidence-only %s roots", len(expectedAbandonedPrepublicationPackages()), Family)
	}
	expected := expectedAbandonedPrepublicationPackages()
	seen := make(map[string]struct{}, len(record.EvidenceOnlyPackages))
	for _, entry := range record.EvidenceOnlyPackages {
		key := entry.FormRef.Kind + "@" + entry.FormRef.DefinitionVersion
		want, ok := expected[key]
		if !ok {
			return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication entry %s is not an exact abandoned identity", key)
		}
		if _, duplicate := seen[key]; duplicate {
			return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication manifest repeats %s", key)
		}
		seen[key] = struct{}{}
		if entry != want {
			return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication entry %s differs from the exact abandoned identity", key)
		}
		if !formpackage.ValidDigest(entry.PackageDigest) || !safeRelative(entry.ReleaseID) ||
			!safeRelative(entry.ArtifactID) || !safeRelative(entry.Tag) || !safeRelative(entry.SourcePath) {
			return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication entry %s has an unsafe locator", key)
		}
	}
	if len(seen) != len(expected) {
		return AbandonedPrepublicationRecord{}, fmt.Errorf("abandoned prepublication manifest is missing one or more exact identities")
	}
	return record, nil
}

func expectedAbandonedPrepublicationPackages() map[string]AbandonedPrepublicationPackage {
	return map[string]AbandonedPrepublicationPackage{
		"ObjectBucket@0.1.0": {
			FormRef: formpackage.FormRef{
				APIVersion: Family, Kind: "ObjectBucket", DefinitionVersion: "0.1.0",
				SchemaDigest: "sha256:eeda7b2fe4450bdd2301a348c27d7ade81b0a94bf9708655875329d72f902c57",
			},
			PackageDigest: "sha256:52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
			ReleaseID:     "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2pmjvgky3uij2wg23foq",
			ArtifactID:    "sha256-52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
			Tag:           "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2pmjvgky3uij2wg23foq/sha256-52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
			SourcePath:    "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2pmjvgky3uij2wg23foq/sha256-52a0cd0b11d35fbf8ab57ac7d5717f550efa77a2b20997b8ac0abdf3e4752200",
		},
		"WorkerDeployment@0.2.0": {
			FormRef: formpackage.FormRef{
				APIVersion: Family, Kind: "WorkerDeployment", DefinitionVersion: "0.2.0",
				SchemaDigest: "sha256:247d64335cbff296efc0298aa6811f299714fe7187d29aec6f73ed734e978756",
			},
			PackageDigest: "sha256:f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
			ReleaseID:     "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu",
			ArtifactID:    "sha256-f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
			Tag:           "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu/sha256-f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
			SourcePath:    "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu/sha256-f90f1b86cc9311d9457cd1cf0d665e6a310367d52e3f8e8c5c6c5acff842526d",
		},
		"WorkerVersion@0.3.0": {
			FormRef: formpackage.FormRef{
				APIVersion: Family, Kind: "WorkerVersion", DefinitionVersion: "0.3.0",
				SchemaDigest: "sha256:e82dce714f8b623ca926379c855ee9e314c83262e5564828ccc37be2dbe05820",
			},
			PackageDigest: "sha256:d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
			ReleaseID:     "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa",
			ArtifactID:    "sha256-d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
			Tag:           "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa/sha256-d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
			SourcePath:    "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa/sha256-d1ccfb0b47a4110f4ffbe6e842433639b1114feb11d5a690c9dc2ee1f938dd52",
		},
	}
}

// validateRevocationAdvancement binds one exact canonical statement to the
// final entry of one exact canonical checkpoint, then delegates all sequence,
// predecessor-digest, and retained-prefix continuity to released Core v1.1.0.
func validateRevocationAdvancement(previous formpackage.RevocationCheckpointPin, statementRaw, checkpointRaw []byte) (revocationAdvancement, error) {
	canonicalStatement, err := formpackage.Canonicalize(statementRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("canonicalize revocation statement: %w", err)
	}
	if !bytes.Equal(statementRaw, canonicalStatement) {
		return revocationAdvancement{}, fmt.Errorf("revocation statement bytes must be RFC 8785 canonical JSON")
	}
	canonicalCheckpoint, err := formpackage.Canonicalize(checkpointRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("canonicalize revocation checkpoint: %w", err)
	}
	if !bytes.Equal(checkpointRaw, canonicalCheckpoint) {
		return revocationAdvancement{}, fmt.Errorf("revocation checkpoint bytes must be RFC 8785 canonical JSON")
	}
	statement, err := formpackage.ValidateRevocationStatement(statementRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("Core %s revocation statement verification: %w", CoreVersion, err)
	}
	entry, err := formpackage.RevocationCheckpointEntryForStatement(statementRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("Core %s revocation statement entry: %w", CoreVersion, err)
	}
	checkpoint, err := formpackage.ValidateRevocationCheckpoint(checkpointRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("Core %s revocation checkpoint verification: %w", CoreVersion, err)
	}
	if checkpoint.Sequence > MaxRevocationSequence {
		return revocationAdvancement{}, fmt.Errorf("revocation checkpoint sequence %d exceeds publisher replay bound %d", checkpoint.Sequence, MaxRevocationSequence)
	}
	extension, err := trust.VerifyRevocationCheckpointExtension(&previous, checkpointRaw)
	if err != nil {
		return revocationAdvancement{}, fmt.Errorf("Core %s revocation checkpoint continuity: %w", CoreVersion, err)
	}
	if checkpoint.Sequence == 0 || len(checkpoint.Entries) == 0 || checkpoint.Entries[len(checkpoint.Entries)-1] != entry {
		return revocationAdvancement{}, fmt.Errorf("revocation checkpoint final entry does not identify the exact new statement bytes")
	}
	if statement.Sequence != extension.Pin.Sequence || statement.StatementVersion != checkpoint.CheckpointVersion {
		return revocationAdvancement{}, fmt.Errorf("revocation statement sequence/version does not equal the advanced checkpoint")
	}
	return revocationAdvancement{
		Statement: statement,
		Entry:     entry,
		Pin:       extension.Pin,
		Tag:       revocationTagPrefix + statement.StatementVersion,
	}, nil
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
	if err := verifySourceRevocationHistory(repositoryRoot, verifiedRevocationChain{
		checkpoints: []verifiedRevocationCheckpoint{{raw: genesis}},
	}, true); err != nil {
		return PreparationReport{}, fmt.Errorf("genesis source closure: %w", err)
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
		Mode: GenesisMode, Sequence: 0, CheckpointVersion: "0.0.0",
		PackageCount: len(packages), Output: output, Subjects: subjects,
	}, nil
}

// PrepareRevocationSigningRequest creates the next signing request from one
// already verified predecessor set and one exact source-controlled statement /
// cumulative checkpoint pair. The request carries the complete bounded signed
// checkpoint history so a fresh anonymous verifier never trusts a caller-
// supplied pin or serialized verification report.
func PrepareRevocationSigningRequest(repositoryRoot, previousSetRoot, statementVersion, output string) (PreparationReport, error) {
	if err := requireReleasedCore(); err != nil {
		return PreparationReport{}, err
	}
	if !safeRelative(statementVersion) || strings.Contains(statementVersion, "/") || statementVersion == "0.0.0" {
		return PreparationReport{}, fmt.Errorf("revocation statement version %q is not a safe non-genesis SemVer path", statementVersion)
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
	previous, err := verifyPublishedSetEvidence(repositoryRoot, previousSetRoot)
	if err != nil {
		return PreparationReport{}, fmt.Errorf("verify predecessor publisher set: %w", err)
	}
	if len(previous.revocations.checkpoints) == 0 {
		return PreparationReport{}, fmt.Errorf("verified predecessor publisher set has no checkpoint capability")
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
	statementRaw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(revocationStatementSourcePath(statementVersion))), "new revocation statement")
	if err != nil {
		return PreparationReport{}, err
	}
	checkpointRaw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(revocationCheckpointSourcePath(statementVersion))), "new cumulative revocation checkpoint")
	if err != nil {
		return PreparationReport{}, err
	}
	previousCheckpoint := previous.revocations.checkpoints[len(previous.revocations.checkpoints)-1].verification
	advancement, err := validateRevocationAdvancement(previousCheckpoint.Pin, statementRaw, checkpointRaw)
	if err != nil {
		return PreparationReport{}, err
	}
	if advancement.Statement.StatementVersion != statementVersion {
		return PreparationReport{}, fmt.Errorf("revocation source path version %s differs from statementVersion %s", statementVersion, advancement.Statement.StatementVersion)
	}
	proposed := verifiedRevocationChain{
		statements: append(append([]verifiedRevocationStatement(nil), previous.revocations.statements...), verifiedRevocationStatement{
			raw: statementRaw,
			verification: RevocationStatementVerification{
				Sequence: advancement.Entry.Sequence, StatementVersion: advancement.Entry.StatementVersion,
				StatementDigest: advancement.Entry.StatementDigest, PackageDigest: advancement.Entry.PackageDigest,
				FormRef: advancement.Entry.FormRef, SourcePath: revocationStatementSourcePath(statementVersion), Tag: advancement.Tag,
			},
		}),
		checkpoints: append(append([]verifiedRevocationCheckpoint(nil), previous.revocations.checkpoints...), verifiedRevocationCheckpoint{raw: checkpointRaw}),
	}
	if err := verifySourceRevocationHistory(repositoryRoot, proposed, true); err != nil {
		return PreparationReport{}, err
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return PreparationReport{}, fmt.Errorf("create signing output parent: %w", err)
	}
	if err := os.Mkdir(output, 0o755); err != nil {
		return PreparationReport{}, fmt.Errorf("create signing output create-only: %w", err)
	}
	for _, file := range []struct {
		relative string
		raw      []byte
	}{
		{relative: PublisherPolicyPath, raw: policyRaw},
		{relative: TrustedRootPath, raw: rootRaw},
		{relative: RevocationCheckpointPath, raw: checkpointRaw},
	} {
		if err := writePublicFile(output, file.relative, file.raw); err != nil {
			return PreparationReport{}, err
		}
	}
	for _, statement := range proposed.statements {
		if err := writePublicFile(output, revocationStatementEvidencePath(statement.verification.StatementVersion), statement.raw); err != nil {
			return PreparationReport{}, err
		}
	}
	for _, checkpoint := range previous.revocations.checkpoints {
		version := checkpoint.verification.CheckpointVersion
		if err := writePublicFile(output, revocationHistoryCheckpointPath(version), checkpoint.raw); err != nil {
			return PreparationReport{}, err
		}
		if err := writePublicFile(output, revocationHistoryBundlePath(version), checkpoint.bundle); err != nil {
			return PreparationReport{}, err
		}
	}

	subjects := make([]SigningSubject, 0, len(packages)+1)
	subjects = append(subjects, SigningSubject{
		Role: "revocation-checkpoint", Path: RevocationCheckpointPath, Digest: formpackage.DigestBytes(checkpointRaw),
	})
	for _, packageValue := range packages {
		relative := packageSubjectPath(packageValue.locator)
		if err := writePublicFile(output, relative, packageValue.canonicalIndex); err != nil {
			return PreparationReport{}, err
		}
		subjects = append(subjects, SigningSubject{Role: "package-index", Path: relative, Digest: packageValue.candidate.PackageDigest})
	}
	sort.Slice(subjects, func(left, right int) bool { return subjects[left].Path < subjects[right].Path })
	return PreparationReport{
		Status: SigningRequiredStatus, CoreVersion: CoreVersion, Family: Family,
		Mode: AdvancementMode, Sequence: advancement.Pin.Sequence, CheckpointVersion: statementVersion,
		PreviousSetID: previous.report.SetID, RevocationTag: advancement.Tag,
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
	verified, err := verifyEvidence(repositoryRoot, evidenceRoot, expectedSourceCommit, true, packages)
	if err != nil {
		return VerificationReport{}, err
	}
	return verified.report, nil
}

// VerifyPublishedSet reruns the same cryptographic checks over one installed
// create-only set. Package subjects are re-derived from the release closures;
// the set contains no serialized verification shortcut.
func VerifyPublishedSet(repositoryRoot, setRoot string) (VerificationReport, error) {
	verified, err := verifyPublishedSetEvidence(repositoryRoot, setRoot)
	if err != nil {
		return VerificationReport{}, err
	}
	if verified.report.SetID == AbandonedPrepublicationSetID {
		if err := classifyAbandonedPrepublication(repositoryRoot, &verified); err != nil {
			return VerificationReport{}, err
		}
	}
	return verified.report, nil
}

func classifyAbandonedPrepublication(repositoryRoot string, verified *verifiedEvidence) error {
	record, err := ReadAbandonedPrepublication(repositoryRoot)
	if err != nil {
		return err
	}
	if record.SetID != verified.report.SetID || record.SetTag != verified.report.SetTag {
		return fmt.Errorf("abandoned prepublication manifest set identity differs from the verified publisher set")
	}
	current, err := discoverCurrentPackageIdentities(repositoryRoot)
	if err != nil {
		return err
	}
	if err := verifyRetainedPackageInventory(repositoryRoot); err != nil {
		return err
	}
	retained, err := readRetainedPackageEntries(repositoryRoot)
	if err != nil {
		return err
	}
	currentKeys := make(map[string]struct{}, len(current))
	for _, packageValue := range current {
		currentKeys[packageIdentityKey(packageValue.candidate.FormRef, packageValue.candidate.PackageDigest, packageValue.locator)] = struct{}{}
	}
	retainedKeys := make(map[string]struct{}, len(retained))
	for _, entry := range retained {
		retainedKeys[packageIdentityKey(entry.FormRef, entry.PackageDigest, formpackage.PublicationLocator{
			APIVersion: "packages.forms.takoform.com/v1alpha5", ReleaseID: entry.ReleaseID,
			ArtifactID: entry.ArtifactID, Tag: entry.Tag, SourcePath: entry.SourcePath,
		})] = struct{}{}
	}
	expected := make(map[string]AbandonedPrepublicationPackage, len(record.EvidenceOnlyPackages))
	for _, entry := range record.EvidenceOnlyPackages {
		expected[packageIdentityKey(entry.FormRef, entry.PackageDigest, formpackage.PublicationLocator{
			APIVersion: "packages.forms.takoform.com/v1alpha5", ReleaseID: entry.ReleaseID,
			ArtifactID: entry.ArtifactID, Tag: entry.Tag, SourcePath: entry.SourcePath,
		})] = entry
	}
	found := make(map[string]struct{}, len(expected))
	for _, packageReport := range verified.report.Packages {
		key := packageIdentityKey(packageReport.FormRef, packageReport.PackageDigest, packageReport.Locator)
		if _, ok := currentKeys[key]; ok {
			continue
		}
		if _, ok := retainedKeys[key]; ok {
			continue
		}
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("verified abandoned set package %s is neither current, retained, nor an exact evidence-only identity", packageReport.Kind)
		}
		if _, duplicate := found[key]; duplicate {
			return fmt.Errorf("verified abandoned set repeats evidence-only package %s", packageReport.Kind)
		}
		found[key] = struct{}{}
	}
	if len(found) != len(expected) {
		return fmt.Errorf("verified abandoned set evidence-only package set differs from the exact manifest")
	}
	verified.report.Disposition = record.Disposition
	verified.report.EvidenceOnlyPackages = append([]AbandonedPrepublicationPackage(nil), record.EvidenceOnlyPackages...)
	return nil
}

func verifyPublishedSetEvidence(repositoryRoot, setRoot string) (verifiedEvidence, error) {
	if err := requireReleasedCore(); err != nil {
		return verifiedEvidence{}, err
	}
	setRoot, err := filepath.Abs(setRoot)
	if err != nil {
		return verifiedEvidence{}, fmt.Errorf("resolve trust set: %w", err)
	}
	setID := filepath.Base(setRoot)
	if !validCommit(setID) {
		return verifiedEvidence{}, fmt.Errorf("trust set directory %q is not an exact source commit", setID)
	}
	packages, err := discoverPublishedSetPackages(repositoryRoot, setRoot)
	if err != nil {
		return verifiedEvidence{}, err
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

func verifyEvidence(repositoryRoot, evidenceRoot, expectedSourceCommit string, includesSubjects bool, packages []verifiedCandidate) (verifiedEvidence, error) {
	repositoryRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return verifiedEvidence{}, fmt.Errorf("resolve repository root: %w", err)
	}
	evidenceRoot, err = filepath.Abs(evidenceRoot)
	if err != nil {
		return verifiedEvidence{}, fmt.Errorf("resolve evidence root: %w", err)
	}

	pinnedPolicyRaw, policy, err := readPublisherPolicy(repositoryRoot)
	if err != nil {
		return verifiedEvidence{}, err
	}
	if err := validatePublisherPolicy(policy); err != nil {
		return verifiedEvidence{}, err
	}
	policyRaw, err := readRegular(filepath.Join(evidenceRoot, PublisherPolicyPath), "publisher policy")
	if err != nil {
		return verifiedEvidence{}, err
	}
	if !bytes.Equal(policyRaw, pinnedPolicyRaw) {
		return verifiedEvidence{}, fmt.Errorf("evidence publisher policy differs from the repository-pinned policy")
	}
	rootRaw, err := readRegular(filepath.Join(evidenceRoot, TrustedRootPath), "trusted root")
	if err != nil {
		return verifiedEvidence{}, err
	}
	pinnedRootRaw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(trustedRootSource)), "repository-pinned trusted root")
	if err != nil {
		return verifiedEvidence{}, err
	}
	if !bytes.Equal(rootRaw, pinnedRootRaw) {
		return verifiedEvidence{}, fmt.Errorf("evidence trusted root differs from the repository-pinned trusted root")
	}
	checkpointRaw, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(RevocationCheckpointPath)), "revocation checkpoint")
	if err != nil {
		return verifiedEvidence{}, err
	}
	checkpointValue, err := formpackage.ValidateRevocationCheckpoint(checkpointRaw)
	if err != nil {
		return verifiedEvidence{}, fmt.Errorf("Core %s revocation checkpoint validation: %w", CoreVersion, err)
	}
	if checkpointValue.Sequence > MaxRevocationSequence {
		return verifiedEvidence{}, fmt.Errorf("revocation checkpoint sequence %d exceeds publisher replay bound %d", checkpointValue.Sequence, MaxRevocationSequence)
	}
	if err := verifyEvidenceInventory(evidenceRoot, packages, includesSubjects, checkpointValue); err != nil {
		return verifiedEvidence{}, err
	}
	checkpointBundle, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(RevocationBundlePath)), "revocation checkpoint signature bundle")
	if err != nil {
		return verifiedEvidence{}, err
	}
	revocations, err := verifyRevocationChain(evidenceRoot, checkpointRaw, checkpointBundle, rootRaw, policy, checkpointValue)
	if err != nil {
		return verifiedEvidence{}, err
	}
	if err := verifySourceRevocationHistory(repositoryRoot, revocations, includesSubjects); err != nil {
		return verifiedEvidence{}, err
	}
	checkpoint := revocations.checkpoints[len(revocations.checkpoints)-1].verification

	packageReports := make([]PackageVerification, 0, len(packages))
	for _, packageValue := range packages {
		subject := packageValue.canonicalIndex
		if includesSubjects {
			prepared, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(packageSubjectPath(packageValue.locator))), packageValue.candidate.Kind+" package-index subject")
			if err != nil {
				return verifiedEvidence{}, err
			}
			if !bytes.Equal(prepared, subject) {
				return verifiedEvidence{}, fmt.Errorf("%s package-index subject differs from the exact Core-canonical release bytes", packageValue.candidate.Kind)
			}
		}
		bundlePath := filepath.Join(evidenceRoot, filepath.FromSlash(packageBundlePath(packageValue.locator)))
		bundleRaw, err := readRegular(bundlePath, packageValue.candidate.Kind+" signature bundle")
		if err != nil {
			return verifiedEvidence{}, err
		}
		bundleReport, err := trust.VerifyBundle(subject, bundleRaw, rootRaw, policy)
		if err != nil {
			return verifiedEvidence{}, fmt.Errorf("Core %s %s package signature verification: %w", CoreVersion, packageValue.candidate.Kind, err)
		}
		if bundleReport.SubjectDigest != packageValue.candidate.PackageDigest {
			return verifiedEvidence{}, fmt.Errorf("%s signed subject digest %s differs from verified package digest %s", packageValue.candidate.Kind, bundleReport.SubjectDigest, packageValue.candidate.PackageDigest)
		}
		if err := checkpoint.CheckNotRevoked(packageValue.candidate.PackageDigest, packageValue.candidate.FormRef); err != nil {
			return verifiedEvidence{}, fmt.Errorf("Core %s %s revocation check: %w", CoreVersion, packageValue.candidate.Kind, err)
		}
		packageReports = append(packageReports, PackageVerification{
			Kind: packageValue.candidate.Kind, FormRef: packageValue.candidate.FormRef,
			PackageDigest: packageValue.candidate.PackageDigest, Locator: packageValue.locator, Bundle: bundleReport,
		})
	}

	provenance := checkpoint.Bundle
	for _, packageReport := range packageReports {
		if err := sameProvenance(provenance, packageReport.Bundle); err != nil {
			return verifiedEvidence{}, fmt.Errorf("%s signature provenance: %w", packageReport.Kind, err)
		}
	}
	if err := validateOfficialBundleCommit(provenance, expectedSourceCommit); err != nil {
		return verifiedEvidence{}, err
	}
	statements := make([]RevocationStatementVerification, 0, len(revocations.statements))
	tags := make([]string, 0, len(revocations.statements))
	for _, statement := range revocations.statements {
		statements = append(statements, statement.verification)
		tags = append(tags, statement.verification.Tag)
	}
	checkpointHistory := make([]PreviousCheckpointVerification, 0, len(revocations.checkpoints))
	seenCheckpointSets := make(map[string]struct{}, len(revocations.checkpoints))
	for _, historical := range revocations.checkpoints {
		setID := historical.verification.Bundle.SourceCommit
		if _, duplicate := seenCheckpointSets[setID]; duplicate {
			return verifiedEvidence{}, fmt.Errorf("checkpoint history reuses publisher set identity %s", setID)
		}
		seenCheckpointSets[setID] = struct{}{}
		checkpointHistory = append(checkpointHistory, PreviousCheckpointVerification{
			SetID: setID, SetTag: setTagPrefix + setID,
			CheckpointVersion: historical.verification.CheckpointVersion,
			Pin:               historical.verification.Pin,
		})
	}
	report := VerificationReport{
		Status: VerifiedPublicationStatus, CoreVersion: CoreVersion, Family: Family,
		SetID: provenance.SourceCommit, SetTag: setTagPrefix + provenance.SourceCommit,
		PackageCount: len(packageReports), PublisherIdentity: provenance.PublisherIdentity,
		SourceCommit: provenance.SourceCommit, WorkflowCommit: provenance.WorkflowCommit,
		BuildConfigCommit: provenance.BuildConfigCommit, Checkpoint: checkpoint, CheckpointHistory: checkpointHistory,
		RevocationTags: tags, Statements: statements, Packages: packageReports,
	}
	if len(revocations.checkpoints) > 1 {
		previous := checkpointHistory[len(checkpointHistory)-2]
		report.PreviousCheckpoint = &previous
		report.RevocationTag = tags[len(tags)-1]
	}
	return verifiedEvidence{report: report, revocations: revocations}, nil
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

// discoverCurrentPackageIdentities verifies only the candidate closures. It
// intentionally does not require current release roots: abandoned-set
// classification runs before publication materializes the new identities.
func discoverCurrentPackageIdentities(repositoryRoot string) ([]verifiedCandidate, error) {
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
	seen := make(map[string]struct{}, len(candidates.Forms))
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
		key := packageIdentityKey(candidate.FormRef, candidate.PackageDigest, locator)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("duplicate current package identity %s", candidate.Kind)
		}
		seen[key] = struct{}{}
		indexRaw, err := readRegular(filepath.Join(candidateRoot, PackageIndexName), candidate.Kind+" candidate package index")
		if err != nil {
			return nil, err
		}
		canonical, err := formpackage.Canonicalize(indexRaw)
		if err != nil {
			return nil, fmt.Errorf("Core %s canonicalize %s candidate package index: %w", CoreVersion, candidate.Kind, err)
		}
		verified = append(verified, verifiedCandidate{candidate: candidate, locator: locator, releaseRoot: candidateRoot, canonicalIndex: canonical})
	}
	sort.Slice(verified, func(left, right int) bool { return verified[left].locator.Tag < verified[right].locator.Tag })
	return verified, nil
}

func readRetainedPackageEntries(repositoryRoot string) ([]retainedPackageEntry, error) {
	raw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(retainedPackageInventorySource)), "retained package inventory")
	if err != nil {
		return nil, err
	}
	var inventory retainedPackageInventory
	if err := decodeStrict(raw, &inventory); err != nil {
		return nil, fmt.Errorf("decode retained package inventory: %w", err)
	}
	return inventory.Packages, nil
}

func packageIdentityKey(formRef formpackage.FormRef, packageDigest string, locator formpackage.PublicationLocator) string {
	raw, _ := json.Marshal(struct {
		FormRef       formpackage.FormRef            `json:"formRef"`
		PackageDigest string                         `json:"packageDigest"`
		Locator       formpackage.PublicationLocator `json:"locator"`
	}{FormRef: formRef, PackageDigest: packageDigest, Locator: locator})
	return string(raw)
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
	if err := verifyRetainedPackageInventory(repositoryRoot); err != nil {
		return nil, err
	}
	return verified, nil
}

// verifyRetainedPackageInventory authenticates the publisher-owned allowlist
// of pre-current package roots. A Core-valid package is not admitted merely
// because its content-addressed path looks plausible: every locator, FormRef,
// package digest, and complete package closure must equal one exact entry.
func verifyRetainedPackageInventory(repositoryRoot string) error {
	raw, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(retainedPackageInventorySource)), "retained package inventory")
	if err != nil {
		return err
	}
	var inventory retainedPackageInventory
	if err := decodeStrict(raw, &inventory); err != nil {
		return fmt.Errorf("decode retained package inventory: %w", err)
	}
	if inventory.Format != "takoform.retained-package-inventory@v1" || inventory.Family != Family || len(inventory.Packages) != expectedRetainedPackageCount {
		return fmt.Errorf("retained package inventory must contain exactly %d %s roots", expectedRetainedPackageCount, Family)
	}
	expected := map[string]retainedPackageEntry{
		"WorkerDeployment@0.1.0": {
			FormRef: formpackage.FormRef{
				APIVersion: Family, Kind: "WorkerDeployment", DefinitionVersion: "0.1.0",
				SchemaDigest: "sha256:0d2bca351b8ecade0a1ebbddf2463bba22910313ff916414112ec8762204e769",
			},
			PackageDigest: "sha256:535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
			ReleaseID:     "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu",
			ArtifactID:    "sha256-535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
			Tag:           "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu/sha256-535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
			SourcePath:    "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlsirsxa3dppfwwk3tu/sha256-535133f0a79c2091162f2dc237d177702e5e5db5c558c6c2e5bf5bcd76d6ff17",
		},
		"WorkerVersion@0.2.0": {
			FormRef: formpackage.FormRef{
				APIVersion: Family, Kind: "WorkerVersion", DefinitionVersion: "0.2.0",
				SchemaDigest: "sha256:3d4eeed966867a1ef8d7ce629a77c4b9687c6d48d3e496d22314b29aff0a42ed",
			},
			PackageDigest: "sha256:63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
			ReleaseID:     "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa",
			ArtifactID:    "sha256-63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
			Tag:           "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa/sha256-63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
			SourcePath:    "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2xn5zgwzlskzsxe43jn5xa/sha256-63cf4dd3e96f575d1d1631c87d2e0ff0410ca820e142b8d4fa73e30aaa651025",
		},
	}
	seen := make(map[string]struct{}, len(inventory.Packages))
	for _, entry := range inventory.Packages {
		key := entry.FormRef.Kind + "@" + entry.FormRef.DefinitionVersion
		want, ok := expected[key]
		if !ok {
			return fmt.Errorf("retained package inventory entry %s is not an exact retained identity", key)
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("retained package inventory repeats %s", key)
		}
		seen[key] = struct{}{}
		if entry != want {
			return fmt.Errorf("retained package inventory entry %s differs from the exact published identity", key)
		}
		if !formpackage.ValidDigest(entry.PackageDigest) || !safeRelative(entry.ReleaseID) || !safeRelative(entry.ArtifactID) || !safeRelative(entry.Tag) || !safeRelative(entry.SourcePath) {
			return fmt.Errorf("retained package inventory entry %s has an unsafe locator", key)
		}
		releaseRoot := filepath.Join(repositoryRoot, filepath.FromSlash(entry.SourcePath))
		report, err := formpackage.VerifyDirectory(releaseRoot)
		if err != nil {
			return fmt.Errorf("Core %s retained release verification for %s: %w", CoreVersion, key, err)
		}
		if report.FormRef != entry.FormRef || report.PackageDigest != entry.PackageDigest {
			return fmt.Errorf("%s retained release identity differs from its inventory entry", key)
		}
		capability, ok := report.VerifiedPackage()
		if !ok {
			return fmt.Errorf("Core %s did not issue a retained package capability for %s", CoreVersion, key)
		}
		locator, err := formpackage.PublicationLocatorFor(capability.PackageIndex(), capability.PackageDigest())
		if err != nil {
			return fmt.Errorf("Core %s retained publication locator for %s: %w", CoreVersion, key, err)
		}
		if locator.APIVersion != "packages.forms.takoform.com/v1alpha5" || locator.ReleaseID != entry.ReleaseID || locator.ArtifactID != entry.ArtifactID || locator.Tag != entry.Tag || locator.SourcePath != entry.SourcePath {
			return fmt.Errorf("%s retained locator differs from its exact inventory entry", key)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("retained package inventory is missing one or more exact identities")
	}
	return nil
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

func verifyRevocationChain(
	evidenceRoot string,
	currentRaw, currentBundle, trustedRootRaw []byte,
	policy trust.PublisherPolicy,
	current formpackage.RevocationCheckpoint,
) (verifiedRevocationChain, error) {
	statements := make([]verifiedRevocationStatement, 0, current.Sequence)
	for index, expected := range current.Entries {
		statementPath := revocationStatementEvidencePath(expected.StatementVersion)
		raw, err := readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(statementPath)), fmt.Sprintf("revocation statement sequence %d", index+1))
		if err != nil {
			return verifiedRevocationChain{}, err
		}
		entry, err := formpackage.RevocationCheckpointEntryForStatement(raw)
		if err != nil {
			return verifiedRevocationChain{}, fmt.Errorf("Core %s revocation statement %s: %w", CoreVersion, expected.StatementVersion, err)
		}
		if entry != expected {
			return verifiedRevocationChain{}, fmt.Errorf("revocation statement %s does not equal checkpoint entry sequence %d", expected.StatementVersion, expected.Sequence)
		}
		statements = append(statements, verifiedRevocationStatement{
			raw: raw,
			verification: RevocationStatementVerification{
				Sequence: expected.Sequence, StatementVersion: expected.StatementVersion,
				StatementDigest: expected.StatementDigest, PackageDigest: expected.PackageDigest,
				FormRef: expected.FormRef, SourcePath: revocationStatementSourcePath(expected.StatementVersion),
				Tag: revocationTagPrefix + expected.StatementVersion,
			},
		})
	}

	checkpoints := make([]verifiedRevocationCheckpoint, 0, current.Sequence+1)
	var previous *formpackage.RevocationCheckpointPin
	for sequence := uint64(0); sequence <= current.Sequence; sequence++ {
		checkpointRaw := currentRaw
		bundleRaw := currentBundle
		version := current.CheckpointVersion
		if sequence < current.Sequence {
			version = checkpointVersionAt(current, sequence)
			checkpointPath := revocationHistoryCheckpointPath(version)
			bundlePath := revocationHistoryBundlePath(version)
			var err error
			checkpointRaw, err = readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(checkpointPath)), fmt.Sprintf("revocation checkpoint history sequence %d", sequence))
			if err != nil {
				return verifiedRevocationChain{}, err
			}
			bundleRaw, err = readRegular(filepath.Join(evidenceRoot, filepath.FromSlash(bundlePath)), fmt.Sprintf("revocation checkpoint history bundle sequence %d", sequence))
			if err != nil {
				return verifiedRevocationChain{}, err
			}
		}
		checkpoint, err := formpackage.ValidateRevocationCheckpoint(checkpointRaw)
		if err != nil {
			return verifiedRevocationChain{}, fmt.Errorf("Core %s revocation checkpoint history sequence %d: %w", CoreVersion, sequence, err)
		}
		if checkpoint.APIVersion != formpackage.CurrentTrustAPIVersion || checkpoint.Sequence != sequence ||
			checkpoint.CheckpointVersion != version || !slices.Equal(checkpoint.Entries, current.Entries[:int(sequence)]) {
			return verifiedRevocationChain{}, fmt.Errorf("revocation checkpoint history sequence %d is not the exact retained cumulative prefix", sequence)
		}
		verification, err := trust.VerifyRevocationCheckpoint(checkpointRaw, bundleRaw, trustedRootRaw, policy, previous)
		if err != nil {
			return verifiedRevocationChain{}, fmt.Errorf("Core %s revocation checkpoint history sequence %d verification: %w", CoreVersion, sequence, err)
		}
		if err := validateOfficialBundleCommit(verification.Bundle, ""); err != nil {
			return verifiedRevocationChain{}, fmt.Errorf("revocation checkpoint history sequence %d provenance: %w", sequence, err)
		}
		checkpoints = append(checkpoints, verifiedRevocationCheckpoint{raw: checkpointRaw, bundle: bundleRaw, verification: verification})
		pin := verification.Pin
		previous = &pin
	}
	return verifiedRevocationChain{statements: statements, checkpoints: checkpoints}, nil
}

func verifySourceRevocationHistory(repositoryRoot string, chain verifiedRevocationChain, exact bool) error {
	expected := map[string][]byte{"README.md": nil}
	for index, statement := range chain.statements {
		version := statement.verification.StatementVersion
		statementSource := filepath.ToSlash(filepath.Join(version + ".json"))
		checkpointSource := filepath.ToSlash(filepath.Join("checkpoints", version+".json"))
		expected[statementSource] = statement.raw
		expected[checkpointSource] = chain.checkpoints[index+1].raw
		for relative, want := range map[string][]byte{statementSource: statement.raw, checkpointSource: chain.checkpoints[index+1].raw} {
			actual, err := readRegular(filepath.Join(repositoryRoot, filepath.FromSlash(revocationSourceRoot), filepath.FromSlash(relative)), "repository revocation source "+relative)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, want) {
				return fmt.Errorf("repository revocation source %s differs from the signed checkpoint chain", relative)
			}
		}
	}
	if !exact {
		return nil
	}
	actual, err := inventoryRegularFiles(filepath.Join(repositoryRoot, filepath.FromSlash(revocationSourceRoot)))
	if err != nil {
		return fmt.Errorf("inventory repository revocation source: %w", err)
	}
	for _, relative := range actual {
		if _, ok := expected[relative]; !ok {
			return fmt.Errorf("repository revocation source has an uncommitted advancement or rewritten history at %s", relative)
		}
	}
	for relative := range expected {
		if relative == "README.md" {
			continue
		}
		if !slices.Contains(actual, relative) {
			return fmt.Errorf("repository revocation source is missing signed history %s", relative)
		}
	}
	return nil
}

func validateOfficialBundleCommit(bundle trust.BundleVerification, expected string) error {
	commits := []struct {
		role  string
		value string
	}{
		{role: "source", value: bundle.SourceCommit},
		{role: "workflow", value: bundle.WorkflowCommit},
		{role: "build config", value: bundle.BuildConfigCommit},
	}
	for _, commit := range commits {
		if !validCommit(commit.value) {
			return fmt.Errorf("Core verification returned an invalid %s commit %q", commit.role, commit.value)
		}
		if expected != "" && commit.value != expected {
			return fmt.Errorf("signed %s commit %s differs from required protected-main commit %s", commit.role, commit.value, expected)
		}
	}
	if bundle.SourceCommit != bundle.WorkflowCommit || bundle.SourceCommit != bundle.BuildConfigCommit {
		return fmt.Errorf("official publisher source/workflow/build commits are not one exact commit")
	}
	return nil
}

func checkpointVersionAt(checkpoint formpackage.RevocationCheckpoint, sequence uint64) string {
	if sequence == 0 {
		return "0.0.0"
	}
	return checkpoint.Entries[sequence-1].StatementVersion
}

func revocationStatementEvidencePath(version string) string {
	return filepath.ToSlash(filepath.Join(RevocationStatementsPath, version+".json"))
}

func revocationHistoryCheckpointPath(version string) string {
	return filepath.ToSlash(filepath.Join(RevocationHistoryPath, version+".json"))
}

func revocationHistoryBundlePath(version string) string {
	return filepath.ToSlash(filepath.Join(RevocationHistoryPath, version+".sigstore.json"))
}

func revocationStatementSourcePath(version string) string {
	return filepath.ToSlash(filepath.Join(revocationSourceRoot, version+".json"))
}

func revocationCheckpointSourcePath(version string) string {
	return filepath.ToSlash(filepath.Join(revocationSourceCheckpoints, version+".json"))
}

func verifyEvidenceInventory(root string, packages []verifiedCandidate, includesSubjects bool, checkpoint formpackage.RevocationCheckpoint) error {
	allowed := map[string]struct{}{
		PublisherPolicyPath: {}, TrustedRootPath: {}, RevocationCheckpointPath: {}, RevocationBundlePath: {},
	}
	for _, entry := range checkpoint.Entries {
		allowed[revocationStatementEvidencePath(entry.StatementVersion)] = struct{}{}
	}
	for sequence := uint64(0); sequence < checkpoint.Sequence; sequence++ {
		version := checkpointVersionAt(checkpoint, sequence)
		allowed[revocationHistoryCheckpointPath(version)] = struct{}{}
		allowed[revocationHistoryBundlePath(version)] = struct{}{}
	}
	for _, packageValue := range packages {
		if includesSubjects {
			allowed[packageSubjectPath(packageValue.locator)] = struct{}{}
		}
		bundle := packageBundlePath(packageValue.locator)
		allowed[bundle] = struct{}{}
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
	requiredPaths := make([]string, 0, len(allowed))
	for required := range allowed {
		requiredPaths = append(requiredPaths, required)
	}
	sort.Strings(requiredPaths)
	for _, required := range requiredPaths {
		if _, ok := actualSet[required]; !ok {
			switch {
			case strings.HasSuffix(required, ".sigstore.json"):
				return fmt.Errorf("signature bundle is missing: %s", required)
			case strings.HasPrefix(required, "packages/") && strings.HasSuffix(required, "/"+PackageIndexName):
				return fmt.Errorf("required package-index subject %s is missing", required)
			default:
				return fmt.Errorf("required evidence file %s is missing", required)
			}
		}
	}
	return nil
}

func installedEvidencePaths(report VerificationReport) []string {
	paths := []string{PublisherPolicyPath, TrustedRootPath, RevocationCheckpointPath, RevocationBundlePath}
	for _, statement := range report.Statements {
		paths = append(paths, revocationStatementEvidencePath(statement.StatementVersion))
	}
	if report.Checkpoint.Pin.Sequence > 0 {
		paths = append(paths, revocationHistoryCheckpointPath("0.0.0"), revocationHistoryBundlePath("0.0.0"))
		for _, statement := range report.Statements[:len(report.Statements)-1] {
			paths = append(paths,
				revocationHistoryCheckpointPath(statement.StatementVersion),
				revocationHistoryBundlePath(statement.StatementVersion),
			)
		}
	}
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
