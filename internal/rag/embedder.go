package rag

import (
	"context"

	"github.com/yurika0211/luckyharness/internal/embedder"
)

// EmbeddingProvider provides vector embeddings for text.
// Deprecated: Use embedder.Embedder instead. This interface is kept for backward compatibility.
// embedder.Embedder satisfies this interface (it has Embed, EmbedBatch, Dimension, Name).
type EmbeddingProvider = embedder.Embedder

// NewMockEmbedder creates a mock embedder for testing (delegates to embedder package).
func NewMockEmbedder(dim int) *embedder.MockEmbedder {
	return embedder.NewMockEmbedder(dim)
}

// NewOpenAIEmbedder creates an OpenAI embedder (delegates to embedder package).
func NewOpenAIEmbedder(cfg embedder.OpenAIEmbedderConfig) *embedder.OpenAIEmbedder {
	return embedder.NewOpenAIEmbedder(cfg)
}

// mockVector generates a deterministic pseudo-random vector from text.
// Kept for backward compatibility with existing tests.
func mockVector(text string, dim int) []float64 {
	e := embedder.NewMockEmbedder(dim)
	vec, _ := e.Embed(context.Background(), text)
	return vec
}
