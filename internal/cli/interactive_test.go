package cli

import (
	"testing"
)

func TestDecideUseTUI(t *testing.T) {
	cases := []struct {
		name        string
		interactive bool
		quick       bool
		forceTUI    bool
		numSkills   int
		want        bool
		wantErr     bool
	}{
		{"non-tty, bare -> skip", false, false, false, 0, false, false},
		{"tty, quick, bare -> skip (quick wins)", true, true, false, 0, false, false},
		{"tty, bare -> TUI", true, false, false, 0, true, false},
		{"tty, skills given, no force -> skip", true, false, false, 2, false, false},
		{"tty, skills given, forced -> TUI (pre-selected)", true, false, true, 2, true, false},
		{"quick + tui together -> error", true, true, true, 0, false, true},
		{"tui on non-tty -> error", false, false, true, 0, false, true},
		{"quick+tui error takes priority over the tty error", false, true, true, 0, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decideUseTUI(tc.interactive, tc.quick, tc.forceTUI, tc.numSkills)
			if tc.wantErr {
				if err == nil {
					t.Fatal("decideUseTUI() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decideUseTUI() error = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("decideUseTUI() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecideUseTUI_ErrorMessages(t *testing.T) {
	_, err := decideUseTUI(true, true, true, 0)
	if err == nil {
		t.Fatal("expected an error for --quick + --tui")
	}
}
