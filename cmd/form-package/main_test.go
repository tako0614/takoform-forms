package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const moduleWorkerPublicationJSON = `{
  "apiVersion": "packages.forms.takoform.com/v1alpha5",
  "releaseId": "k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2nn5shk3dfk5xxe23foi",
  "artifactId": "sha256-931eda33c673a640530b81779a5821ed27b9244c9f13dec9660867173aa69405",
  "tag": "forms/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2nn5shk3dfk5xxe23foi/sha256-931eda33c673a640530b81779a5821ed27b9244c9f13dec9660867173aa69405",
  "sourcePath": "forms/releases/k-mvsgozjomzxxe3ltfz2gc23pmzxxe3jomnxw2l2nn5shk3dfk5xxe23foi/sha256-931eda33c673a640530b81779a5821ed27b9244c9f13dec9660867173aa69405"
}
`

func TestVerifyCommandEmitsExactPublicationIdentityJSON(t *testing.T) {
	packageRoot := copyPackage(t, filepath.Join("..", "..", "forms", "candidates", "edge.forms.takoform.com", "module-worker"))
	var output bytes.Buffer
	if err := run([]string{"verify", packageRoot}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != moduleWorkerPublicationJSON {
		t.Fatalf("publication identity = %q, want %q", output.String(), moduleWorkerPublicationJSON)
	}
}

func TestVerifyCommandRejectsTamperedPackage(t *testing.T) {
	packageRoot := copyPackage(t, filepath.Join("..", "..", "forms", "candidates", "edge.forms.takoform.com", "module-worker"))
	definitionPath := filepath.Join(packageRoot, "definition.json")
	raw, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte("Module Worker"), []byte("Tampered Worker"), 1)
	if bytes.Equal(tampered, raw) {
		t.Fatal("test fixture did not contain the expected title")
	}
	if err := os.WriteFile(definitionPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"verify", packageRoot}, &output); err == nil {
		t.Fatal("tampered package was accepted")
	}
	if output.Len() != 0 {
		t.Fatalf("tampered package emitted output: %q", output.String())
	}
}

func TestRunRejectsMissingAndUnexpectedArguments(t *testing.T) {
	for _, arguments := range [][]string{
		nil,
		{"verify"},
		{"verify", "one", "two"},
		{"unknown", "path"},
	} {
		var output bytes.Buffer
		if err := run(arguments, &output); err == nil {
			t.Fatalf("arguments %v were unexpectedly accepted", arguments)
		}
		if output.Len() != 0 {
			t.Fatalf("arguments %v emitted output: %q", arguments, output.String())
		}
	}
}

func copyPackage(t *testing.T, source string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "package")
	if err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	return target
}
