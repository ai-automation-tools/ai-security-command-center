<h1 align="center">📋 ASCC Changelog</h1>

<p align="center">
  <em>What changed in AI Security Command Center, newest first.</em>
</p>

<p align="center">
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/plan-ROADMAP-8B5CF6?style=for-the-badge" alt="Roadmap"></a>
  <a href="README.md"><img src="https://img.shields.io/badge/↩-docs-6B7280?style=for-the-badge" alt="Docs home"></a>
</p>

---

Entries that close a roadmap item carry the roadmap phase in italics.

## Unreleased

**Docs**
- Requirements review against the stated purpose (enterprise agent management, and governing non-human accounts including AI agents). The public-MVP spec gains a fourth feature, an offline fleet and NHI inventory (`posture`, `discover`, declared and imported identities), plus agent groups, a `retired` status, a computed coverage label and criteria 9 to 17. The build plan and roadmap don't carry these yet.
- The same review found three holes in the MVP as specified and closed them in the spec: an agent could edit the registry or run `ascc revive` on itself, parallel hooks could fork the audit chain, and lines cut from the end of the log still verified.
- Planning package v0.3: epic EP-24 (agent fleet management), FR-REG-12 to 15, FR-POL-11, FR-RT-05 to 07, FR-KS-08, FR-CON-15 to 18, threat T-28, abuse cases ABUSE-33 and 34, tickets TKT-041 and 042.
- `AGENTS.md` carries the two reports' rules: cedar-go v1.9.0 and the Go 1.26 bump it brings, the fail-closed handling of erroring policies, and the hook contract. *Slice 0: Contracts*
- Claude Code hook contract (`docs/reports/2026-claude-code-hook-contract.md`): the PreToolUse payload, the deny JSON and exit codes, the fail-open defaults and `"onFailure": "block"` (v2.1.295+), the coverage gaps (`@` mentions, Monitor, `PowerShell`, editable settings), and seven rules for Slice 2. *Slice 0: Contracts*
- cedar-go check (`docs/reports/2026-cedar-go-check.md`): cedar-go v1.9.0 matched the Rust reference engine on 9 of 9 requests and on schema validation, at about 1.3 µs per decision. It needs Go 1.26, and in both engines a `forbid` that errors on a missing attribute fails open, so Slice 1 now carries four fail-closed rules. *Slice 0: Contracts*

## 2026-11-09

**Added**
- Repo scaffold: a Go module with the `ascc` CLI (`src/cmd/ascc`) that prints its version, with `gofmt`, `go vet`, `go test` and `go build` all passing, an MIT licence, and an `.env.example` that says there are no settings.
- CI on push to `main` and on every PR, on Ubuntu and Windows: format, vet, test and build. *Public release*
- `SECURITY.md` with private reporting through GitHub, and what counts as a vulnerability for a tool whose worst bug is a wrong allow. *Public release*
- Demo shell in `site/`: the org source bar, a build-stamp meta tag, a "coming soon" body, and the app icon (a violet hexagon with a check on the shared dark tile) as its favicon. *Public release*

**Docs**
- Root README, `AGENTS.md`, the docs hub, this changelog, and a roadmap that puts Slices 0 to 3 of the build plan above the release line.
- The combined planning package (A to K) copied from the private planning workspace into `docs/product/plan/` with lowercase file names, and the resource library into `docs/resources/`. The two source runs, the idea analysis and the market notes stay private.
- A public-MVP spec and build plan cut from the plan's MVP: one Go binary, one runtime, local files.
- A full-history secret scan came back clean. *Public release*

---

<p align="center">
  <a href="README.md">← Docs home</a> ·
  <a href="ROADMAP.md">Roadmap →</a>
</p>
