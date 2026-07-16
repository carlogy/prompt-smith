package tui

// focusZone is which region of the TUI currently receives key input:
// the skill picker, one of the editable fields, or the preview.
type focusZone int

const (
	focusSkills focusZone = iota
	focusGoal
	focusContext
	focusConstraints
	focusRole
	focusOutputFormat
	focusPreview
)

// focusCycle is the canonical Tab order.
var focusCycle = []focusZone{
	focusSkills, focusGoal, focusContext, focusConstraints,
	focusRole, focusOutputFormat, focusPreview,
}

// nextFocus/prevFocus advance the cycle with wraparound. An unrecognized
// zone (shouldn't happen) falls back to focusSkills.
func nextFocus(f focusZone) focusZone {
	for i, z := range focusCycle {
		if z == f {
			return focusCycle[(i+1)%len(focusCycle)]
		}
	}
	return focusSkills
}

func prevFocus(f focusZone) focusZone {
	for i, z := range focusCycle {
		if z == f {
			return focusCycle[(i-1+len(focusCycle))%len(focusCycle)]
		}
	}
	return focusSkills
}
