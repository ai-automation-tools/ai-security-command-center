---
description: Build ascc and run every sample request and hook payload through it, as a decision table
argument-hint: "[registry-and-policies folder, default examples]"
allowed-tools: Bash, PowerShell, Read, Glob
---

Show what the current policies decide. Folder: `$ARGUMENTS`, or `examples` when empty.

1. Build: `go build -o bin/ ./src/cmd/ascc`. Stop with the error if it fails.
2. From that folder, run `ascc decide --request <file>` for every file in `requests/`.
3. For every file in `hook-payloads/`, pipe it to `ascc hook claude-code --agent dev-agent --audit <temp file>`.
   Put the audit file in a temp folder outside the repo and delete it afterwards. Empty output with exit 0 is
   an allow; the deny JSON with exit 0 is a deny; exit 2 means ascc couldn't run.
4. Then `ascc audit verify --audit <that temp file>` to confirm the chain those calls wrote.

Report one table: input file, agent, tool, decision, determining policy ids, reason, exit code. Under it, list
anything surprising: a sample that should deny and allows, a deny with no policy id that isn't a default deny,
a non-zero exit, or a policy in `policies/` that no sample exercises (every rule needs an allow fixture and a
deny fixture).

Change nothing in the repo.
