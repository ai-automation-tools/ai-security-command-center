# Security Policy

AI Security Command Center decides whether an AI agent may run a tool, and keeps the record of what it decided.
A bug that lets a call through that policy denies, or that lets the record be changed unnoticed, is the worst
kind of bug this project can have, so we take reports seriously.

## Reporting a vulnerability

**Please don't open a public issue for a security vulnerability.**

Report it privately through GitHub:

1. Go to the repository's **Security** tab, then **Report a vulnerability** (GitHub Private Vulnerability
   Reporting).
2. Describe the issue, its impact, and the steps to reproduce it. A proof of concept helps.

We aim to acknowledge a report within a few days and keep you posted while we fix it. Please give us reasonable
time to ship a fix before you disclose it.

## Scope

ASCC runs on your machine, or entirely in your browser tab for the demo. There's no hosted service, no account
and no telemetry. The surfaces that matter most:

- **A wrong allow.** A request that the policy set denies, or that matches no permit, coming back allowed. That
  includes an unknown agent or tool, a suspended agent, and any error path: the hook fails closed, and a code
  path that fails open is a vulnerability.
- **Audit tampering that verifies.** An edited, removed, inserted or reordered audit line that
  `ascc audit verify` doesn't catch.
- **A secret reaching output.** Hook payloads can contain secrets. The audit log stores a hash of the raw tool
  input, never the input. A secret in any log line, output or error message is a bug.
- **Network.** The CLI and the demo make no network calls. A code path that does is in scope.
- **Hostile input.** Hook payloads, policies and registry files are parsed, never evaluated. A malformed or
  oversized file must produce a deny or an error, not a crash that leaks contents or a hang.

### Not vulnerabilities (by design)

- A tool the Claude Code hook doesn't fire for isn't enforced. The README's coverage section lists those, and
  `inventory` labels the agent accordingly.
- A permissive policy you wrote is a policy, not a bypass.
- A missing runtime, connector or feature is a feature request. Open an issue.

## Supported versions

ASCC is pre-1.0. Fixes land on the latest `main`.

## Test fixtures

Every agent, owner, repo and URL in this repo's tests, examples and demo is fictional (`example.com` owners,
`paste.example` targets). Never commit a real hook payload, a token, a `.env` file or a local path. A full-history
secret scan (`gitleaks git . --log-opts=--all`) came back clean on 2026-11-09.
