package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

type telegramMethodRecorder struct {
	mu      sync.Mutex
	methods []string
}

func (r *telegramMethodRecorder) add(method string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.methods = append(r.methods, method)
}

func (r *telegramMethodRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.methods))
	copy(out, r.methods)
	return out
}

func newCaptureBotAdapter(t *testing.T) (*Adapter, func(), *telegramMethodRecorder) {
	t.Helper()

	recorder := &telegramMethodRecorder{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result map[string]any

		switch {
		case containsMethod(r.URL.Path, "getMe"):
			result = map[string]any{
				"ok": true,
				"result": map[string]any{
					"id":         123456789,
					"is_bot":     true,
					"first_name": "TestBot",
					"username":   "testbot",
				},
			}
		case containsMethod(r.URL.Path, "sendMessage"):
			recorder.add("sendMessage")
			result = map[string]any{
				"ok": true,
				"result": map[string]any{
					"message_id": 42,
					"chat": map[string]any{
						"id": 12345,
					},
					"text": "ok",
				},
			}
		case containsMethod(r.URL.Path, "sendPhoto"):
			recorder.add("sendPhoto")
			result = map[string]any{
				"ok": true,
				"result": map[string]any{
					"message_id": 43,
					"chat": map[string]any{
						"id": 12345,
					},
				},
			}
		case containsMethod(r.URL.Path, "sendDocument"):
			recorder.add("sendDocument")
			result = map[string]any{
				"ok": true,
				"result": map[string]any{
					"message_id": 44,
					"chat": map[string]any{
						"id": 12345,
					},
				},
			}
		default:
			result = map[string]any{
				"ok":     true,
				"result": map[string]any{},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on localhost: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", baseURL+"/bot%s/%s")
	if err != nil {
		_ = server.Close()
		_ = listener.Close()
		t.Fatalf("create mock bot: %v", err)
	}

	adapter := NewAdapter(Config{Token: bot.Token})
	adapter.bot = bot
	adapter.botUsername = "testbot"
	adapter.running = true
	cleanup := func() {
		_ = server.Close()
		_ = listener.Close()
	}
	return adapter, cleanup, recorder
}

func TestParseOutboundMediaResponseDirective(t *testing.T) {
	text, media := parseOutboundMediaResponse("Here is the report\n\ntg://document /tmp/report.pdf monthly report")
	if text != "Here is the report" {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(media) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(media))
	}
	if media[0].Kind != outboundMediaDocument {
		t.Fatalf("expected document media, got %q", media[0].Kind)
	}
	if media[0].Source != "/tmp/report.pdf" {
		t.Fatalf("unexpected source: %q", media[0].Source)
	}
	if media[0].Caption != "monthly report" {
		t.Fatalf("unexpected caption: %q", media[0].Caption)
	}
}

func TestParseOutboundMediaResponseMarkdownImage(t *testing.T) {
	text, media := parseOutboundMediaResponse("Take a look:\n\n![trend](https://example.com/trend.png)")
	if text != "Take a look:" {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(media) != 1 || media[0].Kind != outboundMediaPhoto {
		t.Fatalf("expected one photo media item, got %#v", media)
	}
	if media[0].Caption != "trend" {
		t.Fatalf("unexpected caption: %q", media[0].Caption)
	}
}

func TestParseOutboundMediaResponseMarkdownSandboxLink(t *testing.T) {
	text, media := parseOutboundMediaResponse("PNG 在这里：[quadratic.png](sandbox:/tmp/quadratic.png)")
	if text != "PNG 在这里：" {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(media) != 1 || media[0].Kind != outboundMediaPhoto {
		t.Fatalf("expected one photo media item, got %#v", media)
	}
	if media[0].Source != "sandbox:/tmp/quadratic.png" {
		t.Fatalf("unexpected source: %q", media[0].Source)
	}
}

func TestParseOutboundMediaResponseImplicitDocument(t *testing.T) {
	text, media := parseOutboundMediaResponse("/tmp/output.pdf")
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
	if len(media) != 1 || media[0].Kind != outboundMediaDocument {
		t.Fatalf("expected implicit document media, got %#v", media)
	}
}

func TestMaterializeGeneratedDocumentSVG(t *testing.T) {
	response := "可以，先给你一个可直接保存为 `quadratic.svg` 的示例：\n\n```svg\n<svg><rect width=\"10\" height=\"10\"/></svg>\n```"
	text, media, err := materializeGeneratedDocuments(response)
	if err != nil {
		t.Fatalf("materializeGeneratedDocuments: %v", err)
	}
	if !strings.Contains(text, "quadratic.svg") {
		t.Fatalf("expected explanatory text to remain, got %q", text)
	}
	if len(media) != 1 || media[0].Kind != outboundMediaDocument {
		t.Fatalf("expected one generated document, got %#v", media)
	}
	if !strings.HasSuffix(media[0].Source, ".svg") {
		t.Fatalf("expected svg temp file, got %q", media[0].Source)
	}
	data, err := os.ReadFile(media[0].Source)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if !strings.Contains(string(data), "<svg>") {
		t.Fatalf("unexpected generated file content: %q", string(data))
	}
	_ = os.Remove(media[0].Source)
}

func TestMaterializeGeneratedDocumentWithoutFilename(t *testing.T) {
	response := "下面是图像文件内容：\n\n```svg\n<svg><circle cx=\"5\" cy=\"5\" r=\"5\"/></svg>\n```"
	text, media, err := materializeGeneratedDocuments(response)
	if err != nil {
		t.Fatalf("materializeGeneratedDocuments: %v", err)
	}
	if !strings.Contains(text, "下面是图像文件内容") {
		t.Fatalf("expected explanatory text to remain, got %q", text)
	}
	if len(media) != 1 {
		t.Fatalf("expected one generated document, got %#v", media)
	}
	if media[0].Caption != "generated-1.svg" {
		t.Fatalf("unexpected generated caption: %q", media[0].Caption)
	}
	_ = os.Remove(media[0].Source)
}

func TestMaterializeGeneratedDocumentsMultipleBlocks(t *testing.T) {
	response := "保存为 `a.svg`\n```svg\n<svg><rect width=\"1\" height=\"1\"/></svg>\n```\n\n保存为 `b.json`\n```json\n{\"ok\":true}\n```"
	text, media, err := materializeGeneratedDocuments(response)
	if err != nil {
		t.Fatalf("materializeGeneratedDocuments: %v", err)
	}
	if strings.Contains(text, "```") {
		t.Fatalf("expected code fences removed, got %q", text)
	}
	if len(media) != 2 {
		t.Fatalf("expected two generated documents, got %#v", media)
	}
	if media[0].Caption != "a.svg" || media[1].Caption != "b.json" {
		t.Fatalf("unexpected captions: %#v", media)
	}
	for _, item := range media {
		_ = os.Remove(item.Source)
	}
}

func TestMaterializeGeneratedDocumentsMultipleUnnamedBlocks(t *testing.T) {
	response := "给你两个文件：\n\n```json\n{\"ok\":true}\n```\n\n```svg\n<svg><circle r=\"5\"/></svg>\n```"
	text, media, err := materializeGeneratedDocuments(response)
	if err != nil {
		t.Fatalf("materializeGeneratedDocuments: %v", err)
	}
	if !strings.Contains(text, "给你两个文件") {
		t.Fatalf("expected explanatory text to remain, got %q", text)
	}
	if len(media) != 2 {
		t.Fatalf("expected 2 generated documents, got %#v", media)
	}
	if !strings.HasSuffix(media[0].Caption, ".json") {
		t.Fatalf("expected json caption, got %q", media[0].Caption)
	}
	if !strings.HasSuffix(media[1].Caption, ".svg") {
		t.Fatalf("expected svg caption, got %q", media[1].Caption)
	}
	for _, item := range media {
		_ = os.Remove(item.Source)
	}
}

func TestSendAssistantResponsePhotoDirective(t *testing.T) {
	adapter, cleanup, recorder := newCaptureBotAdapter(t)
	defer cleanup()

	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "chart.png")
	if err := os.WriteFile(imagePath, []byte("fake image data"), 0600); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	handler := NewHandler(adapter, nil)
	msg := &gateway.Message{
		ID: "1",
		Chat: gateway.Chat{
			ID:   "12345",
			Type: gateway.ChatPrivate,
		},
	}

	response := "Here is the chart\n\ntg://photo " + imagePath + " chart"
	if err := handler.sendAssistantResponse(context.Background(), msg, response); err != nil {
		t.Fatalf("sendAssistantResponse: %v", err)
	}

	methods := strings.Join(recorder.snapshot(), ",")
	if methods != "sendMessage,sendPhoto" {
		t.Fatalf("unexpected send sequence: %s", methods)
	}
}

func TestSendAssistantResponseImplicitDocument(t *testing.T) {
	adapter, cleanup, recorder := newCaptureBotAdapter(t)
	defer cleanup()

	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "report.pdf")
	if err := os.WriteFile(docPath, []byte("%PDF-1.4"), 0600); err != nil {
		t.Fatalf("write temp doc: %v", err)
	}

	handler := NewHandler(adapter, nil)
	msg := &gateway.Message{
		ID: "1",
		Chat: gateway.Chat{
			ID:   "12345",
			Type: gateway.ChatPrivate,
		},
	}

	if err := handler.sendAssistantResponse(context.Background(), msg, docPath); err != nil {
		t.Fatalf("sendAssistantResponse: %v", err)
	}

	methods := recorder.snapshot()
	if len(methods) != 1 || methods[0] != "sendDocument" {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}
