package tool

import (
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "echo",
		Description: "Echo back the input",
		Handler: func(args map[string]any) (string, error) {
			return args["text"].(string), nil
		},
	})

	tool, ok := r.Get("echo")
	if !ok {
		t.Error("tool not found")
	}
	if tool.Name != "echo" {
		t.Errorf("expected echo, got %s", tool.Name)
	}
}

func TestCall(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "add",
		Description: "Add numbers",
		Handler: func(args map[string]any) (string, error) {
			return "result", nil
		},
	})

	result, err := r.Call("add", map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "result" {
		t.Errorf("expected result, got %s", result)
	}
}

func TestCallNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Call("nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{Name: "a"})
	r.Register(&Tool{Name: "b"})

	tools := r.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}
