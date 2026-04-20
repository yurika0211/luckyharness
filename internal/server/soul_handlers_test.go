package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer 创建测试用 HTTP 服务器
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	a := createTestAgent(t)
	cfg := ServerConfig{
		Addr:       ":0",
		EnableCORS: true,
		RateLimit:  60,
	}
	s := New(a, cfg)

	// 使用 httptest.Server 包装
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/soul/templates", s.handleSoulTemplates)
	mux.HandleFunc("/api/v1/soul/templates/", s.handleSoulTemplateByID)

	return httptest.NewServer(mux)
}

// TestSoulTemplatesList 测试列出 SOUL 模板
func TestSoulTemplatesList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/soul/templates")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	templates, ok := result["templates"].([]interface{})
	if !ok {
		t.Fatal("templates field missing or wrong type")
	}
	if len(templates) == 0 {
		t.Error("expected at least one template")
	}
}

// TestSoulTemplatesListByLanguage 测试按语言列出模板
func TestSoulTemplatesListByLanguage(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/soul/templates?language=zh")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	templates, ok := result["templates"].([]interface{})
	if !ok {
		t.Fatal("templates field missing or wrong type")
	}
	if len(templates) == 0 {
		t.Error("expected at least one Chinese template")
	}
}

// TestSoulTemplateGetByID 测试获取指定模板
func TestSoulTemplateGetByID(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/soul/templates/default")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["id"] != "default" {
		t.Errorf("expected id 'default', got %v", result["id"])
	}
}

// TestSoulTemplateGetByIDNotFound 测试获取不存在的模板
func TestSoulTemplateGetByIDNotFound(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/soul/templates/nonexistent")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestSoulTemplateAdd 测试添加自定义模板
func TestSoulTemplateAdd(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	body := `{"id":"test-custom","name":"Custom","language":"en","content":"You are a custom assistant."}`
	resp, err := http.Post(srv.URL+"/api/v1/soul/templates", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	// 验证可以获取
	resp2, err := http.Get(srv.URL + "/api/v1/soul/templates/test-custom")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp2.StatusCode)
	}
}

// TestSoulTemplateAddNoID 测试添加无 ID 模板
func TestSoulTemplateAddNoID(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	body := `{"name":"No ID","language":"en","content":"You are a test."}`
	resp, err := http.Post(srv.URL+"/api/v1/soul/templates", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestSoulTemplateDelete 测试删除自定义模板
func TestSoulTemplateDelete(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// 先添加
	body := `{"id":"to-delete","name":"To Delete","language":"en","content":"You will be deleted."}`
	resp, err := http.Post(srv.URL+"/api/v1/soul/templates", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	resp.Body.Close()

	// 删除
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/soul/templates/to-delete", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestSoulTemplateDeleteBuiltin 测试删除内置模板（应失败）
func TestSoulTemplateDeleteBuiltin(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/soul/templates/default", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// TestSoulTemplateRender 测试模板渲染
func TestSoulTemplateRender(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// 添加带变量的模板
	body := `{"id":"render-test","name":"Render Test","language":"en","content":"Hello, {{name}}! You are {{role}}.","variables":{"name":"User","role":"assistant"}}`
	resp, err := http.Post(srv.URL+"/api/v1/soul/templates", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	resp.Body.Close()

	// 获取并渲染
	resp2, err := http.Get(srv.URL + "/api/v1/soul/templates/render-test?var_name=Alice&var_role=coder")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp2.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	rendered, ok := result["rendered"].(string)
	if !ok {
		t.Fatal("rendered field missing or wrong type")
	}
	if rendered != "Hello, Alice! You are coder." {
		t.Errorf("unexpected rendered content: %q", rendered)
	}
}

// TestSoulTemplatesMethodNotAllowed 测试不支持的方法
func TestSoulTemplatesMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/soul/templates", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}