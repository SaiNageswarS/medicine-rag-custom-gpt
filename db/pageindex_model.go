package db

// PageIndexDocModel stores a PageIndex tree structure for a single medicine article.
// Written by the Python ingestion pipeline, read by the Go API.
type PageIndexDocModel struct {
	DocID          string          `json:"docId" bson:"_id"`       // e.g. "ACONITUM"
	DocName        string          `json:"docName" bson:"docName"` // e.g. "ACONITUM"
	DocDescription string          `json:"docDescription" bson:"docDescription"`
	LineCount      int             `json:"lineCount" bson:"lineCount"`
	Structure      []PageIndexNode `json:"structure" bson:"structure"`
}

// PageIndexNode is a single node in the PageIndex tree.
type PageIndexNode struct {
	Title         string          `json:"title" bson:"title"`
	NodeID        string          `json:"node_id" bson:"node_id"`
	LineNum       int             `json:"line_num" bson:"line_num"`
	Summary       string          `json:"summary,omitempty" bson:"summary,omitempty"`
	PrefixSummary string          `json:"prefix_summary,omitempty" bson:"prefix_summary,omitempty"`
	Text          string          `json:"text,omitempty" bson:"text,omitempty"`
	Nodes         []PageIndexNode `json:"nodes,omitempty" bson:"nodes,omitempty"`
}

func (m PageIndexDocModel) Id() string             { return m.DocID }
func (m PageIndexDocModel) CollectionName() string { return "pageindex_docs" }
