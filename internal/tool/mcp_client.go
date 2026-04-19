package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// MCPClient 是 MCP (Model Context Protocol) 客户端
// 用于连接外部 MCP Server 并调用其工具
type MCPClient struct {
	mu       sync.RWMutex
	servers  map[string]*MCPServerConfig // 已连接的 MCP Server
	client   *http.Client
}

// MCPServerConfig MCP Server 配置
type MCPServerConfig struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	APIKey   string `json:"api_key,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// MCPToolInfo MCP Server 返回的工具信息
type MCPToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// MCPRequest MCP JSON-RPC 请求
type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int    `json:"id"`
}

// MCPResponse MCP JSON-RPC 响应
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// MCPError MCP 错误
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// NewMCPClient 创建 MCP 客户端
func NewMCPClient() *MCPClient {
	return &MCPClient{
		servers: make(map[string]*MCPServerConfig),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AddServer 添加 MCP Server
func (c *MCPClient) AddServer(cfg MCPServerConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg.Enabled = true
	c.servers[cfg.Name] = &cfg
}

// RemoveServer 移除 MCP Server
func (c *MCPClient) RemoveServer(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.servers, name)
}

// ListServers 列出所有 MCP Server
func (c *MCPClient) ListServers() []MCPServerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var servers []MCPServerConfig
	for _, s := range c.servers {
		servers = append(servers, *s)
	}
	return servers
}

// ListTools 列出 MCP Server 的工具
func (c *MCPClient) ListTools(serverName string) ([]MCPToolInfo, error) {
	c.mu.RLock()
	server, ok := c.servers[serverName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverName)
	}
	if !server.Enabled {
		return nil, fmt.Errorf("MCP server disabled: %s", serverName)
	}

	// 发送 tools/list 请求
	resp, err := c.sendRequest(server, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list tools from %s: %w", serverName, err)
	}

	var tools []MCPToolInfo
	if err := json.Unmarshal(resp.Result, &tools); err != nil {
		// 尝试解析为 {tools: [...]} 格式
		var wrapper struct {
			Tools []MCPToolInfo `json:"tools"`
		}
		if err2 := json.Unmarshal(resp.Result, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parse tools response: %w", err)
		}
		tools = wrapper.Tools
	}

	return tools, nil
}

// CallTool 调用 MCP Server 的工具
func (c *MCPClient) CallTool(serverName, toolName string, args map[string]any) (string, error) {
	c.mu.RLock()
	server, ok := c.servers[serverName]
	c.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP server not found: %s", serverName)
	}
	if !server.Enabled {
		return "", fmt.Errorf("MCP server disabled: %s", serverName)
	}

	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}

	resp, err := c.sendRequest(server, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("call tool %s on %s: %w", toolName, serverName, err)
	}

	// 解析结果
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		// 尝试直接作为文本返回
		return string(resp.Result), nil
	}

	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return "", fmt.Errorf("MCP tool error: %s", errMsg)
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	return stringsJoin(texts, "\n"), nil
}

// RegisterMCPTools 将 MCP Server 的工具注册到 Registry
func RegisterMCPTools(r *Registry, client *MCPClient) {
	servers := client.ListServers()
	for _, server := range servers {
		tools, err := client.ListTools(server.Name)
		if err != nil {
			continue // 跳过无法连接的
		}

		for _, t := range tools {
			// 将 MCP 参数转换为 tool.Param
			params := convertMCPParams(t.Parameters)

			tool := &Tool{
				Name:        fmt.Sprintf("mcp_%s_%s", server.Name, t.Name),
				Description: t.Description,
				Parameters:  params,
				Category:    CatMCP,
				Source:      server.Name,
				Permission:  PermApprove, // MCP 工具默认需要审批
				Enabled:     true,
				Handler: func(serverName, toolName string) func(map[string]any) (string, error) {
					return func(args map[string]any) (string, error) {
						return client.CallTool(serverName, toolName, args)
					}
				}(server.Name, t.Name),
			}
			r.Register(tool)
		}
	}
}

// sendRequest 发送 JSON-RPC 请求
func (c *MCPClient) sendRequest(server *MCPServerConfig, method string, params any) (*MCPResponse, error) {
	reqBody := MCPRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", server.URL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if server.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+server.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, mcpResp.Error
	}

	return &mcpResp, nil
}

// convertMCPParams 将 MCP 参数格式转换为 tool.Param
func convertMCPParams(mcpParams map[string]any) map[string]Param {
	params := make(map[string]Param)

	props, ok := mcpParams["properties"].(map[string]any)
	if !ok {
		return params
	}

	requiredMap := make(map[string]bool)
	if req, ok := mcpParams["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				requiredMap[s] = true
			}
		}
	}

	for name, prop := range props {
		p, ok := prop.(map[string]any)
		if !ok {
			continue
		}

		param := Param{
			Required: requiredMap[name],
		}

		if t, ok := p["type"].(string); ok {
			param.Type = t
		}
		if d, ok := p["description"].(string); ok {
			param.Description = d
		}
		if def, ok := p["default"]; ok {
			param.Default = def
		}

		params[name] = param
	}

	return params
}

func stringsJoin(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}
