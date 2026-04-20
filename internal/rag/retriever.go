package rag

import (
	"context"
	"fmt"
	"math"
	"sort"
)

// RetrievalResult is a result from the RAG retriever.
type RetrievalResult struct {
	ChunkID   string
	Content   string
	Score     float64
	Metadata  map[string]string
	DocTitle  string
	DocSource string
}

// RetrieverConfig holds retriever configuration.
type RetrieverConfig struct {
	TopK         int     // number of results to return (default 5)
	MinScore     float64 // minimum similarity score (default 0.5)
	UseMMR       bool    // use Maximal Marginal Relevance for diversity
	MMRLambda    float64 // MMR trade-off: 0=max diversity, 1=max relevance (default 0.5)
	FilterSource string  // filter by source
}

func DefaultRetrieverConfig() RetrieverConfig {
	return RetrieverConfig{
		TopK:      5,
		MinScore:  0.3,
		UseMMR:    false,
		MMRLambda: 0.5,
	}
}

// Retriever searches the vector store and returns relevant chunks.
type Retriever struct {
	store    *VectorStore
	indexer  *Indexer
	embedder EmbeddingProvider
	config   RetrieverConfig
}

func NewRetriever(store *VectorStore, indexer *Indexer, embedder EmbeddingProvider, config RetrieverConfig) *Retriever {
	if config.TopK <= 0 {
		config.TopK = 5
	}
	if config.MinScore <= 0 {
		config.MinScore = 0.3
	}
	if config.MMRLambda <= 0 {
		config.MMRLambda = 0.5
	}
	return &Retriever{
		store:    store,
		indexer:  indexer,
		embedder: embedder,
		config:   config,
	}
}

// Search queries the knowledge base and returns relevant chunks.
func (r *Retriever) Search(ctx context.Context, query string) ([]RetrievalResult, error) {
	// Embed the query
	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Search with a larger pool for MMR
	fetchK := r.config.TopK
	if r.config.UseMMR {
		fetchK = r.config.TopK * 4 // fetch more candidates for MMR reranking
	}

	var results []SearchResult
	if r.config.FilterSource != "" {
		results = r.store.SearchWithFilter(queryVec, fetchK, "source", r.config.FilterSource)
	} else {
		results = r.store.Search(queryVec, fetchK)
	}

	// Filter by minimum score
	var filtered []SearchResult
	for _, sr := range results {
		if sr.Score >= r.config.MinScore {
			filtered = append(filtered, sr)
		}
	}

	// MMR reranking
	if r.config.UseMMR && len(filtered) > r.config.TopK {
		filtered = r.mmrRerank(queryVec, filtered)
	}

	// Limit to TopK
	if len(filtered) > r.config.TopK {
		filtered = filtered[:r.config.TopK]
	}

	// Enrich with chunk content
	out := make([]RetrievalResult, len(filtered))
	for i, sr := range filtered {
		chunk, _ := r.indexer.GetChunk(sr.ID)
		content := ""
		docTitle := ""
		docSource := ""
		if chunk != nil {
			content = chunk.Content
			docTitle = chunk.Metadata["title"]
			docSource = chunk.Metadata["source"]
		}
		out[i] = RetrievalResult{
			ChunkID:   sr.ID,
			Content:   content,
			Score:     sr.Score,
			Metadata:  sr.Metadata,
			DocTitle:  docTitle,
			DocSource: docSource,
		}
	}

	return out, nil
}

// mmrRerank applies Maximal Marginal Relevance to diversify results.
func (r *Retriever) mmrRerank(queryVec []float64, candidates []SearchResult) []SearchResult {
	lambda := r.config.MMRLambda
	selected := make([]SearchResult, 0, r.config.TopK)
	remaining := make([]SearchResult, len(candidates))
	copy(remaining, candidates)

	for len(selected) < r.config.TopK && len(remaining) > 0 {
		bestIdx := 0
		bestScore := math.Inf(-1)

		for i, cand := range remaining {
			// Relevance component
			relevance := cand.Score

			// Diversity component: max similarity to already selected
			maxSim := 0.0
			if len(selected) > 0 {
				candVec, exists := r.store.Get(cand.ID)
				if exists {
					for _, sel := range selected {
						selVec, selExists := r.store.Get(sel.ID)
						if selExists {
							sim := cosineSimilarity(candVec.Vector, selVec.Vector)
							if sim > maxSim {
								maxSim = sim
							}
						}
					}
				}
			}

			// MMR score = λ * relevance - (1-λ) * max_similarity
			mmrScore := lambda*relevance - (1-lambda)*maxSim
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// BuildContext assembles retrieved chunks into a context string for the agent.
func (r *Retriever) BuildContext(results []RetrievalResult) string {
	if len(results) == 0 {
		return ""
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	context := "## Retrieved Knowledge\n\n"
	for i, res := range results {
		context += fmt.Sprintf("### Source %d: %s (score: %.2f)\n", i+1, res.DocTitle, res.Score)
		context += res.Content + "\n\n"
	}

	return context
}

// UpdateConfig updates the retriever configuration.
func (r *Retriever) UpdateConfig(config RetrieverConfig) {
	if config.TopK > 0 {
		r.config.TopK = config.TopK
	}
	if config.MinScore > 0 {
		r.config.MinScore = config.MinScore
	}
	r.config.UseMMR = config.UseMMR
	if config.MMRLambda > 0 {
		r.config.MMRLambda = config.MMRLambda
	}
	r.config.FilterSource = config.FilterSource
}

// Config returns the current retriever configuration.
func (r *Retriever) Config() RetrieverConfig {
	return r.config
}
