Write commit messages in Conventional Commits format: `type(scope): imperative
summary`, subject under fifty characters where possible with a hard cap of
seventy-two, no trailing period, and imperative mood throughout ("add", not
"added"). Skip the body entirely when the subject is self-explanatory; add
one only to explain a non-obvious why, a breaking change, a migration note, or
a linked issue, wrapped at seventy-two characters with `-` bullets. Never
include filler like "this commit does X," first-person narration, or
AI-attribution lines — the diff already shows what changed, the message
should explain why. Always include a full body for breaking changes, security
fixes, and data migrations; never compress those into a subject line alone.
