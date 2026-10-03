// Command actor-workflow-candidate prints one unpublished Actor+Workflow
// source closure. It writes no catalog, package, release, or Host-support state.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: actor-workflow-candidate")
		os.Exit(2)
	}
	candidate, err := edgeformcatalog.RenderActorWorkflowCandidate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "actor-workflow-candidate:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		fmt.Fprintln(os.Stderr, "actor-workflow-candidate:", err)
		os.Exit(1)
	}
}
