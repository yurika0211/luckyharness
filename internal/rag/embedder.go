package rag

import (
	"context"
	"fmt"
	"math"
)

// EmbeddingProvider provides vector embeddings for text.
type EmbeddingProvider interface {
	// Embed returns the embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)
	// EmbedBatch returns embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
	// Dimension returns the dimension of the embedding vectors.
	Dimension() int
	// Name returns the provider name.
	Name() string
}

// MockEmbedder is a simple embedder for testing that uses hash-based vectors.
type MockEmbedder struct {
	dim int
}

func NewMockEmbedder(dim int) *MockEmbedder {
	if dim <= 0 {
		dim = 128
	}
	return &MockEmbedder{dim: dim}
}

func (m *MockEmbedder) Name() string { return "mock" }

func (m *MockEmbedder) Dimension() int { return m.dim }

func (m *MockEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	return mockVector(text, m.dim), nil
}

func (m *MockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, t := range texts {
		result[i] = mockVector(t, m.dim)
	}
	return result, nil
}

// mockVector generates a deterministic pseudo-random vector from text.
// Uses a simple hash-based approach for reproducibility in tests.
func mockVector(text string, dim int) []float64 {
	vec := make([]float64, dim)
	if len(text) == 0 {
		return vec
	}
	// Simple hash: distribute characters across dimensions
	for i, ch := range text {
		idx := i % dim
		vec[idx] += float64(ch)
	}
	// Normalize
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// OpenAIEmbedder calls OpenAI's embedding API.
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	baseURL    string
	dimension  int
	httpClient interface{} // *http.Client, kept as interface to avoid import
}

type OpenAIEmbedderConfig struct {
	APIKey    string
	Model     string
	BaseURL   string // defaults to https://api.openai.com/v1
	Dimension int    // defaults to model's native dimension
}

func NewOpenAIEmbedder(cfg OpenAIEmbedderConfig) *OpenAIEmbedder {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "text-embedding-3-small"
	}
	dim := cfg.Dimension
	if dim <= 0 {
		dim = 1536 // text-embedding-3-small default
	}
	return &OpenAIEmbedder{
		apiKey:    cfg.APIKey,
		model:     model,
		baseURL:   baseURL,
		dimension: dim,
	}
}

func (o *OpenAIEmbedder) Name() string { return "openai-embedding" }

func (o *OpenAIEmbedder) Dimension() int { return o.dimension }

func (o *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vecs, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vecs[0], nil
}

func (o *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	// This is a placeholder implementation.
	// In production, this would call the OpenAI embeddings API:
	// POST {baseURL}/embeddings with model + input array
	// For now, return mock vectors to allow the RAG pipeline to work without API keys.
	embedder := NewMockEmbedder(o.dimension)
	return embedder.EmbedBatch(ctx, texts)
}
