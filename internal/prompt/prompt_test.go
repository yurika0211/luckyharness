package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// PT-1: Parser Tests
// ---------------------------------------------------------------------------

func TestParsePlainText(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("hello world")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeText || nodes[0].text != "hello world" {
		t.Errorf("expected text node 'hello world', got %+v", nodes[0])
	}
}

func TestParseVariable(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("Hello {{name}}!")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].typ != nodeText || nodes[0].text != "Hello " {
		t.Errorf("first node wrong: %+v", nodes[0])
	}
	if nodes[1].typ != nodeVar || nodes[1].text != "name" {
		t.Errorf("second node wrong: %+v", nodes[1])
	}
	if nodes[2].typ != nodeText || nodes[2].text != "!" {
		t.Errorf("third node wrong: %+v", nodes[2])
	}
}

func TestParseDottedVariable(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{user.name}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeVar || nodes[0].text != "user.name" {
		t.Errorf("expected var node 'user.name', got %+v", nodes[0])
	}
}

func TestParseIfBlock(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{#if admin}}Admin!{{/if}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeIf {
		t.Errorf("expected if node, got %d", nodes[0].typ)
	}
	if nodes[0].cond != "admin" {
		t.Errorf("expected cond 'admin', got %s", nodes[0].cond)
	}
	if len(nodes[0].children) != 1 {
		t.Errorf("expected 1 child, got %d", len(nodes[0].children))
	}
}

func TestParseIfElseBlock(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{#if admin}}Admin!{{else}}User!{{/if}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeIf {
		t.Errorf("expected if node, got %d", nodes[0].typ)
	}
	if len(nodes[0].children) != 1 {
		t.Errorf("expected 1 child in if branch, got %d", len(nodes[0].children))
	}
	if len(nodes[0].alt) != 1 {
		t.Errorf("expected 1 child in else branch, got %d", len(nodes[0].alt))
	}
}

func TestParseEachBlock(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{#each items as item}}- {{item}}{{/each}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeEach {
		t.Errorf("expected each node, got %d", nodes[0].typ)
	}
	if nodes[0].name != "items" {
		t.Errorf("expected name 'items', got %s", nodes[0].name)
	}
	if nodes[0].iterVar != "item" {
		t.Errorf("expected iterVar 'item', got %s", nodes[0].iterVar)
	}
}

func TestParsePartial(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{>header}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodePartial || nodes[0].name != "header" {
		t.Errorf("expected partial node 'header', got %+v", nodes[0])
	}
}

func TestParseBlock(t *testing.T) {
	p := NewParser()
	nodes, err := p.Parse("{{#block content}}Hello{{/block}}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].typ != nodeBlock || nodes[0].name != "content" {
		t.Errorf("expected block node 'content', got %+v", nodes[0])
	}
}

// ---------------------------------------------------------------------------
// PT-2: TemplateStore Tests
// ---------------------------------------------------------------------------

func TestTemplateStoreRegisterAndGet(t *testing.T) {
	store := NewTemplateStore()
	tmpl := &Template{Name: "greeting", Content: "Hello {{name}}!"}

	if err := store.Register(tmpl); err != nil {
		t.Fatalf("register error: %v", err)
	}

	got, err := store.Get("greeting")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got.Content != tmpl.Content {
		t.Errorf("expected content %q, got %q", tmpl.Content, got.Content)
	}
}

func TestTemplateStoreGetNotFound(t *testing.T) {
	store := NewTemplateStore()
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestTemplateStoreRegisterNoName(t *testing.T) {
	store := NewTemplateStore()
	tmpl := &Template{Name: "", Content: "test"}
	if err := store.Register(tmpl); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestTemplateStoreDelete(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "temp", Content: "test"})
	store.Delete("temp")
	_, err := store.Get("temp")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestTemplateStoreList(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "a", Content: "a"})
	store.Register(&Template{Name: "b", Content: "b"})

	names := store.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestTemplateStoreLoadFromDir(t *testing.T) {
	tmpDir := t.TempDir()
	content := "Hello {{name}}!"
	if err := os.WriteFile(filepath.Join(tmpDir, "greet.tmpl"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewTemplateStore()
	if err := store.LoadFromDir(tmpDir); err != nil {
		t.Fatalf("load dir: %v", err)
	}

	got, err := store.Get("greet")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got.Content != content {
		t.Errorf("expected %q, got %q", content, got.Content)
	}
}

// ---------------------------------------------------------------------------
// PT-3: Render Engine Tests
// ---------------------------------------------------------------------------

func TestRenderVariable(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "Hello {{name}}!"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"name": "World"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestRenderDottedVariable(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{user.name}} is {{user.age}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{
		"user": map[string]interface{}{"name": "Alice", "age": 30},
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Alice is 30" {
		t.Errorf("expected 'Alice is 30', got %q", result)
	}
}

func TestRenderIfTrue(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#if admin}}Admin!{{/if}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"admin": true})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Admin!" {
		t.Errorf("expected 'Admin!', got %q", result)
	}
}

func TestRenderIfFalse(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#if admin}}Admin!{{/if}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"admin": false})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestRenderIfElse(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#if admin}}Admin!{{else}}User!{{/if}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"admin": false})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "User!" {
		t.Errorf("expected 'User!', got %q", result)
	}
}

func TestRenderIfEquality(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#if role==admin}}Admin!{{else}}Not admin{{/if}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"role": "admin"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Admin!" {
		t.Errorf("expected 'Admin!', got %q", result)
	}
}

func TestRenderIfNegation(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#if !admin}}Not admin{{/if}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"admin": false})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Not admin" {
		t.Errorf("expected 'Not admin', got %q", result)
	}
}

func TestRenderEach(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#each items as item}}[{{item}}]{{/each}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{
		"items": []interface{}{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "[a][b][c]" {
		t.Errorf("expected '[a][b][c]', got %q", result)
	}
}

func TestRenderEachWithIndex(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{#each items as item, idx}}{{idx}}:{{item}} {{/each}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{
		"items": []interface{}{"x", "y"},
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "0:x") || !strings.Contains(result, "1:y") {
		t.Errorf("expected index:value pairs, got %q", result)
	}
}

func TestRenderPartial(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "header", Content: "HEADER: {{title}}"})
	store.Register(&Template{Name: "page", Content: "{{>header}}\nBody content"})
	engine := NewEngine(store)

	result, err := engine.Render("page", RenderData{"title": "Test"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "HEADER: Test") {
		t.Errorf("expected partial rendered, got %q", result)
	}
	if !strings.Contains(result, "Body content") {
		t.Errorf("expected body content, got %q", result)
	}
}

func TestRenderHelperUpper(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{upper name}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"name": "hello"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", result)
	}
}

func TestRenderHelperLower(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{lower name}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"name": "HELLO"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestRenderHelperTruncate(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{truncate text 5}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"text": "Hello World"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Hello..." {
		t.Errorf("expected 'Hello...', got %q", result)
	}
}

func TestRenderHelperDefault(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{default name fallback}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{"fallback": "default_value"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "default_value" {
		t.Errorf("expected 'default_value', got %q", result)
	}
}

func TestRenderHelperJoin(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{join items}}"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{
		"items": []interface{}{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "a, b, c" {
		t.Errorf("expected 'a, b, c', got %q", result)
	}
}

func TestRenderContent(t *testing.T) {
	engine := NewEngine(NewTemplateStore())

	result, err := engine.RenderContent("Hello {{name}}!", RenderData{"name": "World"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestRenderWithLayout(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "layout", Content: "<html>{{body}}</html>"})
	store.Register(&Template{Name: "page", Content: "<p>Content</p>", Layout: "layout"})
	engine := NewEngine(store)

	result, err := engine.RenderWithLayout("page", RenderData{})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "<html><p>Content</p></html>" {
		t.Errorf("expected layout wrapping, got %q", result)
	}
}

func TestRenderWithLayoutNoLayout(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "page", Content: "Hello {{name}}!"})
	engine := NewEngine(store)

	result, err := engine.RenderWithLayout("page", RenderData{"name": "World"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestRenderMissingVariable(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "Hello {{name}}!"})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "Hello !" {
		t.Errorf("expected 'Hello !', got %q", result)
	}
}

func TestValidate(t *testing.T) {
	engine := NewEngine(NewTemplateStore())

	if err := engine.Validate("Hello {{name}}!"); err != nil {
		t.Errorf("valid template should pass: %v", err)
	}
}

func TestCustomHelper(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: "{{repeat name 3}}"})
	engine := NewEngine(store)
	engine.RegisterHelper("repeat", func(args []string, data RenderData) string {
		s := resolveVarStr(args[0], data)
		return s + s + s
	})

	result, err := engine.Render("test", RenderData{"name": "ha"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result != "hahaha" {
		t.Errorf("expected 'hahaha', got %q", result)
	}
}

func TestRenderComplexTemplate(t *testing.T) {
	store := NewTemplateStore()
	store.Register(&Template{Name: "test", Content: `Dear {{user.name}},

{{#if admin}}You have admin access.{{else}}You are a regular user.{{/if}}

Your items:
{{#each items as item}}- {{item}}
{{/each}}

Regards`})
	engine := NewEngine(store)

	result, err := engine.Render("test", RenderData{
		"user":  map[string]interface{}{"name": "Alice"},
		"admin": true,
		"items": []interface{}{"Apple", "Banana"},
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	if !strings.Contains(result, "Dear Alice") {
		t.Error("missing greeting")
	}
	if !strings.Contains(result, "admin access") {
		t.Error("missing admin section")
	}
	if !strings.Contains(result, "- Apple") {
		t.Error("missing Apple item")
	}
	if !strings.Contains(result, "- Banana") {
		t.Error("missing Banana item")
	}
}