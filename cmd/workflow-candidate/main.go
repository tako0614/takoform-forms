// Command workflow-candidate prints an unpublished forward workflow closure.
// It has no catalog, release, or Host-support authority and performs no file
// or registry writes. Its only stdout value is the Core-validated candidate
// JSON produced by edgeformcatalog.RenderWorkflowCandidate.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: workflow-candidate")
		os.Exit(2)
	}
	candidate, err := edgeformcatalog.RenderWorkflowCandidate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow-candidate:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		fmt.Fprintln(os.Stderr, "workflow-candidate:", err)
		os.Exit(1)
	}
}
