---
description: Run the going-public checks - secrets in history, local paths, personal data, fictional-only fixtures, honest claims
allowed-tools: Bash, PowerShell, Read, Grep, Glob
---

This repo goes public at graduation. Report whether it is safe to; change nothing and publish nothing. Making
the repo public is the owner's call and is never part of this command.

Check each and report pass or fail with the file and line of every hit:

1. **Secrets in history:** `gitleaks git . --log-opts=--all`. Any finding is a fail, even one removed in a
   later commit.
2. **Secrets in the tree:** tracked files matching `.env*` other than `.env.example`; anything that looks like
   a key, token or credential in code, fixtures, docs or workflow files.
3. **Local paths:** drive-letter paths, home directories and user names in tracked files (search for
   `[A-Za-z]:\\`, `/Users/`, `/home/` other than the fictional `/home/dev`, and `C:/`).
4. **Personal data:** real names, email addresses, hostnames or account ids in tracked files and in commit
   author fields (`git log --format='%an <%ae>' | sort -u`). Authors must use a noreply address.
5. **Fictional fixtures only:** every host in `examples/`, `site/` and the tests is `example.com`,
   `paste.example` or another reserved name; every agent, team and repo is invented.
6. **Honest claims:** every command and flag in `README.md` runs as written against a fresh
   `go build -o bin/ ./src/cmd/ascc`; the "won't do" list and the guide's limits table match the code; nothing
   says `enforced` where the hook can only observe.
7. **Release gates:** list every unticked item above `## After public release` in `docs/ROADMAP.md`.
8. **CI:** `.github/workflows/` keeps `permissions: contents: read` unless a job needs more, and pins nothing
   to a secret this repo doesn't need.

End with one line: **ready** or **not ready**, and the blocking items in the order to fix them.

$ARGUMENTS
