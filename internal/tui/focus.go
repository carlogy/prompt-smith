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
	focusTarget
)

// focusCycle is the canonical Tab order. focusTarget sits immediately
// before focusSkills (i.e. right after focusPreview on the wrap) rather
// than at the front, so that the default starting zone (focusSkills,
// the zero value) is unaffected and every existing "N tabs from skills
// to <zone>" distance - notably "6 tabs to preview" - stays correct.
var focusCycle = []focusZone{
	focusTarget, focusSkills, focusGoal, focusContext, focusConstraints,
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
