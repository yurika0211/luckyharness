package utils

// ChunkFeature describes a candidate context chunk for ranking.
// All score fields are expected in [0,1].
type ChunkFeature struct {
	Relevance        float64
	Recency          float64
	RoleWeight       float64
	Importance       float64
	DuplicatePenalty float64
	Tokens           int
}

// ScoreWeights controls the weighted contribution of each feature.
type ScoreWeights struct {
	Relevance        float64
	Recency          float64
	RoleWeight       float64
	Importance       float64
	DuplicatePenalty float64
}

// DefaultScoreWeights returns baseline weights for context selection.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Relevance:        0.45,
		Recency:          0.20,
		RoleWeight:       0.15,
		Importance:       0.20,
		DuplicatePenalty: 0.20,
	}
}

// ScoreChunk returns a weighted score in [0,+inf), with duplicate penalty subtracted.
func ScoreChunk(feature ChunkFeature, weights ScoreWeights) float64 {
	w := sanitizeWeights(weights)

	score := w.Relevance*clamp01(feature.Relevance) +
		w.Recency*clamp01(feature.Recency) +
		w.RoleWeight*clamp01(feature.RoleWeight) +
		w.Importance*clamp01(feature.Importance) -
		w.DuplicatePenalty*clamp01(feature.DuplicatePenalty)

	if score < 0 {
		return 0
	}
	return score
}

// ScorePerToken returns score density for budget-constrained ranking.
func ScorePerToken(feature ChunkFeature, weights ScoreWeights) float64 {
	tokens := feature.Tokens
	if tokens <= 0 {
		tokens = 1
	}
	return ScoreChunk(feature, weights) / float64(tokens)
}

func sanitizeWeights(w ScoreWeights) ScoreWeights {
	if w == (ScoreWeights{}) {
		return DefaultScoreWeights()
	}
	if w.Relevance < 0 {
		w.Relevance = 0
	}
	if w.Recency < 0 {
		w.Recency = 0
	}
	if w.RoleWeight < 0 {
		w.RoleWeight = 0
	}
	if w.Importance < 0 {
		w.Importance = 0
	}
	if w.DuplicatePenalty < 0 {
		w.DuplicatePenalty = 0
	}
	return w
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
