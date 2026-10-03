package edgeformcatalog

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/tako0614/takoform/formpackage"
)

type executableInputContract struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	SchemaDigest string `json:"schemaDigest"`
}

type executableInputManifest struct {
	Format     string                    `json:"format"`
	Status     string                    `json:"status"`
	HostAPI    string                    `json:"hostApi"`
	Forms      []formpackage.FormRef     `json:"forms"`
	Interfaces []executableInputContract `json:"interfaces"`
	Bindings   []executableInputContract `json:"bindings"`
	Bundle     struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"bundle"`
	Cases struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"cases"`
}

func TestActorWorkflowExecutableInputPinsSelectedDefinitionsAndBundle(t *testing.T) {
	root := filepath.Join("..", "..", "conformance", "edge-runtime", "actor-workflow")
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest executableInputManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != "edge.actor-workflow-source-input" || manifest.Status != "unpublished-source-input" || manifest.HostAPI != "forms.takoform.com/v1" {
		t.Fatalf("corpus format/status/Host API = %q/%q/%q", manifest.Format, manifest.Status, manifest.HostAPI)
	}
	selected, err := RenderActorWorkflowSelectedSource()
	if err != nil {
		t.Fatal(err)
	}
	previous, err := RenderForms()
	if err != nil {
		t.Fatal(err)
	}
	var forms []formpackage.FormRef
	for index, current := range selected.Forms {
		if reflect.DeepEqual(current, previous[index]) {
			continue
		}
		ref, err := renderedFormRef(current)
		if err != nil {
			t.Fatal(err)
		}
		forms = append(forms, formpackage.FormRef{
			APIVersion: ref.APIVersion, Kind: ref.Kind,
			DefinitionVersion: ref.DefinitionVersion, SchemaDigest: ref.SchemaDigest,
		})
	}
	sort.Slice(forms, func(i, j int) bool { return forms[i].Kind < forms[j].Kind })
	sort.Slice(manifest.Forms, func(i, j int) bool { return manifest.Forms[i].Kind < manifest.Forms[j].Kind })
	if len(forms) != 9 || !reflect.DeepEqual(manifest.Forms, forms) {
		t.Fatalf("corpus FormRefs differ from selected Definitions: got %+v, want %+v", manifest.Forms, forms)
	}
	previousInterfaces, err := RenderInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	previousBindings, err := RenderBindings()
	if err != nil {
		t.Fatal(err)
	}
	checkContracts := func(label string, got []executableInputContract, current, old []RenderedContract, count int) {
		t.Helper()
		var want []executableInputContract
		for index, contract := range current {
			if reflect.DeepEqual(contract, old[index]) {
				continue
			}
			want = append(want, executableInputContract{contract.Name, contract.Version, contract.SchemaDigest})
		}
		sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
		sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
		if len(want) != count || !reflect.DeepEqual(got, want) {
			t.Fatalf("corpus %s differ from selected Definitions: got %+v, want %+v", label, got, want)
		}
	}
	checkContracts("Interfaces", manifest.Interfaces, selected.Interfaces, previousInterfaces, 3)
	checkContracts("Bindings", manifest.Bindings, selected.Bindings, previousBindings, 2)
	if manifest.Bundle.Path != "bundle.mjs" {
		t.Fatalf("corpus bundle path = %q, want bundle.mjs", manifest.Bundle.Path)
	}
	bundle, err := os.ReadFile(filepath.Join(root, manifest.Bundle.Path))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(bundle)); got != manifest.Bundle.SHA256 {
		t.Fatalf("corpus bundle SHA-256 = %s, want %s", got, manifest.Bundle.SHA256)
	}
	if manifest.Cases.Path != "cases.json" {
		t.Fatalf("corpus cases path = %q, want cases.json", manifest.Cases.Path)
	}
	cases, err := os.ReadFile(filepath.Join(root, manifest.Cases.Path))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(cases)); got != manifest.Cases.SHA256 {
		t.Fatalf("corpus cases SHA-256 = %s, want %s", got, manifest.Cases.SHA256)
	}
	var casesDocument struct {
		Status string `json:"status"`
		Cases  []struct {
			ID string `json:"id"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(cases, &casesDocument); err != nil {
		t.Fatal(err)
	}
	if casesDocument.Status != "not-run-without-consumer-adapter" || len(casesDocument.Cases) != 6 {
		t.Fatalf("corpus cases report execution or wrong count: %q/%d", casesDocument.Status, len(casesDocument.Cases))
	}
	seenCases := make(map[string]bool, len(casesDocument.Cases))
	for _, scenario := range casesDocument.Cases {
		if scenario.ID == "" || seenCases[scenario.ID] {
			t.Fatalf("empty or duplicate corpus case %q", scenario.ID)
		}
		seenCases[scenario.ID] = true
	}
}
