package publishertrust

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/tako0614/takoform/formpackage"
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

func TestPrepareSigningRequestEmitsExactCoreSubjectsAndRefusesOverwrite(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join("..", "..")
	output := filepath.Join(t.TempDir(), "request")

	report, err := PrepareSigningRequest(repositoryRoot, output)
	if err != nil {
		t.Fatalf("prepare signing request: %v", err)
	}
	if report.Status != SigningRequiredStatus || report.PackageCount != 16 {
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
