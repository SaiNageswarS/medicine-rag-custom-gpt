package controller

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/go-collection-boot/async"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/db"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/middleware"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

type PageIndexController struct {
	repo odm.OdmCollectionInterface[db.PageIndexDocModel]
}

func ProvidePageIndexController(mongo odm.MongoClient) *PageIndexController {
	repo := odm.CollectionOf[db.PageIndexDocModel](mongo, "devinderhealthcare")
	return &PageIndexController{repo: repo}
}

// ListDocuments returns all documents with their descriptions (no tree structure).
// GET /documents
func (c *PageIndexController) ListDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	docs, err := async.Await(c.repo.Find(ctx, bson.M{}, nil, 0, 0))
	if err != nil {
		logger.Error("Failed to list documents", zap.Error(err))
		http.Error(w, "Failed to list documents", http.StatusInternalServerError)
		return
	}

	type docSummary struct {
		DocID          string `json:"docId"`
		DocName        string `json:"docName"`
		DocDescription string `json:"docDescription"`
		LineCount      int    `json:"lineCount"`
	}

	result := make([]docSummary, 0, len(docs))
	for _, d := range docs {
		result = append(result, docSummary{
			DocID:          d.DocID,
			DocName:        d.DocName,
			DocDescription: d.DocDescription,
			LineCount:      d.LineCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		logger.Error("Failed to encode documents response", zap.Error(err))
	}
}

// GetDocumentStructure returns the tree structure (titles + summaries) without full text.
// GET /documents/{id}/structure
func (c *PageIndexController) GetDocumentStructure(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := extractPathParam(r.URL.Path, "/documents/", "/structure")
	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	doc, err := async.Await(c.repo.FindOneByID(ctx, docID))
	if err != nil {
		logger.Error("Failed to find document", zap.String("docId", docID), zap.Error(err))
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	// Strip text fields from the structure to save tokens
	stripped := stripText(doc.Structure)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stripped); err != nil {
		logger.Error("Failed to encode structure response", zap.Error(err))
	}
}

// GetDocumentContent returns full text for specific line ranges.
// GET /documents/{id}/content?lines=10-25
func (c *PageIndexController) GetDocumentContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	minLine, maxLine, err := parseLineRange(linesParam)
	if err != nil {
		http.Error(w, "Invalid lines format. Use lines=10-25 or lines=5,12,30", http.StatusBadRequest)
		return
	}

	doc, err := async.Await(c.repo.FindOneByID(ctx, docID))
	if err != nil {
		logger.Error("Failed to find document", zap.String("docId", docID), zap.Error(err))
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	// Traverse tree and collect nodes whose line_num falls within the range
	type nodeContent struct {
		Title   string `json:"title"`
		LineNum int    `json:"line_num"`
		Text    string `json:"text"`
	}

	var results []nodeContent
	var traverse func(nodes []db.PageIndexNode)
	traverse = func(nodes []db.PageIndexNode) {
		for _, n := range nodes {
			if n.LineNum >= minLine && n.LineNum <= maxLine {
				results = append(results, nodeContent{
					Title:   n.Title,
					LineNum: n.LineNum,
					Text:    n.Text,
				})
			}
			if len(n.Nodes) > 0 {
				traverse(n.Nodes)
			}
		}
	}
	traverse(doc.Structure)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
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
// e.g. extractPathParam("/documents/ACONITUM/structure", "/documents/", "/structure") => "ACONITUM"
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

// parseLineRange parses line specifications into min and max line numbers.
// Supported formats:
//   - "10-25"           single range
//   - "5,12,30"         comma-separated line numbers
//   - "19-34,321-349"   comma-separated ranges
func parseLineRange(s string) (int, int, error) {
	min, max := int(^uint(0)>>1), 0
	for _, seg := range strings.Split(s, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.Contains(seg, "-") {
			parts := strings.SplitN(seg, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return 0, 0, err
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return 0, 0, err
			}
			if start < min {
				min = start
			}
			if end > max {
				max = end
			}
		} else {
			n, err := strconv.Atoi(seg)
			if err != nil {
				return 0, 0, err
			}
			if n < min {
				min = n
			}
			if n > max {
				max = n
			}
		}
	}
	return min, max, nil
}

// stripText returns a copy of the tree with Text fields removed.
func stripText(nodes []db.PageIndexNode) []db.PageIndexNode {
	out := make([]db.PageIndexNode, len(nodes))
	for i, n := range nodes {
		out[i] = db.PageIndexNode{
			Title:         n.Title,
			NodeID:        n.NodeID,
			LineNum:       n.LineNum,
			Summary:       n.Summary,
			PrefixSummary: n.PrefixSummary,
			// Text intentionally omitted
		}
		if len(n.Nodes) > 0 {
			out[i].Nodes = stripText(n.Nodes)
		}
	}
	return out
}
