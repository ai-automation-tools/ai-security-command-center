---
name: ascc-threat-trace
description: Tie a change in ASCC to the threat it mitigates, the requirement and ADR behind it and the test that proves it, and record a changed architecture decision as an ADR amendment. Use when designing a slice or feature, when a change alters what ASCC defends against or how, when writing a PR body for a security-relevant change, when a roadmap item seems to conflict with an ADR or the Won't list, and when asked what a control does and doesn't cover.
---

# Threat trace

Every control in ASCC answers a named threat. If you can't name it, you are building something nobody asked
for, or missing the reason it has to fail closed.

## The documents

| Question | Where |
|:---|:---|
| What are we defending, from whom, across which boundary | `docs/product/plan/d-threat-model.md` §1 to §3 |
| The threats, T-01 to T-28, with mitigations and residual risk | same file, §4 (⛔ prevents, 🔍 detects only) |
| Attack scenarios S1 to S12 | same file, §5 |
| Accepted gaps and detection-only controls | same file, §7 and §8 |
| Threat to control to test | same file, §9 |
| Requirements (`FR-*`, `NFR-*`) | `docs/product/plan/c-requirements-catalog.md` |
| Binding architecture decisions, ADR-01 to ADR-17 | `docs/product/plan/e2-adrs.md` |
| Abuse and failure tests (`ABUSE-*`, `FAIL-*`) | `docs/product/plan/i-validation.md` |
| What the public MVP must do, with acceptance criteria | `docs/product/spec.md` |

## The threats the MVP actually touches

| Threat | In the MVP | Honest limit |
|:---|:---|:---|
| **T-04** indirect injection steers a tool call | The hook decides every tool call on policy and parsed features | A permitted tool can still be misused within its constraints |
| **T-07** exfiltration through a tool call | `deny-network-with-secret-path` on shell commands | Judged from command text; no egress control until a later slice |
| **T-11** a person bypasses the hook | `ascc posture claude-code` and the computed coverage label | Reported, not prevented, outside managed settings |
| **T-12** shadow agents and orphaned identities | `inventory` findings, `ascc discover` | Discovery is partial by nature |
| **T-13** audit tampering | Hash chain, `audit verify`, `--expect-head` | Tamper-evident, not tamper-proof; the head must be kept off-host |
| **T-14** policy tampering | Schema-validated load, `deny-ascc-state-changes` | An approved but wrong policy still decides |
| **T-22** model output run as a shell command | The hook checks `Bash` and `PowerShell` | Patterns are bypassable |
| **T-28** the agent attacks its own enforcement point | `touches_ascc_state`, the kill switch outside policy | Where the agent's user can write ascc's files, every local control is advisory |

Everything else (T-01 to T-03, T-05, T-06, T-08 to T-10, T-15 onward) belongs to the gateway, broker, egress
and plugin slices. Don't claim it for the MVP.

## Tracing a change

Write these five lines before the code, and put them in the PR body:

```
Threat:    T-xx, in one sentence, with the adversary
Control:   what the change does, and whether it prevents (⛔) or only detects (🔍)
Decision:  the ADR and requirement it implements (ADR-xx, FR-xx or spec criterion n)
Proof:     the test that fails without it, with its allow and deny fixture
Residual:  what still gets through, and where the docs say so
```

If "Residual" is empty you haven't looked hard enough. If "Proof" is empty the change isn't done.

## When a change disagrees with an ADR

The ADRs are binding. A changed decision is never a quiet code change.

1. Stop building. Say which ADR and what the evidence is (a spike result, a failed test, a measured number).
2. Amend in place in `e2-adrs.md`: add a bullet `- **Amended <yyyy-mm-dd>:** <what changed and why>` inside
   that ADR, and update its row in the status table if the status moved. Don't rewrite the original text.
3. If the amendment changes a threat's mitigation or residual risk, update that row in the threat model in the
   same commit.
4. A change that needs something on the Won't list in `AGENTS.md`, or an owner decision (items K1 to K10 in
   `docs/product/plan/k-next-steps.md`), is flagged on the roadmap as `🔒 Needs owner`, not decided here.

## A new threat

Add it as the next `T-nn` row in §4 with STRIDE class, boundary, severity (impact × likelihood), mitigations
marked ⛔ or 🔍, and residual risk; add its traceability row in §9. A control that only detects goes in §8 too.
