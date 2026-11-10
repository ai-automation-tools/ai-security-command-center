// Sample forbids. A forbid beats every permit.
// There is no "deny a suspended agent" policy: ascc denies a suspended or retired agent before any
// policy runs, so the kill switch can't be undone by deleting a rule.

@id("deny-network-with-secret-path")
@reason("a shell command may not send a secret file over the network")
forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { context.network && context.touches_secret_path };

@id("deny-push-to-main")
@reason("agents may not push to main; open a pull request")
forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { context has git_push_target && context.git_push_target == "main" };
