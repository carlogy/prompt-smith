Spend disproportionate effort building a fast, deterministic, pass/fail
feedback loop first — a failing test, a CLI repro, or a captured trace replay;
without one, no amount of code-reading helps. Reproduce the failure and
confirm it matches what was actually reported. Generate three to five ranked,
falsifiable hypotheses before testing any single one, since committing to the
first plausible idea anchors you on the wrong cause. Instrument one variable
at a time against a specific hypothesis rather than logging everything and
grepping. Once a hypothesis is confirmed, write a regression test before
applying the fix, watch it fail, apply the fix, watch it pass, then remove all
temporary instrumentation and confirm the original repro no longer reproduces.
