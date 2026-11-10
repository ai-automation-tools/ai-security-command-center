// Sample forbids. A forbid beats every permit.
// There is no "deny a suspended agent" policy: ascc denies a suspended or retired agent before any
// policy runs, so the kill switch can't be undone by deleting a rule.

@id("deny-network-with-secret-path")
@reason("a shell command may not send a secret file over the network")
forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { context.network && context.touches_secret_path };

@id("deny-push-to-main")
@reason("agents may not push to main or to every branch at once; open a pull request")
forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { context has git_push_target && (context.git_push_target == "main" || context.git_push_target == "*") };

// An agent must not switch its own governor off: no writing the registry, a policy, the audit log or
// a Claude Code settings file, and no running `ascc kill`, `ascc revive` or `ascc hook`. Command parsing can be
// dodged, so also keep those files where the agent's user can't write them.
@id("deny-ascc-state-changes")
@reason("an agent may not change ascc's registry, policies, audit log or hook settings")
forbid (principal, action == Action::"runtime.tool_use", resource)
when { context.touches_ascc_state };
