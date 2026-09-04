//go:build windows

package computer

import (
	"testing"
	"unsafe"
)

func TestNewWindowsBackend(t *testing.T) {
	backend, err := NewBackend("windows")
	if err != nil {
		t.Fatalf("NewBackend(windows) error = %v", err)
	}
	if backend.Name() != "windows" {
		t.Fatalf("backend name = %q", backend.Name())
	}
}

func TestWindowsVirtualKeyAliases(t *testing.T) {
	for raw, want := range map[string]uint16{
		"ctrl":  0x11,
		"win":   0x5B,
		"enter": 0x0D,
		"F12":   0x7B,
		"a":     'A',
	} {
		got, ok := windowsVirtualKey(raw)
		if !ok || got != want {
			t.Fatalf("windowsVirtualKey(%q) = %#x, %v; want %#x, true", raw, got, ok, want)
		}
	}
	if _, ok := windowsVirtualKey("not-a-key"); ok {
		t.Fatal("unsupported key was accepted")
	}
}

func TestWindowsUnicodeInputLayout(t *testing.T) {
	input := windowsKeyboardInput(0, 'A', winKeyEventUnicode)
	if input.Type != winInputKeyboard || input.Data[2] != 'A' || input.Data[4] != winKeyEventUnicode {
		t.Fatalf("unexpected unicode input layout: %#v", input)
	}
	if got := unsafe.Sizeof(winInput{}); got != 40 {
		t.Fatalf("INPUT size = %d, want 40", got)
	}
}
