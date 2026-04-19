package tool

import (
	"encoding/json"
	"testing"
)

func TestMCPClientAddRemoveServer(t *testing.T) {
	client := NewMCPClient()

	client.AddServer(MCPServerConfig{
		Name: "test-server",
		URL:  "http://localhost:8080",
	})

	servers := client.ListServers()
	if len(servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(servers))
	}

	client.RemoveServer("test-server")
	servers = client.ListServers()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers after remove, got %d", len(servers))
	}
}

func TestMCPClientListToolsNotFound(t *testing.T) {
	client := NewMCPClient()

	_, err := client.ListTools("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestMCPClientCallToolNotFound(t *testing.T) {
	client := NewMCPClient()

	_, err := client.CallTool("nonexistent", "tool", nil)
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestMCPClientCallToolDisabled(t *testing.T) {
	client := NewMCPClient()
	client.AddServer(MCPServerConfig{
		Name:    "disabled-server",
		URL:     "http://localhost:8080",
		Enabled: true,
	})

	// Disable server
	cfg := MCPServerConfig{
		Name:    "disabled-server",
		URL:     "http://localhost:8080",
		Enabled: false,
	}
	client.AddServer(cfg)

	// Override: AddServer sets Enabled=true, so manually set
	client.servers["disabled-server"].Enabled = false

	_, err := client.CallTool("disabled-server", "tool", nil)
	if err == nil {
		t.Error("expected error for disabled server")
	}
}

func TestMCPRequestFormat(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed MCPRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", parsed.JSONRPC)
	}
	if parsed.Method != "tools/list" {
		t.Errorf("expected method tools/list, got %s", parsed.Method)
	}
}

func TestMCPResponseParsing(t *testing.T) {
	respJSON := `{"jsonrpc":"2.0","result":{"tools":[{"name":"search","description":"Search"}]},"id":1}`
	var resp MCPResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0")
	}
}

func TestConvertMCPParams(t *testing.T) {
	mcpParams := map[string]any{
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
			"count": map[string]any{
				"type":        "number",
				"description": "Result count",
				"default":     5,
			},
		},
		"required": []any{"query"},
	}

	params := convertMCPParams(mcpParams)
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}
	if !params["query"].Required {
		t.Error("query should be required")
	}
	if params["count"].Required {
		t.Error("count should not be required")
	}
	if params["count"].Default != 5 {
		t.Errorf("expected default 5, got %v", params["count"].Default)
	}
}

func TestRegisterMCPTools(t *testing.T) {
	r := NewRegistry()
	client := NewMCPClient()

	// 注册空 MCP（无 server 连接）
	RegisterMCPTools(r, client)

	// 没有 server，不应该有工具
	mcpTools := r.ListByCategory(CatMCP)
	if len(mcpTools) != 0 {
		t.Errorf("expected 0 MCP tools without servers, got %d", len(mcpTools))
	}
}
