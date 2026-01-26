package controller

import (
	"net/http"
	"strings"

	"github.com/SaiNageswarS/agent-boot/agentboot"
	"github.com/SaiNageswarS/agent-boot/llm"
	embedder "github.com/SaiNageswarS/go-api-boot/embed"
	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/appconfig"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/db"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/mcp"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/middleware"
	"go.uber.org/zap"
)

// QueryController handles HTTP requests for query operations
type QueryController struct {
	ccfg               *appconfig.AppConfig
	tool               *mcp.SearchTool
	toolResultRenderer *agentboot.ToolResultRenderer
}

// ProvideQueryController creates a new QueryController instance
// Creates a minimal agent with just the tool (no orchestration components)
// to leverage RunTool's nice wrappers (markdown formatting, summarization, etc.)
func ProvideQueryController(mongo odm.MongoClient, embedder embedder.Embedder, ccfg *appconfig.AppConfig) *QueryController {
	chunkRepository := odm.CollectionOf[db.ChunkModel](mongo, "devinderhealthcare")
	vectorRepository := odm.CollectionOf[db.ChunkAnnModel](mongo, "devinderhealthcare")

	search := mcp.NewSearchTool(chunkRepository, vectorRepository, embedder)
	llmClient := llm.NewAnthropicClient("claude-3-5-haiku-20241022")

	toolResultRenderer := agentboot.NewToolResultRenderer(agentboot.WithSummarizationModel(llmClient))

	return &QueryController{
		tool:               search,
		toolResultRenderer: toolResultRenderer,
		ccfg:               ccfg,
	}
}

func (c *QueryController) HandleQuery(w http.ResponseWriter, r *http.Request) {
	// Get query from URL parameters
	query := r.URL.Query().Get("query")

	// Validate query
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	// Use agent.RunTool which provides nice wrappers (markdown formatting, summarization, etc.)
	// without needing full agent orchestration
	ctx := r.Context()
	toolResultsChan := c.tool.Run(ctx, query)

	formattedPassages, err := c.toolResultRenderer.Render(ctx, query, "", toolResultsChan, c.ccfg.EnableSearchSummarization)
	if err != nil {
		logger.Error("Failed to render tool results", zap.Error(err))
		http.Error(w, "Failed to render tool results", http.StatusInternalServerError)
		return
	}

	// Set response headers for markdown
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Send markdown response directly (concatenate with two newlines)
	markdownResponse := strings.Join(formattedPassages, "\n---\n\n")
	if _, err := w.Write([]byte(markdownResponse)); err != nil {
		logger.Error("Failed to write response", zap.Error(err))
		return
	}

	logger.Info("Query processed successfully", zap.String("query", query))
}

func (c *QueryController) Routes() []server.Route {
	return []server.Route{
		{
			Pattern: "/search",
			Method:  http.MethodGet,
			Handler: middleware.APIKeyAuthMiddleware(c.HandleQuery),
		},
	}
}
