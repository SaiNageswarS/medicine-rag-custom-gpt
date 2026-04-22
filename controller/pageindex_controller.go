package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/mcp"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/middleware"
	"go.uber.org/zap"
)

type PageIndexController struct {
	svc *mcp.PageIndexService
}

func ProvidePageIndexController(mongo odm.MongoClient) *PageIndexController {
	return &PageIndexController{svc: mcp.ProvidePageIndexService(mongo)}
}

// ListDocuments returns all documents with their descriptions (no tree structure).
// GET /documents
func (c *PageIndexController) ListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := c.svc.ListDocuments(r.Context())
	if err != nil {
		logger.Error("Failed to list documents", zap.Error(err))
		http.Error(w, "Failed to list documents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(docs); err != nil {
		logger.Error("Failed to encode documents response", zap.Error(err))
	}
}

// GetDocumentStructure returns the tree structure (titles + summaries) without full text.
// GET /documents/{id}/structure
func (c *PageIndexController) GetDocumentStructure(w http.ResponseWriter, r *http.Request) {
	docID := extractPathParam(r.URL.Path, "/documents/", "/structure")
	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	structure, err := c.svc.GetDocumentStructure(r.Context(), docID)
	if err != nil {
		logger.Error("Failed to find document", zap.String("docId", docID), zap.Error(err))
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(structure); err != nil {
		logger.Error("Failed to encode structure response", zap.Error(err))
	}
}

// GetDocumentContent returns full text for specific line ranges.
// GET /documents/{id}/content?lines=10-25
func (c *PageIndexController) GetDocumentContent(w http.ResponseWriter, r *http.Request) {
	docID := extractPathParam(r.URL.Path, "/documents/", "/content")
	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	linesParam := r.URL.Query().Get("lines")
	if linesParam == "" {
		http.Error(w, "lines parameter is required (e.g. lines=10-25)", http.StatusBadRequest)
		return
	}

	nodes, err := c.svc.GetDocumentContent(r.Context(), docID, linesParam)
	if err != nil {
		logger.Error("Failed to get document content", zap.String("docId", docID), zap.Error(err))
		http.Error(w, "Failed to get content", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		logger.Error("Failed to encode content response", zap.Error(err))
	}
}

func (c *PageIndexController) Routes() []server.Route {
	return []server.Route{
		{
			Pattern: "/documents",
			Method:  http.MethodGet,
			Handler: middleware.APIKeyAuthMiddleware(c.ListDocuments),
		},
		{
			Pattern: "/documents/{id}/structure",
			Method:  http.MethodGet,
			Handler: middleware.APIKeyAuthMiddleware(c.GetDocumentStructure),
		},
		{
			Pattern: "/documents/{id}/content",
			Method:  http.MethodGet,
			Handler: middleware.APIKeyAuthMiddleware(c.GetDocumentContent),
		},
	}
}

// --- helpers ---

// extractPathParam extracts a path segment between a prefix and suffix.
func extractPathParam(path, prefix, suffix string) string {
	after, found := strings.CutPrefix(path, prefix)
	if !found {
		return ""
	}
	before, found := strings.CutSuffix(after, suffix)
	if !found {
		return ""
	}
	return before
}
