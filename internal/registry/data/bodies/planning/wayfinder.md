When an idea is too big for one session and the route to it isn't visible yet,
chart it as a durable map before doing any of the work — plan, don't do. Keep a
map index and a directory of decision tickets together per effort, in a
project-local location. The map is a low-resolution index holding the
destination, one-line decisions already made, fog (real but not yet sharp enough
to ticket), and out-of-scope rulings; give each decision its own ticket file
holding only immutable id, type, question, and — once resolved — answer. Keep
mutable state only in the map's roster (status, blocked-by, claimed-by), never
in a ticket file, and derive the frontier (open, unclaimed, unblocked) from it
each time, never cached. Resolve at most one non-research ticket per session,
claiming it in the roster first; match resolution to type — research delegated,
grilling and logic-prototype tickets resolved live with the user, never answered
on their behalf. After resolving, record the answer, close the ticket, log one
decision line, and advance the frontier before stopping. Do not load every
ticket body when the map exists to avoid that, and do not pre-slice fog into
tickets before a question is sharp enough to state precisely.
