package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yurika0211/luckyagent/internal/gateway"
)

type uploadResponse struct {
	Attachments []gateway.Attachment `json:"attachments"`
	Count       int                  `json:"count"`
}

func postUpload(t *testing.T, server *Server, files map[string][]byte, contentTypes map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for name, data := range files {
		header := make(map[string][]string)
		header["Content-Disposition"] = []string{`form-data; name="file"; filename="` + name + `"`}
		if ct, ok := contentTypes[name]; ok {
			header["Content-Type"] = []string{ct}
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	server.handleUploads(recorder, req)
	return recorder
}

func TestUploadStoresFileAndReturnsDescriptor(t *testing.T) {
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())

	payload := []byte("hello attachment")
	recorder := postUpload(t, server,
		map[string][]byte{"notes.txt": payload},
		map[string]string{"notes.txt": "text/plain"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response uploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Count != 1 || len(response.Attachments) != 1 {
		t.Fatalf("count = %d, attachments = %d, want 1", response.Count, len(response.Attachments))
	}

	att := response.Attachments[0]
	if att.FileName != "notes.txt" {
		t.Fatalf("file_name = %q, want notes.txt", att.FileName)
	}
	if att.Type != gateway.AttachmentDocument {
		t.Fatalf("type = %q, want document", att.Type)
	}
	if att.FileSize != int64(len(payload)) {
		t.Fatalf("file_size = %d, want %d", att.FileSize, len(payload))
	}
	if att.FilePath == "" {
		t.Fatal("file_path is empty; the multimodal pipeline reads documents from disk")
	}

	stored, err := os.ReadFile(att.FilePath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("stored bytes = %q, want %q", stored, payload)
	}
}

func TestUploadClassifiesImagesAndAudio(t *testing.T) {
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())

	cases := []struct {
		name     string
		mime     string
		wantType gateway.AttachmentType
	}{
		{"shot.png", "image/png", gateway.AttachmentImage},
		{"clip.mp3", "audio/mpeg", gateway.AttachmentAudio},
		{"movie.mp4", "video/mp4", gateway.AttachmentVideo},
		{"paper.pdf", "application/pdf", gateway.AttachmentDocument},
		// Browsers frequently send this for unknown types; the extension decides.
		{"photo.jpg", "application/octet-stream", gateway.AttachmentImage},
	}

	for _, tc := range cases {
		recorder := postUpload(t, server,
			map[string][]byte{tc.name: []byte("x")},
			map[string]string{tc.name: tc.mime})
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
		}
		var response uploadResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if got := response.Attachments[0].Type; got != tc.wantType {
			t.Fatalf("%s (%s): type = %q, want %q", tc.name, tc.mime, got, tc.wantType)
		}
	}
}

func TestUploadRejectsPathTraversalInFilename(t *testing.T) {
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())

	recorder := postUpload(t, server,
		map[string][]byte{`../../escaped.txt`: []byte("nope")},
		nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response uploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	att := response.Attachments[0]
	if strings.Contains(att.FileName, "..") || strings.ContainsAny(att.FileName, `/\`) {
		t.Fatalf("file_name = %q, want the directory components stripped", att.FileName)
	}

	dir, err := server.uploadDir()
	if err != nil {
		t.Fatalf("upload dir: %v", err)
	}
	resolved, err := filepath.Abs(att.FilePath)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if !strings.HasPrefix(resolved, dir) {
		t.Fatalf("stored at %q, which escapes the upload directory %q", resolved, dir)
	}
}

func TestUploadWithoutFileIsRejected(t *testing.T) {
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("message", "no file here")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	server.handleUploads(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestUploadRejectsNonPost(t *testing.T) {
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads", nil)
	recorder := httptest.NewRecorder()
	server.handleUploads(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
