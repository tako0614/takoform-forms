package publishertrust

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

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
