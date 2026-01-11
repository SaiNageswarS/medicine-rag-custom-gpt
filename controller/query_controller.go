package controller

import (
	"encoding/json"
	"net/http"

	"github.com/SaiNageswarS/agent-boot/agentboot"
	"github.com/SaiNageswarS/agent-boot/llm"
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
	tool               *mcp.SearchTool
	toolResultRenderer *agentboot.ToolResultRenderer
}

// ProvideQueryController creates a new QueryController instance
// Creates a minimal agent with just the tool (no orchestration components)
// to leverage RunTool's nice wrappers (markdown formatting, summarization, etc.)
func ProvideQueryController(mongo odm.MongoClient, embedder embed.Embedder) *QueryController {
	chunkRepository := odm.CollectionOf[db.ChunkModel](mongo, "devinderhealthcare")
	vectorRepository := odm.CollectionOf[db.ChunkAnnModel](mongo, "devinderhealthcare")

	search := mcp.NewSearchTool(chunkRepository, vectorRepository, embedder)
	llmClient := llm.NewAnthropicClient("claude-3-5-haiku-20241022")

	toolResultRenderer := agentboot.NewToolResultRenderer(agentboot.WithSummarizationModel(llmClient))

	return &QueryController{
		tool:               search,
		toolResultRenderer: toolResultRenderer,
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

	// Use agent.RunTool which provides nice wrappers (markdown formatting, summarization, etc.)
	// without needing full agent orchestration
	ctx := r.Context()
	toolResultsChan := c.tool.Run(ctx, req.Query)

	formattedResult, err := c.toolResultRenderer.Render(ctx, req.Query, "", toolResultsChan, true)
	if err != nil {
		logger.Error("Failed to render tool results", zap.Error(err))
		http.Error(w, "Failed to render tool results", http.StatusInternalServerError)
		return
	}

	// Create response
	response := model.QueryResponse{
		Query:    req.Query,
		Passages: formattedResult,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode response", zap.Error(err))
		// Note: Can't call http.Error here as headers may already be written
		return
	}

	logger.Info("Query processed successfully", zap.String("query", req.Query))
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
