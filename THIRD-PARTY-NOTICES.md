# Third-party notices

promptsmith is licensed under AGPL-3.0-or-later (see [LICENSE](LICENSE)).
That license governs promptsmith's own source. The web UI (`--ui`) also
embeds two small, unmodified third-party JavaScript files, vendored
into the repo rather than loaded from a CDN so the built binary stays a
single, self-contained, offline-capable artifact (see the rationale at
`internal/server/templates.go` and `internal/server/assets/tailwind/input.css`).
Both are permissively licensed and redistributed here, byte-for-byte,
under the terms below.

**License identifier, stated plainly:** both projects below are marked
BSD-2-Clause in some third-party summaries, but their actual upstream
`LICENSE` files (verified directly, see sources below) and their npm
package metadata (`"license"` field) agree: the real license is
**0BSD** (SPDX: `0BSD`, "Zero-Clause BSD" / "Free Public License
1.0.0"). 0BSD is the BSD-2-Clause text with both of its conditions
removed - including the one requiring the license or copyright notice
to be reproduced on redistribution. Its entire operative text is:
"Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted." There is no
notice-retention or attribution obligation anywhere in it. So this
file is not closing a compliance gap: htmx.min.js and
idiomorph-ext.min.js could be redistributed, with or without this file
existing, and promptsmith would owe their authors nothing under the
license's own terms.

This file exists anyway, and prior to it neither license's text was
present anywhere in this repo - only promptsmith's own AGPL LICENSE
was. The reasons to keep it are about provenance and supply-chain
hygiene, not legal obligation:

- Both files are vendored, unmodified third-party binaries checked
  directly into the repo, with no build-time fetch step that would
  otherwise pull them (and their version metadata) from a package
  registry. The repo is the *only* record of what these two files
  are.
- This file pins the exact upstream version and a sha256 of the
  vendored bytes for each, with byte-for-byte verification against
  the published upstream release recorded alongside. That is what
  makes a future upgrade, or a CVE response against htmx or
  idiomorph, auditable: anyone can confirm exactly what is currently
  vendored, and exactly what changed when it's replaced.
  `idiomorph-ext.min.js` in particular carries no embedded version
  string at all - without this file, its version is unrecoverable
  from the artifact itself (see the provenance-limitation note under
  its entry below).
- It is the conventional place a license auditor or a downstream
  packager looks first. Having it costs nothing and answers "what's
  in this binary, and under what terms?" before the question is
  asked.
- Reproducing each license's text below is a courtesy and a complete
  audit trail, done deliberately even though, per the paragraph
  above, 0BSD does not require it.

This is the single place that record lives - not a per-file `.LICENSE`
sidecar, and not a comment embedded in the minified JS itself, because
a comment can't reliably survive re-minification or a future
re-vendor.

---

## htmx.min.js

- **Version:** 2.0.10 (confirmed from the vendored file's own contents:
  it contains the literal string `version:"2.0.10"`)
- **Upstream source:** https://unpkg.com/htmx.org@2.0.10/dist/htmx.min.js
- **License text source:** https://raw.githubusercontent.com/bigskysoftware/htmx/v2.0.10/LICENSE
- **SPDX identifier:** 0BSD
- **Vendored:** 2026-07-17 (commit `f06751b`)
- **sha256 (vendored file):** `71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de`
- **Upstream verification:** fetched the upstream file at the URL
  above and compared its sha256 against the vendored copy - they are
  **byte-for-byte identical** (same sha256 as above). The vendored
  copy is confirmed to be exactly the published 2.0.10 release
  artifact, unmodified.

### License text (verbatim, from the URL above)

```
Zero-Clause BSD
=============

Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL
WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES
OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE
FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY
DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN
AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT
OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

(This is the license's complete, unmodified text; upstream's own
`LICENSE` file, at the vendored version's tag, carries no separate
copyright-holder line to reproduce.)

---

## idiomorph-ext.min.js

- **Version:** 0.7.4 (the `-ext` build, which wires idiomorph into
  htmx). **Provenance limitation:** unlike htmx.min.js, this file
  contains no embedded version string at all, so the version cannot be
  confirmed from the vendored bytes themselves. The only record of
  which version was vendored is the commit message of `3fc9109`,
  which vendored it as idiomorph 0.7.4. Treat that commit message,
  not the file, as the version's source of truth.
- **Upstream source:** https://cdn.jsdelivr.net/npm/idiomorph@0.7.4/dist/idiomorph-ext.min.js
- **License text source:** https://raw.githubusercontent.com/bigskysoftware/idiomorph/v0.7.4/LICENSE
- **SPDX identifier:** 0BSD
- **Vendored:** 2026-07-30 (commit `3fc9109`)
- **sha256 (vendored file):** `a6437e55b1b6a07bc421f0d230266a39399b6826c6ed19e0ed9c63b707444a5f`
- **Upstream verification:** fetched the upstream file at the URL
  above and compared its sha256 against the vendored copy - they are
  **byte-for-byte identical** (same sha256 as above). The vendored
  copy is confirmed to be exactly the published 0.7.4 release
  artifact, unmodified.

### License text (verbatim, from the URL above)

```
Zero-Clause BSD
=============

Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL
WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES
OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE
FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY
DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN
AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT
OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

(Same note as above: upstream's `LICENSE` file carries no separate
copyright-holder line to reproduce.)

---

## app.css is NOT third-party

`internal/server/assets/static/app.css` is **first-party** output: it
is Tailwind CSS *compiled* from promptsmith's own
`internal/server/assets/tailwind/input.css` by `make ui-css` (see the
comment there and the target at `Makefile`). It is generated output,
not a vendored artifact - unlike htmx.min.js and idiomorph-ext.min.js
above, no third-party bytes are checked into the repo here. Tailwind
itself is a build-time dependency used to produce app.css; its own
code is never shipped inside promptsmith's binary. **Do not add a
third-party attribution entry for app.css to this file** - that would
misrepresent build output as vendored code. If that ever needs to
change (e.g. bundling a font, an icon set, or any other
non-Tailwind-generated asset into app.css), give it its own entry
above, following the same format.

## Re-vendoring obligation

If either `htmx.min.js` or `idiomorph-ext.min.js` is replaced or
upgraded, **update this file in the same commit**: new version, new
vendored date, new sha256, and re-verify (or re-fetch) the license
text if the upstream project has changed it. This file is the only
provenance record either vendored file has - the version pinned above
is not recoverable from the artifact itself for `idiomorph-ext.min.js`,
which embeds no version string at all. An out-of-date entry here
silently breaks that record; don't treat this file as a one-time
exercise.
