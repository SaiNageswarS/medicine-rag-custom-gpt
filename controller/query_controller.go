package controller

import (
	"encoding/json"
	"net/http"

	"github.com/SaiNageswarS/go-api-boot/embed"
	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/open-ai-api/db"
	"github.com/SaiNageswarS/open-ai-api/mcp"
	"github.com/SaiNageswarS/open-ai-api/model"
	"go.uber.org/zap"
)

// QueryController handles HTTP requests for query operations
type QueryController struct {
	search *mcp.SearchTool
}

// ProvideQueryController creates a new QueryController instance
func ProvideQueryController(mongo odm.MongoClient, embedder embed.Embedder) *QueryController {
	chunkRepository := odm.CollectionOf[db.ChunkModel](mongo, "devinderhealthcare")
	vectorRepository := odm.CollectionOf[db.ChunkAnnModel](mongo, "devinderhealthcare")

	return &QueryController{
		search: mcp.NewSearchTool(chunkRepository, vectorRepository, embedder),
	}
}

// HandleQuery handles POST requests to process queries from ChatGPT custom GPT
func (c *QueryController) HandleQuery(w http.ResponseWriter, r *http.Request) {
	// Decode request body
	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate query
	if req.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	// Process query using RAG service
	passages := c.search.Run(r.Context(), req.Query)

	for passage := range passages {
		logger.Info("Passage", zap.String("title", passage.Title))
	}

	// Create response
	response := model.QueryResponse{
		Query:    req.Query,
		Passages: []model.Passage{},
	}

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("Query processed successfully", zap.String("query", req.Query), zap.Int("passages_count", len(passages)))
}

func (c *QueryController) Routes() []server.Route {
	return []server.Route{
		{
			Pattern: "/query",
			Method:  http.MethodPost,
			Handler: c.HandleQuery,
		},
	}
}
