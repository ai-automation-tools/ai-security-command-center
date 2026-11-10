// Sample permits. Nothing here is allowed unless a policy says so: default deny is Cedar's own rule.
// Every policy needs an @id; @reason is the plain-English line ascc shows with the decision.

// With the hook on matcher "*", Claude Code's own planning and question tools reach the PDP too.
// Without this permit, default deny would stall every session.
@id("allow-internal-tools")
@reason("Claude Code's own planning and question tools touch nothing outside the session")
permit (principal is Agent, action == Action::"runtime.tool_use", resource in ToolGroup::"internal");

@id("allow-repo-reads")
@reason("any registered agent may read inside its repo")
permit (principal is Agent, action == Action::"runtime.tool_use", resource in ToolGroup::"read-only")
when { context.path_in_repo };

// A group permit: adding a developer agent is a registry entry, not a new policy.
@id("allow-repo-writes-for-developers")
@reason("agents in the developers group may write inside their repo")
permit (principal in AgentGroup::"developers", action == Action::"runtime.tool_use", resource in ToolGroup::"write")
when { context.path_in_repo };

@id("allow-shell-low-risk")
@reason("agents at risk tier 2 or lower may run shell commands")
permit (principal is Agent, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { principal.risk_tier <= 2 };
