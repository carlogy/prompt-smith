package cli

import "testing"

func TestNewRootCmd_HasExpectedSubcommands(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	want := map[string]bool{"list": false, "validate": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}
