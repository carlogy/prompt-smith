package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatVersion_UsesMainVersionWhenPresent(t *testing.T) {
	// A real tag (go install module@v1.2.3) or Go's own auto-generated
	// pseudo-version (v0.0.0-<timestamp>-<hash>[+dirty]) already embeds
	// everything useful - trust it as-is rather than appending a
	// second, redundant revision/dirty suffix on top.
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}
	if got := formatVersion(info); got != "v1.2.3" {
		t.Errorf("formatVersion() = %q, want %q", got, "v1.2.3")
	}

	pseudo := &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260716222712-117c5b5923b5+dirty"}}
	if got := formatVersion(pseudo); got != pseudo.Main.Version {
		t.Errorf("formatVersion() = %q, want the pseudo-version unchanged: %q", got, pseudo.Main.Version)
	}
}

func TestFormatVersion_FallsBackToVCSRevisionWhenDevel(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := formatVersion(info)
	if !strings.Contains(got, "abcdef1") {
		t.Errorf("formatVersion() = %q, want it to include the short revision", got)
	}
	if !strings.Contains(got, "dirty") {
		t.Errorf("formatVersion() = %q, want it to flag a dirty tree", got)
	}
}

func TestFormatVersion_PlainDevelWhenNoVCSInfo(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	if got := formatVersion(info); got != "(devel)" {
		t.Errorf("formatVersion() = %q, want %q", got, "(devel)")
	}
}

func TestBuildVersion_ReturnsNonEmptyString(t *testing.T) {
	got := buildVersion()
	if got == "" {
		t.Error(`buildVersion() = "", want a non-empty version string`)
	}
}

func TestVersionFlagAndSubcommand_AgreeAndAreNonEmpty(t *testing.T) {
	run := func(args []string) string {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
		return stdout.String()
	}

	flagOut := run([]string{"--version"})
	subOut := run([]string{"version"})

	if flagOut == "" || subOut == "" {
		t.Fatalf("expected non-empty output, got flag=%q subcommand=%q", flagOut, subOut)
	}
	if !strings.Contains(flagOut, buildVersion()) || !strings.Contains(subOut, buildVersion()) {
		t.Errorf("expected both --version (%q) and the version subcommand (%q) to report the same version (%q)",
			flagOut, subOut, buildVersion())
	}
}
