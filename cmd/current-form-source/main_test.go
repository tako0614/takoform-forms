package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderSourceEmitsOnlyTheCurrentEdgeFamily(t *testing.T) {
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	wantGroups := []string{"edge.forms.takoform.com"}
	wantCounts := []int{17}
	if len(document.Families) != len(wantGroups) {
		t.Fatalf("families = %d, want %d", len(document.Families), len(wantGroups))
	}
	for index, family := range document.Families {
		if family.Group != wantGroups[index] {
			t.Fatalf("family[%d] = %q, want %q", index, family.Group, wantGroups[index])
		}
		var forms []struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(family.Forms, &forms); err != nil {
			t.Fatalf("decode %s Forms: %v", family.Group, err)
		}
		if len(forms) != wantCounts[index] {
			t.Fatalf("%s Forms = %d, want %d", family.Group, len(forms), wantCounts[index])
		}
		foundObjectBucket := false
		for _, form := range forms {
			if form.Kind == "ObjectBucket" {
				foundObjectBucket = true
				break
			}
		}
		if !foundObjectBucket {
			t.Fatal("current aggregate omits ObjectBucket")
		}
	}
}

func TestRenderSourceBuildsGlobalProviderNeutralContracts(t *testing.T) {
	document, err := renderSource()
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Interfaces) != 8 {
		t.Fatalf("interfaces = %d, want 8", len(document.Interfaces))
	}
	if len(document.Bindings) != 7 {
		t.Fatalf("bindings = %d, want 7", len(document.Bindings))
	}
	seen := map[string]struct{}{}
	foundObjectInterface := false
	foundObjectBinding := false
	for _, contract := range append(append([]sourceContract(nil), document.Interfaces...), document.Bindings...) {
		if _, duplicate := seen[contract.Name]; duplicate {
			t.Fatalf("duplicate global contract %q", contract.Name)
		}
		seen[contract.Name] = struct{}{}
		if contract.SchemaDigest == "" || contract.DefinitionJSON == "" {
			t.Fatalf("contract %q has no exact identity", contract.Name)
		}
		if contract.Name == "edge.objects" {
			foundObjectInterface = true
		}
		if strings.Contains(contract.Name, "object-bucket") {
			foundObjectBinding = true
		}
	}
	if !foundObjectInterface || !foundObjectBinding {
		t.Fatalf("current aggregate omits ObjectBucket contracts: interface=%v binding=%v", foundObjectInterface, foundObjectBinding)
	}
}

func TestSortExactContractsAllowsVersionsAndRejectsExactDuplicate(t *testing.T) {
	t.Parallel()
	contracts := []sourceContract{
		{Name: "example.contract", Version: "2.0.0"},
		{Name: "example.contract", Version: "1.0.0"},
		{Name: "another.contract", Version: "1.0.0"},
	}
	if err := sortExactContracts(contracts); err != nil {
		t.Fatal(err)
	}
	if contracts[0].Name != "another.contract" || contracts[1].Version != "1.0.0" || contracts[2].Version != "2.0.0" {
		t.Fatalf("exact contracts are not deterministically sorted: %#v", contracts)
	}
	contracts = append(contracts, sourceContract{Name: "example.contract", Version: "2.0.0"})
	if err := sortExactContracts(contracts); err == nil || !strings.Contains(err.Error(), "example.contract@2.0.0") {
		t.Fatalf("exact duplicate error = %v", err)
	}
}
