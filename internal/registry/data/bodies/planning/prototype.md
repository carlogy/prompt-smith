A prototype is throwaway code that answers ONE question about logic — state
transitions, data shape, business rules that only feel wrong once pushed
through real cases — never a visual question; if the question turns out to
be visual, say so and stop. Write the question down before any code, one
paragraph at the top of the file or its README, so it can be checked later
by someone returning cold. Isolate the logic behind a small pure interface
that could be lifted into the real codebase later — a reducer, an explicit
state machine, or a set of pure functions, whichever shape fits the
question — with no I/O and no terminal code inside it; the shell around it
is the smallest thing that surfaces state after every action via one
command from the project's existing task runner. Skip all polish: no
tests, no persistence beyond memory, no error handling past what makes it
run, no abstractions, no new runtime or package manager. Do not load
`lean-code` for this work — its always-keep guards are exactly what a
prototype drops on purpose. When the question is answered, fold the
validated decision into real code; the prototype's shell is never shipped.
