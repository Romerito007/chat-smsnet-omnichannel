// Command openapigen writes the committed OpenAPI document to docs/openapi.yaml
// from the single source of truth in presenter/openapi. Run it after changing
// any route or DTO: `go run ./cmd/openapigen`. The OpenAPITest sync test fails
// if docs/openapi.yaml is stale.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/romerito007/chat-smsnet-omnichannel/presenter/openapi"
)

func main() {
	out := "docs/openapi.yaml"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	data, err := openapi.YAML()
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes)\n", out, len(data))
}
