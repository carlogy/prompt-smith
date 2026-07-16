Never call a task done without verification, and run the check after every
meaningful change rather than saving it for the end. Discover the project's
actual tooling first (a documented config takes priority, otherwise infer
build/typecheck, lint, and test commands from the ecosystem) and run them in
order: build or typecheck to catch structural errors, lint for style and
correctness warnings, then the relevant test suite. On failure, fix and
restart from the top of the loop, but stop after two failed attempts and ask
for guidance rather than retrying the same fix a third time. Scope the test
run to the affected area when a change is small and localized, but run the
full suite whenever a public interface changed or the blast radius is
unclear, and never suppress or ignore a warning instead of surfacing it.
