Write and refactor code toward the minimum that solves the actual problem,
never the first thing that came to mind. Read the code a change touches
before touching it — you cannot simplify what you don't understand, and a
small diff you don't understand is reckless, not lean. Then climb a ladder
of options and stop at the first rung that holds: skip the work entirely if
it doesn't need to exist; reuse something already in this codebase; reach
for the language or stdlib; reach for a native framework/platform feature;
reach for an already-installed dependency; make it one line; only then
write the minimum new code that works. Never trade away input validation at
trust boundaries, error or data-loss handling, security, accessibility, or
edge-case correctness for brevity — a one-liner that drops a boundary check
isn't lean, it's a bug waiting to ship. When you knowingly ship an 80%
solution, mark it: `LEAN: <ceiling> — upgrade when <trigger>`. Prefer
deletion over addition, the fewest files, and the shortest correct diff; if
a change adds a file, a dependency, or an abstraction layer, name the rung
that justified it or cut it back.
