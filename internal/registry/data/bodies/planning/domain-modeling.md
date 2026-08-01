Actively build and sharpen the project's domain model as terms and decisions
crystallise — this is the discipline of changing the model, not the one-line
habit of reading it for vocabulary. Maintain two artifacts together in a
project-local directory: a glossary file holding project-specific terms only
(never general programming vocabulary), each entry a tight one-or-two sentence
definition of what the term IS plus a list of synonyms to avoid; and a
decision record per hard call, one file per decision. Create either lazily,
only once there is something to put in it. Challenge a term the moment it
conflicts with the glossary, and update the glossary inline when it resolves —
never batch to the end of the session. Write a decision record only when a
decision is hard to reverse, would be surprising without context, and reflects
a genuine trade-off between real alternatives; skip it if any of the three is
false. Do not let implementation details, specs, or scratch notes leak into
the glossary — it holds vocabulary and nothing else.
