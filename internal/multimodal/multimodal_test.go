package multimodal

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalProvider_AnalyzeText(t *testing.T) {
	provider := NewLocalProvider(ModalityText)
	assert.Equal(t, "local", provider.Name())
	assert.Equal(t, []Modality{ModalityText}, provider.SupportedModalities())
	assert.NoError(t, provider.Validate())

	ctx := context.Background()
	input := &Input{
		ID:       "test-1",
		Modality: ModalityText,
		MimeType: "text/plain",
		Data:     []byte("Hello, world!"),
	}

	result, err := provider.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "test-1", result.InputID)
	assert.Equal(t, ModalityText, result.Modality)
	assert.Equal(t, "Hello, world!", result.Text)
	assert.Equal(t, 1.0, result.Confidence)
	assert.Contains(t, result.Labels, "text")
}

func TestLocalProvider_AnalyzeImage(t *testing.T) {
	provider := NewLocalProvider(ModalityImage, ModalityText)

	ctx := context.Background()
	input := &Input{
		ID:       "img-1",
		Modality: ModalityImage,
		MimeType: "image/png",
		Data:     make([]byte, 1024),
	}

	result, err := provider.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, ModalityImage, result.Modality)
	assert.Contains(t, result.Text, "Image")
	assert.Equal(t, 0.5, result.Confidence)
	assert.Equal(t, "1024", result.Metadata["size"])
}

func TestLocalProvider_AnalyzeAudio(t *testing.T) {
	provider := NewLocalProvider(ModalityAudio)

	ctx := context.Background()
	input := &Input{
		ID:       "audio-1",
		Modality: ModalityAudio,
		MimeType: "audio/mp3",
		Data:     make([]byte, 2048),
	}

	result, err := provider.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, ModalityAudio, result.Modality)
	assert.Contains(t, result.Text, "Audio")
	assert.Equal(t, 0.3, result.Confidence)
}

func TestLocalProvider_AnalyzeVideo(t *testing.T) {
	provider := NewLocalProvider(ModalityVideo)

	ctx := context.Background()
	input := &Input{
		ID:       "vid-1",
		Modality: ModalityVideo,
		MimeType: "video/mp4",
		Data:     make([]byte, 4096),
	}

	result, err := provider.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, ModalityVideo, result.Modality)
	assert.Contains(t, result.Text, "Video")
	assert.Equal(t, 0.3, result.Confidence)
}

func TestLocalProvider_AnalyzeStream(t *testing.T) {
	provider := NewLocalProvider(ModalityText)

	ctx := context.Background()
	input := &Input{
		ID:       "stream-1",
		Modality: ModalityText,
		MimeType: "text/plain",
		Data:     []byte("streaming text"),
	}

	ch, err := provider.AnalyzeStream(ctx, input)
	require.NoError(t, err)

	var chunks []string
	for chunk := range ch {
		if !chunk.Done {
			chunks = append(chunks, chunk.Text)
		}
	}
	assert.Equal(t, []string{"streaming text"}, chunks)
}

func TestLocalProvider_AnalyzedCount(t *testing.T) {
	provider := NewLocalProvider(ModalityText)
	assert.Equal(t, 0, provider.AnalyzedCount())

	ctx := context.Background()
	input := &Input{Modality: ModalityText, Data: []byte("test")}

	provider.Analyze(ctx, input)
	assert.Equal(t, 1, provider.AnalyzedCount())

	provider.Analyze(ctx, input)
	assert.Equal(t, 2, provider.AnalyzedCount())
}

func TestProcessor_RegisterProvider(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText, ModalityImage)

	err := proc.RegisterProvider(provider, true)
	require.NoError(t, err)

	modalities := proc.SupportedModalities()
	assert.Equal(t, 2, len(modalities))
	assert.Contains(t, modalities, ModalityText)
	assert.Contains(t, modalities, ModalityImage)
	assert.Equal(t, 1, len(proc.ProvidersForModality(ModalityText)))
	assert.Equal(t, 1, len(proc.ProvidersForModality(ModalityImage)))
}

func TestProcessor_Analyze(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText, ModalityImage)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		ID:       "proc-1",
		Modality: ModalityText,
		MimeType: "text/plain",
		Data:     []byte("processor test"),
	}

	result, err := proc.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "processor test", result.Text)
	assert.NotZero(t, result.Duration)
}

func TestProcessor_AnalyzeNoProvider(t *testing.T) {
	proc := NewProcessor()

	ctx := context.Background()
	input := &Input{
		Modality: ModalityVideo,
		Data:     []byte("test"),
	}

	_, err := proc.Analyze(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider")
}

func TestProcessor_AnalyzeWithProvider(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		Modality: ModalityText,
		Data:     []byte("named provider test"),
	}

	result, err := proc.AnalyzeWithProvider(ctx, "local", input)
	require.NoError(t, err)
	assert.Equal(t, "named provider test", result.Text)
}

func TestProcessor_AnalyzeWithProviderNotFound(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		Modality: ModalityText,
		Data:     []byte("test"),
	}

	_, err := proc.AnalyzeWithProvider(ctx, "nonexistent", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProcessor_AnalyzeStream(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		Modality: ModalityText,
		Data:     []byte("stream test"),
	}

	ch, err := proc.AnalyzeStream(ctx, input)
	require.NoError(t, err)

	var text string
	for chunk := range ch {
		if !chunk.Done {
			text += chunk.Text
		}
	}
	assert.Equal(t, "stream test", text)
}

func TestProcessor_MultipleProviders(t *testing.T) {
	proc := NewProcessor()

	p1 := NewLocalProvider(ModalityText)
	p2 := NewLocalProvider(ModalityText)

	proc.RegisterProvider(p1, true)  // default
	proc.RegisterProvider(p2, false) // non-default

	providers := proc.ProvidersForModality(ModalityText)
	assert.Equal(t, 2, len(providers))
}

func TestDetectModality(t *testing.T) {
	tests := []struct {
		mime     string
		expected Modality
	}{
		{"text/plain", ModalityText},
		{"text/markdown", ModalityText},
		{"image/png", ModalityImage},
		{"image/jpeg", ModalityImage},
		{"audio/mp3", ModalityAudio},
		{"audio/wav", ModalityAudio},
		{"video/mp4", ModalityVideo},
		{"video/webm", ModalityVideo},
		{"application/json", ModalityText},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, DetectModality(tt.mime))
	}
}

func TestNewInput(t *testing.T) {
	input := NewInput(ModalityText, "text/plain", []byte("hello"))
	assert.NotEmpty(t, input.ID)
	assert.Equal(t, ModalityText, input.Modality)
	assert.Equal(t, "text/plain", input.MimeType)
	assert.Equal(t, []byte("hello"), input.Data)
	assert.WithinDuration(t, time.Now(), input.CreatedAt, time.Second)
}

func TestNewInputFromReader(t *testing.T) {
	reader := bytes.NewReader([]byte("reader test"))
	input, err := NewInputFromReader(ModalityText, "text/plain", reader)
	require.NoError(t, err)
	assert.Equal(t, []byte("reader test"), input.Data)
	assert.Equal(t, ModalityText, input.Modality)
}

func TestNewInputFromURL(t *testing.T) {
	input := NewInputFromURL(ModalityImage, "https://example.com/img.png")
	assert.Equal(t, "https://example.com/img.png", input.URL)
	assert.Equal(t, ModalityImage, input.Modality)
}

func TestNewInputFromPath(t *testing.T) {
	input := NewInputFromPath(ModalityAudio, "/tmp/audio.mp3")
	assert.Equal(t, "/tmp/audio.mp3", input.FilePath)
	assert.Equal(t, ModalityAudio, input.Modality)
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "hello", truncateString("hello", 10))
	assert.Equal(t, "hello...", truncateString("hello world", 5))
}

func TestLocalProvider_UnsupportedModality(t *testing.T) {
	provider := NewLocalProvider(ModalityText)

	ctx := context.Background()
	input := &Input{
		Modality: ModalityVideo,
		Data:     []byte("test"),
	}

	_, err := provider.Analyze(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestProcessor_SupportedModalities(t *testing.T) {
	proc := NewProcessor()
	assert.Empty(t, proc.SupportedModalities())

	proc.RegisterProvider(NewLocalProvider(ModalityText, ModalityImage), true)
	modalities := proc.SupportedModalities()
	assert.Equal(t, 2, len(modalities))
	assert.Contains(t, modalities, ModalityText)
	assert.Contains(t, modalities, ModalityImage)
}

func TestProcessor_AnalyzeImageWithLocalProvider(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityImage, ModalityText)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		ID:       "img-proc-1",
		Modality: ModalityImage,
		MimeType: "image/jpeg",
		Data:     make([]byte, 512),
	}

	result, err := proc.Analyze(ctx, input)
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Image")
	assert.Equal(t, 0.5, result.Confidence)
}

func TestLocalProvider_DefaultModality(t *testing.T) {
	provider := NewLocalProvider()
	assert.Equal(t, []Modality{ModalityText}, provider.SupportedModalities())
}

func TestProcessor_AnalyzeRecordsDuration(t *testing.T) {
	proc := NewProcessor()
	provider := NewLocalProvider(ModalityText)
	proc.RegisterProvider(provider, true)

	ctx := context.Background()
	input := &Input{
		Modality: ModalityText,
		Data:     []byte("duration test"),
	}

	result, err := proc.Analyze(ctx, input)
	require.NoError(t, err)
	assert.True(t, result.Duration >= 0)
}

func TestProcessor_AnalyzeRecordsError(t *testing.T) {
	proc := NewProcessor()
	// No provider registered for video

	ctx := context.Background()
	input := &Input{
		ID:       "err-1",
		Modality: ModalityVideo,
		Data:     []byte("test"),
	}

	_, err := proc.Analyze(ctx, input)
	assert.Error(t, err)
}

func TestStringsContains(t *testing.T) {
	// Verify our modality constants
	assert.True(t, strings.Contains(string(ModalityImage), "image"))
	assert.True(t, strings.Contains(string(ModalityAudio), "audio"))
	assert.True(t, strings.Contains(string(ModalityVideo), "video"))
	assert.True(t, strings.Contains(string(ModalityText), "text"))
}