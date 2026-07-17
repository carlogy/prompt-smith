//go:build empty

package registry

import (
	"embed"
	"io/fs"
)

// embeddedRaw is the "empty" registry scaffold: the canonical
// categories and targets, but no skills - see embed_default.go for the
// normal, default build.
//
//go:embed data-empty
var embeddedRaw embed.FS

// embeddedData returns the embedded registry's root as an fs.FS, ready
// for LoadFS. See embed_default.go for the counterpart this mirrors.
func embeddedData() (fs.FS, error) {
	return fs.Sub(embeddedRaw, "data-empty")
}
