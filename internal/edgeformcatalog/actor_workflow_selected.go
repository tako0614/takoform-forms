package edgeformcatalog

import "fmt"

// ActorWorkflowSelectedSource is the publisher's unpublished source selection.
// It does not modify the retained catalog, generated packages, or release set.
type ActorWorkflowSelectedSource struct {
	Forms      []RenderedForm
	Interfaces []RenderedContract
	Bindings   []RenderedContract
}

// RenderActorWorkflowSelectedSource replaces only the nine dependent Forms,
// three Interfaces and two Bindings in the current Edge source. The other
// eight/five/five rendered definitions are copied byte-for-byte.
func RenderActorWorkflowSelectedSource() (ActorWorkflowSelectedSource, error) {
	baseForms, err := RenderForms()
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	baseInterfaces, err := RenderInterfaces()
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	baseBindings, err := RenderBindings()
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	if len(baseInterfaces) != 8 || len(baseBindings) != 7 {
		return ActorWorkflowSelectedSource{}, fmt.Errorf("current Edge contract roster %d/%d, want 8/7 before joint selection", len(baseInterfaces), len(baseBindings))
	}
	joint, err := renderActorWorkflowSourceCandidate()
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	forms := []RenderedForm{
		joint.ModuleWorker, joint.Actor.Form, joint.Workflow.Form,
		joint.WorkerVersion, joint.WorkerDeployment,
	}
	forms = append(forms, joint.RuntimeDependants...)
	interfaces := []RenderedContract{joint.RuntimeInterface, joint.Actor.Interface, joint.Workflow.Interface}
	bindings := []RenderedContract{joint.Actor.Binding, joint.Workflow.Binding}
	selectedForms, err := replaceSelectedForms(baseForms, forms)
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	selectedInterfaces, err := replaceSelectedContracts(baseInterfaces, interfaces)
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	selectedBindings, err := replaceSelectedContracts(baseBindings, bindings)
	if err != nil {
		return ActorWorkflowSelectedSource{}, err
	}
	return ActorWorkflowSelectedSource{selectedForms, selectedInterfaces, selectedBindings}, nil
}

func replaceSelectedForms(base, replacements []RenderedForm) ([]RenderedForm, error) {
	if len(base) != 17 || len(replacements) != 9 {
		return nil, fmt.Errorf("Actor+Workflow Form source count %d/%d, want 17/9", len(base), len(replacements))
	}
	byKind := make(map[string]RenderedForm, len(replacements))
	for _, form := range replacements {
		if _, duplicate := byKind[form.Kind]; duplicate {
			return nil, fmt.Errorf("duplicate replacement Form %s", form.Kind)
		}
		byKind[form.Kind] = form
	}
	out := append([]RenderedForm(nil), base...)
	used := make(map[string]bool, len(replacements))
	for index, old := range out {
		if replacement, ok := byKind[old.Kind]; ok {
			if used[old.Kind] || replacement.Slug != old.Slug {
				return nil, fmt.Errorf("duplicate or wrong-slug replacement Form %s", old.Kind)
			}
			out[index] = replacement
			used[old.Kind] = true
		}
	}
	if len(used) != len(replacements) {
		return nil, fmt.Errorf("Actor+Workflow Form replacement missed %d current kinds", len(replacements)-len(used))
	}
	return out, nil
}

func replaceSelectedContracts(base, replacements []RenderedContract) ([]RenderedContract, error) {
	byName := make(map[string]RenderedContract, len(replacements))
	for _, contract := range replacements {
		if _, duplicate := byName[contract.Name]; duplicate {
			return nil, fmt.Errorf("duplicate replacement contract %s", contract.Name)
		}
		byName[contract.Name] = contract
	}
	out := append([]RenderedContract(nil), base...)
	used := make(map[string]bool, len(replacements))
	for index, old := range out {
		if replacement, ok := byName[old.Name]; ok {
			if used[old.Name] {
				return nil, fmt.Errorf("duplicate current contract %s", old.Name)
			}
			out[index] = replacement
			used[old.Name] = true
		}
	}
	if len(used) != len(replacements) {
		return nil, fmt.Errorf("Actor+Workflow contract replacement missed %d current names", len(replacements)-len(used))
	}
	return out, nil
}
