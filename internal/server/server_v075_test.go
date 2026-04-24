package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandlePluginInstall 测试插件安装 handler
func TestHandlePluginInstall(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 创建临时插件目录
	pluginDir := t.TempDir()

	// 创建 plugin.json
	pluginJSON := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "Test plugin"
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0644); err != nil {
		t.Fatalf("Failed to create plugin.json: %v", err)
	}

	// 测试请求
	reqBody := map[string]string{"path": pluginDir}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/install", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handlePluginInstall(w, req)

	// 验证响应（可能成功或失败，但不应 panic）
	t.Logf("Response: %d - %s", w.Code, w.Body.String())
}

// TestHandlePluginInstallInvalidRequest 测试无效请求
func TestHandlePluginInstallInvalidRequest(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试空 body
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/install", strings.NewReader(""))
	w := httptest.NewRecorder()

	srv.handlePluginInstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid body, got %d", w.Code)
	}

	// 测试缺少 path
	reqBody := map[string]string{}
	body, _ := json.Marshal(reqBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/plugins/install", bytes.NewReader(body))
	w = httptest.NewRecorder()

	srv.handlePluginInstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing path, got %d", w.Code)
	}
}

// TestHandlePluginInstallWrongMethod 测试错误 HTTP 方法
func TestHandlePluginInstallWrongMethod(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/install", nil)
	w := httptest.NewRecorder()

	srv.handlePluginInstall(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET, got %d", w.Code)
	}
}

// TestHandlePluginUninstall 测试插件卸载 handler
func TestHandlePluginUninstall(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试不存在的插件
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plugins/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.handlePluginUninstall(w, req)

	// 应该返回 404 或 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 404 or 500 for non-existent plugin, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlePluginUninstallWrongMethod 测试错误 HTTP 方法
func TestHandlePluginUninstallWrongMethod(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/test", nil)
	w := httptest.NewRecorder()

	srv.handlePluginUninstall(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET, got %d", w.Code)
	}
}

// TestHandlePluginUninstallMissingName 测试缺少插件名
func TestHandlePluginUninstallMissingName(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plugins/", nil)
	w := httptest.NewRecorder()

	srv.handlePluginUninstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing plugin name, got %d", w.Code)
	}
}

// TestHandlePluginToggle 测试插件启用 handler
func TestHandlePluginToggle(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试启用不存在的插件
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/test/enable", nil)
	w := httptest.NewRecorder()

	srv.handlePluginToggle(w, req)

	// 应该返回 404 或 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 404 or 500 for non-existent plugin, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlePluginToggleDisable 测试插件禁用 handler
func TestHandlePluginToggleDisable(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试禁用不存在的插件
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/test/disable", nil)
	w := httptest.NewRecorder()

	srv.handlePluginToggle(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 404 or 500 for non-existent plugin, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlePluginToggleInvalidActionV075 测试无效操作
func TestHandlePluginToggleInvalidActionV075(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试无效路径（既不是 enable 也不是 disable）
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/test/invalid", nil)
	w := httptest.NewRecorder()

	srv.handlePluginToggle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid action, got %d", w.Code)
	}
}

// TestHandlePluginToggleWrongMethod 测试错误 HTTP 方法
func TestHandlePluginToggleWrongMethod(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/test/enable", nil)
	w := httptest.NewRecorder()

	srv.handlePluginToggle(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET, got %d", w.Code)
	}
}

// TestHandlePluginSearch 测试插件搜索 handler
func TestHandlePluginSearch(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	// 测试搜索
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/search?q=test", nil)
	w := httptest.NewRecorder()

	srv.handlePluginSearch(w, req)

	// 应该返回 200（即使没有结果）
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for search, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlePluginSearchMissingQuery 测试缺少查询参数
func TestHandlePluginSearchMissingQuery(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/search", nil)
	w := httptest.NewRecorder()

	srv.handlePluginSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing query, got %d", w.Code)
	}
}

// TestHandlePluginSearchWrongMethod 测试错误 HTTP 方法
func TestHandlePluginSearchWrongMethod(t *testing.T) {
	a := createTestAgent(t)
	srv := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/search?q=test", nil)
	w := httptest.NewRecorder()

	srv.handlePluginSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST, got %d", w.Code)
	}
}
