// Command vector-index-candidate prints an unpublished development candidate.
// It has no catalog, release, or Host-support authority and performs no file
// or registry writes. The sole stdout value is
// {form, interface, binding, workerVersion, workerDeployment}, each with exact
// Core-validated rendered bytes and digest-bound references where applicable.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: vector-index-candidate")
		os.Exit(2)
	}
	candidate, err := edgeformcatalog.RenderVectorIndexCandidate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vector-index-candidate:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		fmt.Fprintln(os.Stderr, "vector-index-candidate:", err)
		os.Exit(1)
	}
}
