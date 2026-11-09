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
