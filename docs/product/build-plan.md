# Build plan: AI Security Command Center

> Slices for the public MVP in [spec.md](spec.md). The full delivery plan, staffing and estimates for the
> platform are in [plan/h-roadmap.md](plan/h-roadmap.md) and [plan/j-backlog.md](plan/j-backlog.md).

**MVP = Slices 0 to 3.** The repo goes public when Slice 3 is done and the release gates in
[ROADMAP.md](../ROADMAP.md) are ticked. Everything after is built in public.

| Slice | What | Plan reference | MVP |
|:-:|:---|:---|:-:|
| 0 | Scaffold, CI, demo shell (shipped with the repo); cedar-go check and Claude Code hook contract, written up | TKT-002, TKT-004 | ✅ |
| 1 | Registry, Cedar PDP, `decide`, `inventory` | M1.1 (PDP only), ADR-03 | ✅ |
| 2 | Claude Code hook, hash-chained audit, kill switch | M1.3 (adapter only), M1.5 (audit, kill), ADR-10, ADR-13 | ✅ |
| 3 | WebAssembly browser demo | — | ✅ |
| 4 | Remaining Phase 0 spikes and ADR acceptance | TKT-001, TKT-003, TKT-031 | |
| 5 | Schemas, Compose skeleton, STS and the PDP as a service | TKT-005, TKT-006, M1.1 | |
| 6 | MCP and LLM gateway | M1.2, ADR-04 | |
| 7 | stdio MCP wrapper, default-deny egress, credential broker | M1.3, ADR-07, ADR-08 | |
| 8 | Plugin host and the OpenBao connector | M1.4, ADR-15, ADR-16 | |
| 9 | GitHub connector, console, per-PEP kill confirmation | M1.5 | |
| 10 | Plan MVP exit: demo script cold run, MVP-1 to MVP-12 evidence | M1.6 | |

## Why this cut

The plan's MVP needs a Compose stack and owner decisions before the first useful line of code. The cut keeps the
core loop, decide then enforce then record then stop, on the one runtime that already has a blocking hook, so a
weekly run can build and test it offline. Slices 4 onward are the plan's Phase 1, unchanged.

## Stop rules

- **cedar-go can't express the policy set** (Slice 0 report): switch to the ADR-03 fallback before Slice 1 and
  file it as an ADR amendment.
- **The Claude Code hook can't block a tool the MVP needs** (Slice 0 report): list the gap in the README's
  coverage section and mark that tool `observed`, not `enforced`. Never claim enforcement the hook can't give.
- **Slice 1 or 2 needs more than 6 runs:** stop and write up what made it big before building more.

---

<p align="center">
  <a href="spec.md">← Spec</a> ·
  <a href="README.md">Product</a>
</p>
