package rag

import (
	"context"
	"fmt"
	"sync"
)

// RAGManager is the top-level RAG system manager.
type RAGManager struct {
	store    *VectorStore
	indexer  *Indexer
	retriever *Retriever
	embedder EmbeddingProvider

	mu sync.RWMutex
}

type RAGConfig struct {
	EmbeddingDim    int
	RetrieverConfig RetrieverConfig
}

func DefaultRAGConfig() RAGConfig {
	return RAGConfig{
		EmbeddingDim:    0, // 0 = auto-detect from embedder
		RetrieverConfig: DefaultRetrieverConfig(),
	}
}

// NewRAGManager creates a new RAG system with the given embedder.
func NewRAGManager(embedder EmbeddingProvider, config RAGConfig) *RAGManager {
	dim := config.EmbeddingDim
	if dim <= 0 {
		dim = embedder.Dimension()
	}
	if dim <= 0 {
		dim = 128
	}

	store := NewVectorStore(dim)
	indexer := NewIndexer(store, embedder)
	retriever := NewRetriever(store, indexer, embedder, config.RetrieverConfig)

	return &RAGManager{
		store:     store,
		indexer:   indexer,
		retriever: retriever,
		embedder:  embedder,
	}
}

// IndexFile indexes a single file.
func (m *RAGManager) IndexFile(path string) (*Document, error) {
	return m.indexer.IndexFile(path)
}

// IndexText indexes raw text content.
func (m *RAGManager) IndexText(source, title, content string) (*Document, error) {
	return m.indexer.IndexText(source, title, content)
}

// IndexDirectory indexes all .md/.txt files in a directory.
func (m *RAGManager) IndexDirectory(dir string) ([]*Document, error) {
	return m.indexer.IndexDirectory(dir)
}

// Search queries the knowledge base.
func (m *RAGManager) Search(ctx context.Context, query string) ([]RetrievalResult, error) {
	return m.retriever.Search(ctx, query)
}

// SearchWithContext queries and returns assembled context string.
func (m *RAGManager) SearchWithContext(ctx context.Context, query string) (string, []RetrievalResult, error) {
	results, err := m.retriever.Search(ctx, query)
	if err != nil {
		return "", nil, err
	}
	context := m.retriever.BuildContext(results)
	return context, results, nil
}

// RemoveDocument removes a document from the index.
func (m *RAGManager) RemoveDocument(docID string) bool {
	return m.indexer.RemoveDocument(docID)
}

// Stats returns index statistics.
func (m *RAGManager) Stats() IndexStats {
	return m.indexer.Stats()
}

// ListDocuments returns all document IDs.
func (m *RAGManager) ListDocuments() []string {
	return m.indexer.ListDocuments()
}

// GetDocument returns a document by ID.
func (m *RAGManager) GetDocument(docID string) (*Document, bool) {
	return m.indexer.GetDocument(docID)
}

// UpdateRetrieverConfig updates the retriever configuration.
func (m *RAGManager) UpdateRetrieverConfig(config RetrieverConfig) {
	m.retriever.UpdateConfig(config)
}

// RetrieverConfig returns the current retriever configuration.
func (m *RAGManager) RetrieverConfig() RetrieverConfig {
	return m.retriever.Config()
}

// Store returns the underlying vector store (for advanced use).
func (m *RAGManager) Store() *VectorStore {
	return m.store
}

// Indexer returns the underlying indexer (for advanced use).
func (m *RAGManager) Indexer() *Indexer {
	return m.indexer
}

// Retriever returns the underlying retriever (for advanced use).
func (m *RAGManager) Retriever() *Retriever {
	return m.retriever
}

// String returns a summary of the RAG system.
func (m *RAGManager) String() string {
	stats := m.Stats()
	return fmt.Sprintf("RAGManager{docs=%d, chunks=%d, embedder=%s, dim=%d}",
		stats.DocumentCount, stats.ChunkCount, m.embedder.Name(), m.store.Dimension())
}
