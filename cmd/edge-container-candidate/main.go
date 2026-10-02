// Command edge-container-candidate prints one UNPUBLISHED Form-only local
// ContainerService package closure. It does not print or mint a container.http
// Interface, Worker Binding, Workflow successor, release identity, trust
// statement, catalog entry, or publication request.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: edge-container-candidate")
		os.Exit(2)
	}
	candidate, err := edgeformcatalog.RenderEdgeContainerServiceFormCandidate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "edge-container-candidate:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		fmt.Fprintln(os.Stderr, "edge-container-candidate:", err)
		os.Exit(1)
	}
}
