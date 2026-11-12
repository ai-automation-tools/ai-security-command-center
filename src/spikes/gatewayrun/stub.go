package gatewayrun

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/types/known/structpb"

	api "github.com/ai-automation-tools/ai-security-command-center/src/spikes/gatewayrun/extmcp"
)

// Stub is the ext_mcp policy service agentgateway calls out to. Its rules are a stand-in for ASCC's PDP,
// small enough to read in one go, and like the PDP it denies whatever it can't place:
//
//   - no agent id in the metadata, or an id it doesn't know, is a deny;
//   - a suspended agent is denied everything, on the request and on the response;
//   - nobody may call delete_repo;
//   - agent-intern isn't shown delete_repo in tools/list;
//   - a tools/list whose read_file description differs from the pinned one is rejected whole.
type Stub struct {
	api.UnimplementedExtMcpServer

	mu        sync.Mutex
	suspended map[string]bool

	// Requests and Responses count the callouts received, to prove the gateway really asked.
	Requests, Responses atomic.Int64
}

// NewStub returns a stub with nobody suspended.
func NewStub() *Stub { return &Stub{suspended: map[string]bool{}} }

// Suspend is the kill switch: every later callout for the agent is a deny.
func (s *Stub) Suspend(agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suspended[agent] = true
}

var knownAgents = map[string]bool{"agent-dev": true, "agent-intern": true, "agent-stream": true}

// pinned is the description each tool was approved with. A tool with no pin isn't checked.
var pinned = map[string]string{"read_file": readFileDescription}

// agentDenied returns the reason an agent may do nothing at all, or "".
func (s *Stub) agentDenied(agent string) string {
	if !knownAgents[agent] {
		return "ascc: unknown agent, denied by default"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.suspended[agent] {
		return "ascc: agent " + agent + " is suspended"
	}
	return ""
}

func (s *Stub) CheckRequest(_ context.Context, req *api.McpRequest) (*api.McpRequestResult, error) {
	s.Requests.Add(1)
	deny := func(reason string) (*api.McpRequestResult, error) {
		return &api.McpRequestResult{Result: &api.McpRequestResult_Error{Error: denied(reason)}}, nil
	}
	if reason := s.agentDenied(agentOf(req.GetMetadataContext())); reason != "" {
		return deny(reason)
	}
	if req.GetMethod() == "tools/call" {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.GetMcpRequest(), &p); err != nil || p.Name == "" {
			return deny("ascc: tools/call with no readable tool name")
		}
		if p.Name == "delete_repo" {
			return deny("ascc: forbid-delete-repo: no agent may call delete_repo")
		}
	}
	return &api.McpRequestResult{Result: &api.McpRequestResult_Pass{Pass: &api.Pass{}}}, nil
}

func (s *Stub) CheckResponse(_ context.Context, resp *api.McpResponse) (*api.McpResponseResult, error) {
	s.Responses.Add(1)
	deny := func(reason string) (*api.McpResponseResult, error) {
		return &api.McpResponseResult{Result: &api.McpResponseResult_Error{Error: denied(reason)}}, nil
	}
	agent := agentOf(resp.GetMetadataContext())
	if reason := s.agentDenied(agent); reason != "" {
		return deny(reason)
	}
	if resp.GetMethod() != "tools/list" {
		return &api.McpResponseResult{Result: &api.McpResponseResult_Pass{Pass: &api.Pass{}}}, nil
	}
	// Keep every field of the result and of each tool that this code doesn't know about.
	var result map[string]json.RawMessage
	var tools []map[string]json.RawMessage
	if err := json.Unmarshal(resp.GetMcpResponse(), &result); err != nil {
		return deny("ascc: tools/list result isn't JSON")
	}
	if err := json.Unmarshal(result["tools"], &tools); err != nil {
		return deny("ascc: tools/list result has no readable tools")
	}
	kept := make([]map[string]json.RawMessage, 0, len(tools))
	for _, tool := range tools {
		var name, description string
		_ = json.Unmarshal(tool["name"], &name)
		_ = json.Unmarshal(tool["description"], &description)
		if want, ok := pinned[name]; ok && description != want {
			return deny("ascc: tool " + name + " no longer matches its pinned description")
		}
		if name == "delete_repo" && agent == "agent-intern" {
			continue
		}
		kept = append(kept, tool)
	}
	if len(kept) == len(tools) {
		return &api.McpResponseResult{Result: &api.McpResponseResult_Pass{Pass: &api.Pass{}}}, nil
	}
	result["tools"], _ = json.Marshal(kept)
	mutated, _ := json.Marshal(result)
	return &api.McpResponseResult{Result: &api.McpResponseResult_Mutated{Mutated: mutated}}, nil
}

func denied(reason string) *api.AuthorizationError {
	return &api.AuthorizationError{Code: api.AuthorizationError_PERMISSION_DENIED, Reason: reason}
}

// agentOf reads the agent id the gateway's config put in the callout's metadata. The response callout
// carries no headers, so metadata is the only place both callouts can find it.
func agentOf(metadata *structpb.Struct) string {
	return metadata.GetFields()["agent"].GetStringValue()
}
