package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/plugin"
)

// ===== v0.15.0: Plugin API 端点 =====

// handlePlugins 插件列表
// GET /api/v1/plugins
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	reg, _, _, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 过滤参数
	pluginType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	var plugins []*plugin.PluginEntry
	if pluginType != "" {
		plugins = reg.ListByType(pluginType)
	} else if status != "" {
		plugins = reg.ListByStatus(plugin.PluginStatus(status))
	} else {
		plugins = reg.List()
	}

	type pluginJSON struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Type        string `json:"type"`
		Author      string `json:"author"`
		Description string `json:"description"`
		Status      string `json:"status"`
		InstallPath string `json:"install_path,omitempty"`
	}

	result := make([]pluginJSON, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, pluginJSON{
			Name:        p.Manifest.Name,
			Version:     p.Manifest.Version,
			Type:        p.Manifest.Type,
			Author:      p.Manifest.Author,
			Description: p.Manifest.Description,
			Status:      string(p.Status),
			InstallPath: p.Manifest.InstallPath,
		})
	}

	s.sendJSON(w, http.StatusOK, map[string]any{
		"plugins": result,
		"count":   len(result),
	})
}

// handlePluginGet 获取单个插件详情
// GET /api/v1/plugins/{name}
func (s *Server) handlePluginGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	name := extractPathSuffix(r.URL.Path, "/api/v1/plugins/")
	if name == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "plugin name required"})
		return
	}

	reg, _, sandbox, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	entry, ok := reg.Get(name)
	if !ok {
		s.sendJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
		return
	}

	m := entry.Manifest
	perms, _ := sandbox.GetPermissions(name)

	type depJSON struct {
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
	}

	deps := make([]depJSON, 0, len(m.Dependencies))
	for _, d := range m.Dependencies {
		deps = append(deps, depJSON{Name: d.Name, Version: d.Version})
	}

	permStrs := make([]string, 0, len(perms))
	for _, p := range perms {
		permStrs = append(permStrs, string(p))
	}

	s.sendJSON(w, http.StatusOK, map[string]any{
		"name":         m.Name,
		"version":      m.Version,
		"type":         m.Type,
		"author":       m.Author,
		"description":  m.Description,
		"license":      m.License,
		"homepage":     m.Homepage,
		"repository":   m.Repository,
		"entry":        m.Entry,
		"min_version":  m.MinVersion,
		"tags":         m.Tags,
		"dependencies": deps,
		"permissions":  permStrs,
		"status":       string(entry.Status),
		"install_path": m.InstallPath,
		"error":        entry.Error,
	})
}

// handlePluginInstall 安装插件
// POST /api/v1/plugins/install
func (s *Server) handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Path string `json:"path"` // 本地路径
		URL  string `json:"url"`  // 远程 URL（暂不支持）
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Path == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	reg, inst, sandbox, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	result, err := inst.Install(req.Path)
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 注册沙箱
	entry, _ := reg.Get(result.Name)
	if entry != nil {
		sandbox.RegisterPlugin(result.Name, entry.Manifest)
	}

	s.sendJSON(w, http.StatusOK, map[string]any{
		"name":       result.Name,
		"version":    result.Version,
		"installed":  result.Installed,
		"updated":    result.Updated,
		"installDir": result.InstallDir,
	})
}

// handlePluginUninstall 卸载插件
// DELETE /api/v1/plugins/{name}
func (s *Server) handlePluginUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	name := extractPathSuffix(r.URL.Path, "/api/v1/plugins/")
	if name == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "plugin name required"})
		return
	}

	_, inst, _, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := inst.Uninstall(name); err != nil {
		s.sendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]string{"status": "uninstalled", "name": name})
}

// handlePluginSearch 搜索插件
// GET /api/v1/plugins/search?q=xxx
func (s *Server) handlePluginSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'q' is required"})
		return
	}

	reg, _, _, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 搜索本地已安装的插件
	plugins := reg.List()
	type pluginJSON struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Author      string `json:"author"`
	}

	var results []pluginJSON
	lowerQuery := strings.ToLower(query)
	for _, p := range plugins {
		if strings.Contains(strings.ToLower(p.Manifest.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(p.Manifest.Description), lowerQuery) ||
			strings.Contains(strings.ToLower(p.Manifest.Author), lowerQuery) {
			results = append(results, pluginJSON{
				Name:        p.Manifest.Name,
				Version:     p.Manifest.Version,
				Type:        p.Manifest.Type,
				Description: p.Manifest.Description,
				Author:      p.Manifest.Author,
			})
		}
	}

	s.sendJSON(w, http.StatusOK, map[string]any{
		"query":  query,
		"results": results,
		"count":   len(results),
	})
}

// handlePluginToggle 启用/禁用插件
// POST /api/v1/plugins/{name}/enable
// POST /api/v1/plugins/{name}/disable
func (s *Server) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	path := r.URL.Path
	enable := strings.HasSuffix(path, "/enable")
	disable := strings.HasSuffix(path, "/disable")

	if !enable && !disable {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action, use /enable or /disable"})
		return
	}

	// 提取插件名
	var name string
	if enable {
		name = strings.TrimSuffix(extractPathSuffix(path, "/api/v1/plugins/"), "/enable")
	} else {
		name = strings.TrimSuffix(extractPathSuffix(path, "/api/v1/plugins/"), "/disable")
	}

	if name == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "plugin name required"})
		return
	}

	reg, _, _, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var status plugin.PluginStatus
	var action string
	if enable {
		status = plugin.StatusInstalled
		action = "enabled"
	} else {
		status = plugin.StatusDisabled
		action = "disabled"
	}

	if err := reg.UpdateStatus(name, status); err != nil {
		s.sendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]string{
		"name":   name,
		"status": action,
	})
}

// handlePluginPermissions 管理插件权限
// GET /api/v1/plugins/{name}/permissions
// POST /api/v1/plugins/{name}/permissions (grant/revoke)
func (s *Server) handlePluginPermissions(w http.ResponseWriter, r *http.Request) {
	_, _, sandbox, err := s.getPluginDeps()
	if err != nil {
		s.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 提取插件名
	name := extractPathSuffix(r.URL.Path, "/api/v1/plugins/")
	name = strings.TrimSuffix(name, "/permissions")
	name = strings.TrimSuffix(name, "/perms")

	if name == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "plugin name required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		perms, err := sandbox.GetPermissions(name)
		if err != nil {
			s.sendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		permStrs := make([]string, 0, len(perms))
		for _, p := range perms {
			permStrs = append(permStrs, string(p))
		}
		s.sendJSON(w, http.StatusOK, map[string]any{
			"name":        name,
			"permissions": permStrs,
		})

	case http.MethodPost:
		var req struct {
			Action     string `json:"action"` // "grant" or "revoke"
			Permission string `json:"permission"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		perm := plugin.Permission(req.Permission)
		var actionErr error
		if req.Action == "grant" {
			actionErr = sandbox.GrantPermission(name, perm)
		} else if req.Action == "revoke" {
			actionErr = sandbox.RevokePermission(name, perm)
		} else {
			s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "action must be 'grant' or 'revoke'"})
			return
		}

		if actionErr != nil {
			s.sendJSON(w, http.StatusNotFound, map[string]string{"error": actionErr.Error()})
			return
		}

		s.sendJSON(w, http.StatusOK, map[string]string{
			"name":       name,
			"action":     req.Action,
			"permission": req.Permission,
		})

	default:
		s.sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// getPluginDeps 获取插件系统依赖
func (s *Server) getPluginDeps() (*plugin.Registry, *plugin.Installer, *plugin.Sandbox, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, nil, nil, err
	}

	pluginsDir := filepath.Join(mgr.HomeDir(), "plugins")
	reg := plugin.NewRegistry(pluginsDir)
	inst := plugin.NewInstaller(reg, pluginsDir)
	sandbox := plugin.NewDefaultSandbox()

	if err := reg.LoadFromDisk(); err != nil {
		// 非致命错误，继续
		fmt.Fprintf(os.Stderr, "⚠️  加载插件警告: %v\n", err)
	}

	for _, entry := range reg.List() {
		sandbox.RegisterPlugin(entry.Manifest.Name, entry.Manifest)
	}

	return reg, inst, sandbox, nil
}

// extractPathSuffix 从路径中提取后缀
func extractPathSuffix(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}