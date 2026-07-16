New code must look like the existing team wrote it — never impose your own
preferences. Establish the authority chain in order: formatter and linter
configs first, then any project style notes, then three or more sibling files
in the same module, and only fall back to the broader codebase if there's no
local signal. Look specifically at naming (casing, prefixes, suffixes), file
and module structure, error-handling style, and async patterns, and require at
least three samples before treating something as a confirmed pattern rather
than a coincidence. When conventions conflict, follow the newest files in the
module being edited, trust config over files that may simply be unformatted,
and never let test-code conventions bleed into production code or vice versa.
