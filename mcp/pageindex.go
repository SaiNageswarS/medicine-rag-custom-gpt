package mcp

import (
	"context"
	"strconv"
	"strings"

	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-collection-boot/async"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// DocSummary is a lightweight representation of a PageIndex document.
type DocSummary struct {
	DocID          string `json:"doc_id"`
	DocName        string `json:"doc_name"`
	DocDescription string `json:"doc_description"`
	LineCount      int    `json:"line_count"`
}

// NodeContent is a single section's text extracted by line range.
type NodeContent struct {
	Title   string `json:"title"`
	LineNum int    `json:"line_num"`
	Text    string `json:"text"`
}

// PageIndexService holds the shared data-access logic used by both the
// REST controller and the MCP configurator.
type PageIndexService struct {
	Repo odm.OdmCollectionInterface[db.PageIndexDocModel]
}

func ProvidePageIndexService(mongo odm.MongoClient) *PageIndexService {
	repo := odm.CollectionOf[db.PageIndexDocModel](mongo, "devinderhealthcare")
	return &PageIndexService{Repo: repo}
}

// ListDocuments returns summaries for every document in the collection.
func (s *PageIndexService) ListDocuments(ctx context.Context) ([]DocSummary, error) {
	docs, err := async.Await(s.Repo.Find(ctx, bson.M{}, nil, 0, 0))
	if err != nil {
		return nil, err
	}

	result := make([]DocSummary, 0, len(docs))
	for _, d := range docs {
		result = append(result, DocSummary{
			DocID:          d.DocID,
			DocName:        d.DocName,
			DocDescription: d.DocDescription,
			LineCount:      d.LineCount,
		})
	}
	return result, nil
}

// GetDocumentStructure returns the tree for a document with text stripped.
func (s *PageIndexService) GetDocumentStructure(ctx context.Context, docID string) ([]db.PageIndexNode, error) {
	doc, err := async.Await(s.Repo.FindOneByID(ctx, docID))
	if err != nil {
		return nil, err
	}
	return StripText(doc.Structure), nil
}

// GetDocumentContent returns text nodes whose line numbers fall within the
// given range specification (e.g. "10-25" or "5,12,30").
func (s *PageIndexService) GetDocumentContent(ctx context.Context, docID, lines string) ([]NodeContent, error) {
	minLine, maxLine, err := ParseLineRange(lines)
	if err != nil {
		return nil, err
	}

	doc, err := async.Await(s.Repo.FindOneByID(ctx, docID))
	if err != nil {
		return nil, err
	}

	return CollectNodes(doc.Structure, minLine, maxLine), nil
}

// --- Shared helpers ---

// StripText returns a copy of the tree with Text fields removed.
func StripText(nodes []db.PageIndexNode) []db.PageIndexNode {
	out := make([]db.PageIndexNode, len(nodes))
	for i, n := range nodes {
		out[i] = db.PageIndexNode{
			Title:         n.Title,
			NodeID:        n.NodeID,
			LineNum:       n.LineNum,
			Summary:       n.Summary,
			PrefixSummary: n.PrefixSummary,
		}
		if len(n.Nodes) > 0 {
			out[i].Nodes = StripText(n.Nodes)
		}
	}
	return out
}

// CollectNodes traverses the tree and returns nodes whose LineNum is in [min, max].
func CollectNodes(nodes []db.PageIndexNode, minLine, maxLine int) []NodeContent {
	var results []NodeContent
	var traverse func([]db.PageIndexNode)
	traverse = func(ns []db.PageIndexNode) {
		for _, n := range ns {
			if n.LineNum >= minLine && n.LineNum <= maxLine {
				results = append(results, NodeContent{
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
	traverse(nodes)
	return results
}

// ParseLineRange parses line specifications into min and max line numbers.
// Supported formats: "10-25", "5,12,30", "19-34,321-349".
func ParseLineRange(s string) (int, int, error) {
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
