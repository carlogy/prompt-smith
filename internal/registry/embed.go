package registry

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed data
var embedded embed.FS

// Load parses the registry embedded in this binary. It is the production
// entry point; LoadFS does the actual parsing and is what tests exercise
// against synthetic filesystems.
func Load() (*Registry, error) {
	sub, err := fs.Sub(embedded, "data")
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}
	return LoadFS(sub)
}
