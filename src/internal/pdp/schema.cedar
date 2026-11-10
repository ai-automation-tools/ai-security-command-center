// The Cedar schema every registry, policy and request is checked against. It is compiled into ascc:
// a policy that reads an attribute this file doesn't declare is refused at load, not skipped at run time.
entity AgentGroup;
entity Agent in [AgentGroup] = { owner: String, runtime: String, risk_tier: Long, status: String };
entity ToolGroup;
entity Tool in [ToolGroup];
action "runtime.tool_use" appliesTo {
  principal: Agent,
  resource: Tool,
  context: { path_in_repo: Bool, network: Bool, touches_secret_path: Bool, git_push_target?: String }
};
