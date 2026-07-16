package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// buildVersion derives a version string from the running binary's
// embedded build info. No build-time ldflags/version-injection needed -
// works with both `go install module@version` and local builds from a
// git checkout.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return formatVersion(info)
}

// formatVersion is buildVersion's pure formatting logic, separated out
// so it's testable against synthetic debug.BuildInfo values instead of
// only whatever this test binary's own build happens to produce.
//
// A real tag (go install module@v1.2.3) or Go's own auto-generated
// pseudo-version (v0.0.0-<timestamp>-<hash>[+dirty]) already embeds
// everything useful in Main.Version - trust it as-is. Only fall back to
// reading raw VCS settings when Main.Version is empty or the generic
// "(devel)" placeholder (Go didn't derive anything useful on its own),
// to avoid reporting a redundant, duplicated revision/dirty suffix on
// top of what Go already embedded.
func formatVersion(info *debug.BuildInfo) string {
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if revision == "" {
		return "(devel)"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified {
		return fmt.Sprintf("(devel) (%s, dirty)", revision)
	}
	return fmt.Sprintf("(devel) (%s)", revision)
}

// newVersionCmd builds the "version" subcommand. Prints in the same
// "promptsmith version X" format as cobra's built-in --version flag
// (enabled via newRootCmd's Version field), so both conventions people
// reach for agree.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the promptsmith version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("promptsmith version %s\n", buildVersion())
		},
	}
}
