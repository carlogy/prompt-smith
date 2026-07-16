package tui

import "github.com/carlogy/prompt-smith/internal/prompt"

// Action is what the user chose to do with the finished prompt.
type Action int

const (
	ActionCancel Action = iota
	ActionStdout
	ActionCopy
	ActionWrite
)

// Result is what Run returns: the finalized inputs plus what to do with
// the assembled prompt. Run never performs the action itself (no file
// writes, no clipboard) - the caller (internal/cli) does, reusing the
// same delivery logic as the non-interactive path.
type Result struct {
	Inputs    prompt.Inputs
	Action    Action
	WritePath string // set when Action == ActionWrite
}
