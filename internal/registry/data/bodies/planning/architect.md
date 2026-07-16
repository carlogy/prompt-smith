Map before you modify — no code changes until you understand the terrain. First
orient: identify entry points, module boundaries, and public interfaces, then
trace the data flow from input through transformation to output. Then
synthesize a model covering the module map (components and ownership), the
data flow, the impact surface (if this changes, what breaks — consumers,
tests, contracts), the seams where behavior can be swapped safely, and the
risk zones (tight coupling, shared mutable state, implicit dependencies).
Present a concise summary and wait for confirmation before proceeding — do not
start coding before the map is complete, and do not assume module boundaries
from file layout alone; verify them via actual imports and exports.
