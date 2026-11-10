---
description: Review the current diff against ASCC's invariants, ADRs and threat model with the security-reviewer agent
argument-hint: "[path, package or git range; default: the diff against main]"
---

Launch the `security-reviewer` agent on: `$ARGUMENTS`, or the diff against `main` plus uncommitted changes when
empty.

Give it the target and nothing else: no summary of what the change is meant to do and no view on whether it is
safe. It reads `AGENTS.md` and the diff itself.

When it returns, relay the verdict and every finding with its file and line. For each Critical or High finding,
check the reported input yourself before repeating it as fact. Don't fix anything until asked; if asked, each
fix lands with the test row that would have caught it.
