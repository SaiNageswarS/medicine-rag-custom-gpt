package mcp

import (
	"context"
	"math"
	"slices"
	"sort"

	"github.com/SaiNageswarS/agent-boot/schema"
	"github.com/SaiNageswarS/go-api-boot/embed"
	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-collection-boot/async"
	"github.com/SaiNageswarS/go-collection-boot/ds"
	"github.com/SaiNageswarS/go-collection-boot/linq"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// search parameters.
const (
	rrfK               = 60  // “dampening” constant from the RRF paper
	textSearchWeight   = 1.0 // optional per-engine weights
	vectorSearchWeight = 1.0
	vecK               = 30 // # of hits to keep from each engine
	textK              = 30
	maxChunks          = 30
)

type SearchTool struct {
	embedder         embed.Embedder
	chunkRepository  odm.OdmCollectionInterface[db.ChunkModel]
	vectorRepository odm.OdmCollectionInterface[db.ChunkAnnModel]
}

func NewSearchTool(chunkRepository odm.OdmCollectionInterface[db.ChunkModel], vectorRepository odm.OdmCollectionInterface[db.ChunkAnnModel], embedder embed.Embedder) *SearchTool {
	return &SearchTool{
		chunkRepository:  chunkRepository,
		vectorRepository: vectorRepository,
		embedder:         embedder,
	}
}

func (s *SearchTool) Run(ctx context.Context, query string) <-chan *schema.ToolResultChunk {
	out := make(chan *schema.ToolResultChunk, 20)

	go func() {
		defer close(out)

		// 1. Perform Hybrid Search and Collect results ranked by RRF score
		rankedChunks, err := async.Await(s.hybridSearch(ctx, query))
		if err != nil {
			logger.Error("Failed to perform hybrid search", zap.Error(err))
			out <- &schema.ToolResultChunk{
				Error: err.Error(),
			}
			return
		}

		// 2. Group by section with adjoining chunks and rank
		sectionChunks := GroupBySectionWithRank(rankedChunks)

		_, err = linq.Pipe3(
			linq.FromSlice(ctx, sectionChunks),

			// sort windows in the section.
			linq.Select(func(sectionChunks []*db.ChunkModel) []*db.ChunkModel {
				sort.Slice(sectionChunks, func(i, j int) bool {
					return sectionChunks[i].WindowIndex < sectionChunks[j].WindowIndex
				})
				return sectionChunks
			}),

			// get neighboring chunks
			linq.Select(func(sectionChunks []*db.ChunkModel) *schema.ToolResultChunk {
				result := &schema.ToolResultChunk{
					Title:       sectionChunks[0].Title,
					Attribution: sectionChunks[0].SourceURI,
					Id:          sectionChunks[0].SectionID,
				}

				cache := make(map[string]*db.ChunkModel, len(sectionChunks)*2)
				for _, ch := range sectionChunks {
					cache[ch.ChunkID] = ch
				}

				// Collect only missing neighbor IDs
				added := ds.NewSet[string]()
				needIds := make([]string, 0, len(sectionChunks)*2)
				for _, ch := range sectionChunks {
					if id := ch.PrevChunkID; id != "" && !added.Contains(id) {
						added.Add(id)
						needIds = append(needIds, id)
					}

					if id := ch.ChunkID; id != "" && !added.Contains(id) {
						added.Add(id)
						needIds = append(needIds, id)
					}

					if id := ch.NextChunkID; id != "" && !added.Contains(id) {
						added.Add(id)
						needIds = append(needIds, id)
					}
				}

				allChunks := s.fetchChunksByIds(ctx, cache, needIds)

				sentences := make([]string, 0, len(allChunks)*20)
				for _, chunk := range allChunks {
					sentences = append(sentences, chunk.Sentences...)
				}

				result.Sentences = sentences
				return result
			}),

			linq.ForEach(func(result *schema.ToolResultChunk) {
				// Add the result to the output channel
				out <- result
			}),
		)

		if err != nil {
			logger.Error("Failed to process section chunks", zap.Error(err))
		}
	}()

	return out
}

// ──────────────────────────────────────────────────────────────────────────────
//
//	Reciprocal-Rank Fusion (RRF)
//
//	Goal
//	────
//	▸ Convert *recall hits* (relevant docs that show up anywhere) into
//	  *precision hits* (relevant docs that land in the first N spots the user
//	  actually sees).
//
//	How it works
//	────────────
//	    RRF_score(d) = Σ_e  w_e / (k + rank_e(d))
//
//	    • One top-rank appearance (rank = 1) gets a big boost 1/(k+1), often
//	      enough to push the doc into the visible window.
//	    • A tail hit (rank = 20) earns < 1 % of that weight, so background
//	      noise barely moves the needle.
//
//	Why *rank* beats raw *score*
//	────────────────────────────
//	    – Scores live on different scales (BM25 ≈ 0-1000, cosine ≈ −1-1,
//	      PageRank ≪ 1).  Cross-normalising them is brittle.
//	    – Even a single engine’s scores drift when we rebuild the index or
//	      retrain embeddings; relative rank is far more stable.
//	    – Rank directly expresses “how good versus its peers,” the signal we
//	      need when merging heterogeneous lists.
//
//	Why we don’t hard-threshold BM25 or similarity scores
//	─────────────────────────────────────────────────────
//	    – The 1/(k+rank) formula *already* down-weights tail hits; a rank-20
//	      doc contributes < 1 % of a rank-1 doc, so low-quality noise is
//	      effectively ignored without hurting recall.
//	    – Fixed score cut-offs tie us to one model/index version and risk
//	      dropping docs that are mediocre in one engine but stellar in another
//	      (the classic hybrid-search win).
//
//	Bottom line
//	───────────
//	Let every engine vote by rank, fuse with 1/(k+rank), and keep explicit
//	score thresholds only for domain-specific guard-rails.
//
// ──────────────────────────────────────────────────────────────────────────────
func (s *SearchTool) hybridSearch(ctx context.Context, query string) <-chan async.Result[[]*db.ChunkModel] {

	return async.Go(func() ([]*db.ChunkModel, error) {
		//----------------------------------------------------------------------
		// 1. Fire the two independent searches in parallel
		//----------------------------------------------------------------------
		textTask := s.chunkRepository.
			TermSearch(ctx, query, odm.TermSearchParams{
				IndexName: db.TextSearchIndexName,
				Path:      db.TextSearchPaths,
				Limit:     textK,
			})

		logger.Info("Getting embedding for query", zap.String("queryInput", query))
		emb, err := async.Await(s.embedder.GetEmbedding(ctx, query, embed.WithTask("retrieval.query")))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "embed: %v", err)
		}

		vecTask := s.vectorRepository.
			VectorSearch(ctx, emb, odm.VectorSearchParams{
				IndexName:     db.VectorIndexName,
				Path:          db.VectorPath,
				K:             vecK,
				NumCandidates: 100,
			})

		//----------------------------------------------------------------------
		// 2. Convert each result list → id→rank    (rank ∈ {1,2,…})
		//----------------------------------------------------------------------
		textRanks, cache, err := collectTextSearchRanks(textTask)
		if err != nil {
			logger.Error("text search failed", zap.Error(err))
		}

		vecRanks, err := collectVectorSearchRanks(vecTask)
		if err != nil {
			logger.Error("vector search failed", zap.Error(err))
		}

		//----------------------------------------------------------------------
		// 3. Reciprocal-Rank Fusion
		//     score(id) = Σ  weight_e / (rrfK + rank_e(id))
		//----------------------------------------------------------------------
		combined := make(map[string]float64)
		for id, r := range textRanks {
			combined[id] = textSearchWeight / float64(rrfK+r)
		}
		for id, r := range vecRanks {
			combined[id] += vectorSearchWeight / float64(rrfK+r)
		}

		//----------------------------------------------------------------------
		// 4. Keep the top-N with a min-heap (higher RRF score = better)
		//----------------------------------------------------------------------
		type pair struct {
			id    string
			score float64
		}

		h := ds.NewMinHeap(func(a, b pair) bool { return a.score < b.score })
		for id, sc := range combined {
			h.Push(pair{id, sc})
			if h.Len() > maxChunks {
				h.Pop()
			}
		}

		ids, err := linq.Pipe2(
			linq.FromSlice(ctx, h.ToSortedSlice()),
			linq.Select(func(p pair) string { return p.id }),
			linq.Reverse[string](), // highest score first
		)

		if err != nil {
			logger.Error("Failed to collect top-N chunk IDs", zap.Error(err))
			return nil, status.Errorf(codes.Internal, "collect top-N: %v", err)
		}

		//----------------------------------------------------------------------
		// 5. Materialise the chunks
		//----------------------------------------------------------------------
		return s.fetchChunksByIds(ctx, cache, ids), nil
	})
}

// Returns id→rank (1-based) **and** a cache of the full ChunkModel docs.
func collectTextSearchRanks(
	task <-chan async.Result[[]odm.SearchHit[db.ChunkModel]],
) (map[string]int, map[string]*db.ChunkModel, error) {

	ranks := make(map[string]int) // id → rank
	cache := make(map[string]*db.ChunkModel)

	hits, err := async.Await(task)
	if err != nil {
		return ranks, cache, status.Errorf(codes.Internal, "await text hits: %v", err)
	}

	for i, h := range hits {
		id := h.Doc.Id()
		if _, seen := ranks[id]; !seen { // keep first (best-ranked) hit
			ranks[id] = i + 1  // 1-based rank
			cache[id] = &h.Doc // stash full doc for later
		}
	}
	return ranks, cache, nil
}

// Returns id→rank (1-based) for vector search hits.
func collectVectorSearchRanks(
	task <-chan async.Result[[]odm.SearchHit[db.ChunkAnnModel]],
) (map[string]int, error) {

	ranks := make(map[string]int)

	hits, err := async.Await(task)
	if err != nil {
		return ranks, status.Errorf(codes.Internal, "await vector hits: %v", err)
	}

	for i, h := range hits {
		id := h.Doc.Id()
		if _, seen := ranks[id]; !seen {
			ranks[id] = i + 1
		}
	}
	return ranks, nil
}

func (s *SearchTool) fetchChunksByIds(ctx context.Context, cache map[string]*db.ChunkModel, rankedIds []string) []*db.ChunkModel {

	if len(rankedIds) == 0 {
		return nil
	}

	/* 1. build map[id]Chunk from cache ------------------------ */
	chunkByID := make(map[string]*db.ChunkModel, len(rankedIds))
	var missing []string

	for _, id := range rankedIds {
		if c, ok := cache[id]; ok {
			chunkByID[id] = c
		} else {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		/* 2. fetch all missing in **one** DB round-trip -------- */
		dbChunks, err := async.Await(
			s.chunkRepository.Find(ctx, bson.M{"_id": bson.M{"$in": missing}}, nil, 0, 0),
		)
		if err != nil {
			logger.Error("Failed to fetch chunks from database", zap.Error(err))
			// we still return whatever we already have
		}
		for _, ch := range dbChunks {
			chunkByID[ch.ChunkID] = &ch
		}
	}

	/* 3. assemble slice in ranking order ---------------------- */
	ordered := make([]*db.ChunkModel, 0, len(rankedIds))
	for _, id := range rankedIds {
		if ch, ok := chunkByID[id]; ok {
			ordered = append(ordered, ch)
		} else {
			logger.Info("chunk id missing after lookup", zap.String("id", id))
		}
	}

	return ordered
}

func GroupBySectionWithRank(chunks []*db.ChunkModel) [][]*db.ChunkModel {
	if len(chunks) == 0 {
		return nil
	}

	// Weights for ranking.
	const (
		W              = 1.0  // base weight
		P              = 1.0  // reciprocal-rank exponent
		AdjacencyBonus = 0.15 // bonus * w if WindowIndex-1 was seen in same section
		Lambda         = 0.10 // diminishing returns soft-cap
	)

	type agg struct {
		score     float64
		count     int
		bestRank  int
		seenWin   map[int]struct{}
		collected []*db.ChunkModel // kept in the order encountered (rank order)
	}

	sections := make(map[string]*agg, len(chunks))

	rr := func(rank int) float64 {
		return W / math.Pow(float64(rank), P)
	}

	for i := range chunks {
		ch := chunks[i]
		rank := i + 1

		a := sections[ch.SectionID]
		if a == nil {
			a = &agg{
				bestRank:  rank,
				seenWin:   make(map[int]struct{}),
				collected: make([]*db.ChunkModel, 0, 4),
			}
			sections[ch.SectionID] = a
		}

		w := rr(rank)
		a.score += w
		a.count++
		a.collected = append(a.collected, ch)

		if _, ok := a.seenWin[ch.WindowIndex-1]; ok {
			a.score += AdjacencyBonus * w
		}
		a.seenWin[ch.WindowIndex] = struct{}{}

		if rank < a.bestRank {
			a.bestRank = rank
		}
	}

	type kv struct {
		secID string
		*agg
	}
	order := make([]kv, 0, len(sections))
	for secID, a := range sections {
		// diminishing returns
		if a.count > 1 {
			a.score /= (1 + Lambda*float64(a.count-1))
		}
		order = append(order, kv{secID, a})
	}

	// Sort sections by score desc, then bestRank asc, then count desc
	slices.SortFunc(order, func(x, y kv) int {
		if x.score != y.score {
			if x.score > y.score {
				return -1
			}
			return 1
		}
		if x.bestRank != y.bestRank {
			if x.bestRank < y.bestRank {
				return -1
			}
			return 1
		}
		if x.count != y.count {
			if x.count > y.count {
				return -1
			}
			return 1
		}
		return 0
	})

	out := make([][]*db.ChunkModel, 0, len(order))
	for _, it := range order {
		out = append(out, it.collected)
	}
	return out
}
