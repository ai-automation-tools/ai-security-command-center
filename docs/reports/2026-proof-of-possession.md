<h1 align="center">🔑 Proof of possession for agent tokens (TKT-003)</h1>

<p align="center">
  <em>A stolen token shouldn't work from another machine. Which way of binding a token to a key can each client actually use?</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/verdict-DPoP_where_we_write_the_client,_mTLS_for_Claude_Code-2ea44f?style=for-the-badge" alt="Verdict: DPoP where ASCC writes the client, mTLS for Claude Code">
  <img src="https://img.shields.io/badge/checked-2026--11--10-6B7280?style=for-the-badge" alt="Checked 2026-11-10">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

**Verdict: there is no one mechanism, because the clients differ.** ASCC's own clients use DPoP. Claude Code
talking to the LLM gateway uses a client certificate, the only binding it supports. **Claude Code talking to a
remote MCP server can't bind its token at all today,** so that path stays a plain bearer token and is held to
low-risk tools until that changes.

This is a reading of the standards and of the clients' documentation on 2026-11-10. Nothing was built: the
token service is Slice 5. It answers R2 in [E §18](../product/plan/e-architecture.md) and amends
[ADR-06](../product/plan/e2-adrs.md#adr-06).

## The three ways

| Mechanism | Standard | How the token is bound | What it costs |
|:--|:--|:--|:--|
| **DPoP** | [RFC 9449](https://www.rfc-editor.org/rfc/rfc9449.html), 2023 | The token carries the key's thumbprint (`cnf.jkt`). Every request carries a fresh signed proof naming the method, the URL and a hash of the token. | The client must sign every request. A server nonce is needed to stop proofs being made ahead of time. |
| **Certificate-bound tokens** | [RFC 8705](https://www.rfc-editor.org/rfc/rfc8705.html), 2020 | The token carries the certificate's thumbprint (`cnf.x5t#S256`). The TLS handshake proves the key. | Whoever ends the TLS connection must be the one checking, or must pass the certificate on in a way the next hop can trust; the RFC leaves that out of scope. |
| **Signed HTTP requests** | [RFC 9421](https://www.rfc-editor.org/rfc/rfc9421.html), 2024 | A signature over chosen parts of the request. | **No published OAuth profile ties an access token to such a key.** The one draft that did expired in 2016. |

OAuth 2.1 (`draft-ietf-oauth-v2-1-17`) says servers SHOULD sender-constrain access tokens and names DPoP and
mTLS, nothing else. Signed requests are out for now: ASCC would be inventing the binding.

## What each client can do

| Client | Finding | Source |
|:--|:--|:--|
| **Claude Code → API or LLM gateway** | Presents a client certificate from PEM files (`CLAUDE_CODE_CLIENT_CERT`, `CLAUDE_CODE_CLIENT_KEY`). Its other credentials can't sign a request: custom headers are static, and the key helper runs on a timer (five minutes by default) and is told nothing about the request. | [network config](https://code.claude.com/docs/en/network-config), [LLM gateway](https://code.claude.com/docs/en/llm-gateway-connect) |
| **Claude Code → remote MCP server** | OAuth, static headers, or a headers helper that runs once per connection. No DPoP: an open issue reports that none is sent. **Whether the client certificate is used on MCP connections isn't documented.** | [MCP](https://code.claude.com/docs/en/mcp), [issue 96290](https://github.com/anthropics/claude-code/issues/96290), [issue 100914](https://github.com/anthropics/claude-code/issues/100914) |
| **The MCP specification** | Revision 2026-07-28 requires `Authorization: Bearer` and says nothing of DPoP, mTLS or sender-constrained tokens. A DPoP profile ([SEP-1932](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/1932)) has been an open draft since 2025-12. | [authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) |
| **ASCC's own clients** | The stdio MCP wrapper, the plugin host, and the hook adapter and CLI of the gateway era are ours, so they can sign whatever is asked of them. This is not the public MVP's `ascc` binary, which makes no network call and holds no token. | — |

## Recommendation, per client

| Client | Binding | Token claim | Notes |
|:--|:--|:--|:--|
| **ASCC's stdio MCP wrapper, hook, CLI, plugin host** | **DPoP** | `cnf.jkt` | A key per agent instance, made on the machine and kept in the OS key store where there is one. The gateway and token service send a nonce. |
| **Claude Code → LLM gateway** | **Client certificate** | `cnf.x5t#S256` | The gateway ends TLS itself, so nothing has to be trusted to pass the certificate on. A short-lived certificate per agent instance, issued at enrolment. |
| **Claude Code → remote MCP through the gateway** | **None available** | no `cnf` | A bearer token with the ≤ 15 minute life ADR-06 already sets, bound to one audience. See the rule below. |
| **Server agents** | **Workload certificate** | `cnf.x5t#S256` | Unchanged: ADR-06's SVID path. |

**One rule follows, and it needs an owner's yes.** A token with no `cnf` should not be enough for a tier 2 or 3
action. The PDP gets a context value saying whether the request's token was bound, and the default policy
denies tier 2 and 3 without it. In practice a Claude Code agent reaches high-risk MCP tools through the stdio
wrapper, which can do DPoP, and not over a remote connection, which can't. That is a tiering rule, so it goes
with decision K3 and isn't settled here.

## What this doesn't fix

- **A key in a file can be copied.** Claude Code's certificate key is a PEM file the agent's own user can read,
  so an agent that can read files can take it with the token. Binding stops a token stolen in transit or from a
  log; it doesn't stop theft from the machine. Only a key the OS won't export does, and Claude Code has no
  support for one (issue 100914 asks for it).
- **A live attacker on the machine can still sign.** RFC 9449 says so itself: with a key that can't be exported,
  an attacker can still make proofs while the client is online. Proof of possession narrows replay; the kill
  switch and short lifetimes are what end a session.
- **The remote MCP path has no binding** until Claude Code or the MCP specification adds one. Coverage must say
  so: a bearer-only path is not reported as bound.

## For Slice 5

- **Verifying DPoP in Go.** The one dedicated library found,
  [go-dpop](https://github.com/AxisCommunications/go-dpop) (MIT), had its last release in 2024-11. The check is
  small (one signed token, four claims, a thumbprint, a nonce), so the choice is between that library and about
  150 lines on the standard library. Decide it with the token service.
- **Test the undocumented case:** start Claude Code with a client certificate and a remote MCP server that asks
  for one, and see whether it is presented. If it is, the remote MCP row becomes certificate-bound.
- **Watch** SEP-1932 and Claude Code issues 96290 and 100914.

---

<p align="center">
  <a href="2026-gateway-build-vs-adopt.md">← Gateway: build or adopt</a> ·
  <a href="../README.md">Docs home</a> ·
  <a href="2026-plugin-contract-spike.md">Plugin contract spike →</a>
</p>
