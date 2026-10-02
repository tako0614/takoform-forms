// Command edge-container-endpoint-candidate prints one UNPUBLISHED
// ContainerEndpoint Form-only package candidate and its local lifecycle cases.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tako0614/takoform-forms/internal/edgeformcatalog"
)

func main() {
	candidate, err := edgeformcatalog.RenderEdgeContainerEndpointFormCandidate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(candidate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
