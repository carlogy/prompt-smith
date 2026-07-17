//go:build !empty

package registry

import (
	"embed"
	"io/fs"
)

// embeddedRaw is the registry data compiled into this binary: the
// canonical skill set by default. Built with `-tags empty` (see
// embed_empty.go), it's an empty scaffold instead - same categories and
// targets, no skills - for users who only want their own, via
// PROMPTSMITH_SKILLS_DIR (see userskills.go).
//
//go:embed data
var embeddedRaw embed.FS

// embeddedData returns the embedded registry's root as an fs.FS, ready
// for LoadFS. Load calls this; it's the one symbol embed_default.go and
// embed_empty.go must each provide, so Load itself never needs to know
// which build tag is active.
func embeddedData() (fs.FS, error) {
	return fs.Sub(embeddedRaw, "data")
}
