package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SaiNageswarS/go-api-boot/odm"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// PageIndexMcp exposes PageIndex data as MCP tools.
// It implements server.MCPConfigurator.
type PageIndexMcp struct {
	svc *PageIndexService
}

func ProvidePageIndexMcp(mongo odm.MongoClient) *PageIndexMcp {
	return &PageIndexMcp{svc: ProvidePageIndexService(mongo)}
}

// --- MCP input types ---

type listDocumentsInput struct{}
type getCurrentDateInput struct{}

type getDocumentStructureInput struct {
	DocID string `json:"doc_id" jsonschema:"required" jsonschema_description:"The document ID (e.g. ACONITUM)"`
}

type getPageContentInput struct {
	DocID string `json:"doc_id" jsonschema:"required" jsonschema_description:"The document ID (e.g. ACONITUM)"`
	Lines string `json:"lines" jsonschema:"required" jsonschema_description:"Line range to fetch. Examples: 10-25 or 5,12,30 or 19-34,321-349"`
}

// ConfigureMCP registers the PageIndex tools and utility tools on the MCP server.
func (m *PageIndexMcp) ConfigureMCP(s *gomcp.Server) {
	gomcp.AddTool(s, &gomcp.Tool{
		Name:        "get_current_date",
		Description: "Returns the current date and time. Use this to include the date in your response.",
		Annotations: &gomcp.ToolAnnotations{ReadOnlyHint: true},
	}, m.handleGetCurrentDate)

	gomcp.AddTool(s, &gomcp.Tool{
		Name:        "list_documents",
		Description: "List all available medicine documents with their descriptions. Call this first to discover what medicines are available.",
		Annotations: &gomcp.ToolAnnotations{ReadOnlyHint: true},
	}, m.handleListDocuments)

	gomcp.AddTool(s, &gomcp.Tool{
		Name:        "get_document_structure",
		Description: "Get the hierarchical table of contents of a medicine document, with section titles, summaries, and line numbers. Text content is stripped to save tokens. Use the line numbers to fetch specific sections with get_page_content.",
		Annotations: &gomcp.ToolAnnotations{ReadOnlyHint: true},
	}, m.handleGetDocumentStructure)

	gomcp.AddTool(s, &gomcp.Tool{
		Name:        "get_page_content",
		Description: "Get the full text content for specific line ranges of a medicine document. Use line numbers from get_document_structure to specify which sections to read.",
		Annotations: &gomcp.ToolAnnotations{ReadOnlyHint: true},
	}, m.handleGetPageContent)
}

// --- Tool handlers ---

func (m *PageIndexMcp) handleListDocuments(ctx context.Context, req *gomcp.CallToolRequest, _ listDocumentsInput) (*gomcp.CallToolResult, any, error) {
	docs, err := m.svc.ListDocuments(ctx)
	if err != nil {
		return nil, nil, err
	}

	jsonBytes, err := json.Marshal(docs)
	if err != nil {
		return nil, nil, err
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: string(jsonBytes)}},
	}, nil, nil
}

func (m *PageIndexMcp) handleGetDocumentStructure(ctx context.Context, req *gomcp.CallToolRequest, input getDocumentStructureInput) (*gomcp.CallToolResult, any, error) {
	structure, err := m.svc.GetDocumentStructure(ctx, input.DocID)
	if err != nil {
		return nil, nil, err
	}

	jsonBytes, err := json.Marshal(structure)
	if err != nil {
		return nil, nil, err
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: string(jsonBytes)}},
	}, nil, nil
}

func (m *PageIndexMcp) handleGetPageContent(ctx context.Context, req *gomcp.CallToolRequest, input getPageContentInput) (*gomcp.CallToolResult, any, error) {
	nodes, err := m.svc.GetDocumentContent(ctx, input.DocID, input.Lines)
	if err != nil {
		res := &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "Invalid lines format. Use 10-25 or 5,12,30"}},
			IsError: true,
		}
		return res, nil, nil
	}

	jsonBytes, err := json.Marshal(nodes)
	if err != nil {
		return nil, nil, err
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: string(jsonBytes)}},
	}, nil, nil
}

func (m *PageIndexMcp) handleGetCurrentDate(_ context.Context, _ *gomcp.CallToolRequest, _ getCurrentDateInput) (*gomcp.CallToolResult, any, error) {
	now := time.Now().Format("2 January 2006, Monday, 3:04 PM MST")
	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: now}},
	}, nil, nil
}
