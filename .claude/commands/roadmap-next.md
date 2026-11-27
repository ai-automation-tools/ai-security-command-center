---
description: Build the next open roadmap item to the "done" definition in AGENTS.md
argument-hint: "[item name to build instead of the first open one]"
---

Work one item from `docs/ROADMAP.md`: `$ARGUMENTS`, or the first `- [ ]` item that isn't `🔒 Needs owner`
(up to six `small` ones may be batched; a feature is one per run).

1. **Read** the item, the spec criteria it names in `docs/product/spec.md`, and its slice in
   `docs/product/build-plan.md`. If it needs anything on the Won't list in `AGENTS.md`, or contradicts an ADR,
   stop and say so.
2. **Trace it** with the `ascc-threat-trace` skill: threat, control, decision, proof, residual.
3. **Branch.** Never work on `main`. Use the current branch if it isn't `main`; the scheduled routine uses
   `roadmap/auto-<date>`.
4. **Mark it `- [-]`**, build it, and write the tests first for every fail-closed path. Use the `ascc-cedar-policy`
   and `ascc-hook-feature` skills where they apply.
5. **Verify** with `/checks`, then run the item's own text against a fresh `go build` and confirm each claim.
6. **Review** with `/invariant-audit` and fix what it finds.
7. **Close it in one commit:** flip the box to `- [x]` with a dated note of what shipped and what differs from
   the item's text, add the `docs/CHANGELOG.md` line tagged with the phase in italics, and fix every doc the
   change made untrue (`README.md`, `docs/`, `AGENTS.md`).

Stop after the commit and report: what was built, the checks' results, what the review found, and what the
change still doesn't cover. Push and open the PR into `main` only when asked.
