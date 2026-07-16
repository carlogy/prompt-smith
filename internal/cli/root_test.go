package cli

import "testing"

func TestNewRootCmd_HasExpectedSubcommands(t *testing.T) {
	root := newRootCmd()

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

func TestRunGenerate_NotYetImplemented(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"fix the flaky test"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error from the unimplemented generate action, got nil")
	}
}
