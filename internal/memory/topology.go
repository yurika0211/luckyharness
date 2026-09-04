package memory

import (
	"path/filepath"
	"sort"
	"strings"
)

// GraphTopologyNode is one note in the memory vault, or one wikilink target
// that no note answers to yet.
type GraphTopologyNode struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Category   string   `json:"category,omitempty"`
	Tier       string   `json:"tier,omitempty"`
	Path       string   `json:"path,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Importance float64  `json:"importance"`
	Degree     int      `json:"degree"`
	// Resolved is false for a link target with no note behind it — Obsidian's
	// "unresolved link". Keeping them visible is the point: they show where the
	// vault refers to something it has not written down.
	Resolved bool `json:"resolved"`
}

// GraphTopologyEdge is a wikilink from one note to another.
type GraphTopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Weight int    `json:"weight"`
}

// GraphTopology is a read-only snapshot of the wikilink graph.
type GraphTopology struct {
	Nodes         []GraphTopologyNode `json:"nodes"`
	Edges         []GraphTopologyEdge `json:"edges"`
	TotalNotes    int                 `json:"total_notes"`
	TotalEdges    int                 `json:"total_edges"`
	IsolatedCount int                 `json:"isolated_count"`
	Unresolved    int                 `json:"unresolved"`
	Truncated     bool                `json:"truncated"`
	Categories    []string            `json:"categories,omitempty"`
}

// GraphTopologyOptions controls what a snapshot includes.
type GraphTopologyOptions struct {
	// Limit caps the node count, keeping the most connected nodes. Zero means
	// no cap.
	Limit int
	// IncludeIsolated keeps notes that neither link out nor are linked to.
	// They dominate the node count in a typical vault while carrying no
	// topology, so they are excluded by default.
	IncludeIsolated bool
}

// GraphTopology returns the note graph derived from Obsidian wikilinks. The
// markdown notes remain the source of truth; this is a derived view for
// display, computed from the same link normalization the recall path uses.
func (s *Store) GraphTopology(opts GraphTopologyOptions) GraphTopology {
	if s == nil {
		return GraphTopology{Nodes: []GraphTopologyNode{}, Edges: []GraphTopologyEdge{}}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// name key -> entry ID, rebuilt here rather than read off s.graph so a
	// stale index can never produce edges that disagree with the notes.
	nameToID := make(map[string]string, len(s.entries)*2)
	for id, entry := range s.entries {
		for _, alias := range graphAliasesForEntry(entry) {
			key := graphKey(alias)
			if key == "" {
				continue
			}
			if _, taken := nameToID[key]; !taken {
				nameToID[key] = id
			}
		}
	}

	nodes := make(map[string]*GraphTopologyNode, len(s.entries))
	for id, entry := range s.entries {
		nodes[id] = &GraphTopologyNode{
			ID:         id,
			Title:      entryDisplayTitle(entry),
			Category:   entry.Category,
			Tier:       entry.Tier.String(),
			Path:       entry.Path,
			Tags:       append([]string(nil), entry.Tags...),
			Importance: entry.Importance,
			Resolved:   true,
		}
	}

	weights := make(map[string]int)
	order := make([]string, 0, len(s.entries))
	unresolved := 0

	for id, entry := range s.entries {
		for _, link := range normalizeLinks(append(append([]string(nil), entry.Links...), extractWikiLinks(entry.Content)...)) {
			targetID := ""
			for _, key := range graphKeysForLink(link) {
				if resolved, ok := nameToID[key]; ok {
					targetID = resolved
					break
				}
			}
			if targetID == "" {
				// Surface the dangling link as its own node instead of
				// dropping the edge and hiding the gap.
				key := graphKey(link)
				if key == "" {
					continue
				}
				targetID = "unresolved:" + key
				if _, seen := nodes[targetID]; !seen {
					nodes[targetID] = &GraphTopologyNode{
						ID:       targetID,
						Title:    strings.TrimSpace(link),
						Resolved: false,
					}
					unresolved++
				}
			}
			if targetID == id {
				continue
			}
			edgeKey := id + "\x00" + targetID
			if _, seen := weights[edgeKey]; !seen {
				order = append(order, edgeKey)
			}
			weights[edgeKey]++
		}
	}

	for _, edgeKey := range order {
		parts := strings.SplitN(edgeKey, "\x00", 2)
		nodes[parts[0]].Degree++
		nodes[parts[1]].Degree++
	}

	totalNotes := len(s.entries)
	isolated := 0
	kept := make([]GraphTopologyNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Degree == 0 {
			isolated++
			if !opts.IncludeIsolated {
				continue
			}
		}
		kept = append(kept, *node)
	}

	// Most connected first, so a cap keeps the informative core; ties break on
	// title to keep the response stable between calls.
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Degree != kept[j].Degree {
			return kept[i].Degree > kept[j].Degree
		}
		return kept[i].Title < kept[j].Title
	})

	truncated := false
	if opts.Limit > 0 && len(kept) > opts.Limit {
		kept = kept[:opts.Limit]
		truncated = true
	}

	visible := make(map[string]struct{}, len(kept))
	for _, node := range kept {
		visible[node.ID] = struct{}{}
	}

	edges := make([]GraphTopologyEdge, 0, len(order))
	for _, edgeKey := range order {
		parts := strings.SplitN(edgeKey, "\x00", 2)
		if _, ok := visible[parts[0]]; !ok {
			continue
		}
		if _, ok := visible[parts[1]]; !ok {
			continue
		}
		edges = append(edges, GraphTopologyEdge{Source: parts[0], Target: parts[1], Weight: weights[edgeKey]})
	}

	categorySet := make(map[string]struct{})
	for _, node := range kept {
		if node.Category != "" {
			categorySet[node.Category] = struct{}{}
		}
	}
	categories := make([]string, 0, len(categorySet))
	for category := range categorySet {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	return GraphTopology{
		Nodes:         kept,
		Edges:         edges,
		TotalNotes:    totalNotes,
		TotalEdges:    len(order),
		IsolatedCount: isolated,
		Unresolved:    unresolved,
		Truncated:     truncated,
		Categories:    categories,
	}
}

// entryDisplayTitle prefers the note filename, which is what wikilinks point
// at, and falls back to the first line of the body.
func entryDisplayTitle(e *Entry) string {
	if e == nil {
		return ""
	}
	if path := strings.TrimSpace(e.Path); path != "" {
		base := strings.TrimSuffix(filepath.Base(filepath.ToSlash(path)), ".md")
		if base != "" {
			return base
		}
	}
	for _, line := range strings.Split(e.Content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "# "))
		if line == "" {
			continue
		}
		if len([]rune(line)) > 60 {
			return string([]rune(line)[:60]) + "…"
		}
		return line
	}
	return e.ID
}
