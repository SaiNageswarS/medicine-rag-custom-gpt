package controller

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SaiNageswarS/agent-boot/agentboot"
	"github.com/SaiNageswarS/agent-boot/llm"
	embedder "github.com/SaiNageswarS/go-api-boot/embed"
	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/open-ai-api/db"
	"github.com/SaiNageswarS/open-ai-api/mcp"
	"github.com/SaiNageswarS/open-ai-api/model"
	"github.com/SaiNageswarS/open-ai-api/templates"
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
func ProvideQueryController(mongo odm.MongoClient, embedder embedder.Embedder) *QueryController {
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

// APIKeyAuthMiddleware validates API key from Authorization header or X-API-Key header
func APIKeyAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("API_KEY")
		if apiKey == "" {
			logger.Error("API_KEY environment variable is not set")
			http.Error(w, "Server configuration error", http.StatusInternalServerError)
			return
		}

		// Check for API key in Authorization header (Bearer token) or X-API-Key header
		authHeader := r.Header.Get("Authorization")
		apiKeyHeader := r.Header.Get("X-API-Key")

		var providedKey string
		if authHeader != "" {
			// Extract token from "Bearer <token>" format
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				providedKey = parts[1]
			} else if len(parts) == 1 {
				// If no Bearer prefix, use the whole header value
				providedKey = parts[0]
			}
		} else if apiKeyHeader != "" {
			providedKey = apiKeyHeader
		}

		if providedKey == "" {
			logger.Error("API key missing from request", zap.String("path", r.URL.Path))
			http.Error(w, "API key required. Provide it in Authorization header (Bearer <key>) or X-API-Key header", http.StatusUnauthorized)
			return
		}

		if providedKey != apiKey {
			logger.Error("Invalid API key provided", zap.String("path", r.URL.Path))
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed to next handler
		next(w, r)
	}
}

// HandlePrivacyPolicy serves the privacy policy page
func (c *QueryController) HandlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	// Parse the embedded template
	tmpl, err := template.ParseFS(templates.FS, "privacy_policy.html")
	if err != nil {
		logger.Error("Failed to parse privacy policy template", zap.Error(err))
		http.Error(w, "Failed to load privacy policy", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := struct {
		LastUpdated string
	}{
		LastUpdated: time.Now().Format("January 2006"),
	}

	// Set response headers
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Execute template
	if err := tmpl.Execute(w, data); err != nil {
		logger.Error("Failed to execute privacy policy template", zap.Error(err))
		// Note: Can't call http.Error here as headers may already be written
		return
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
			Handler: APIKeyAuthMiddleware(c.HandleQuery),
		},
		{
			Pattern: "/privacy-policy",
			Method:  http.MethodGet,
			Handler: c.HandlePrivacyPolicy,
		},
	}
}
