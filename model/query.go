package model

// QueryRequest represents the incoming query request from ChatGPT custom GPT
type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// Passage represents a single passage with source and title for RAG retrieval
type Passage struct {
	Title  string `json:"title"`  // Title of the passage/document
	Source string `json:"source"` // Source URL or identifier
	Text   string `json:"text"`   // The actual passage content
}

// QueryResponse represents the response containing passages for ChatGPT custom GPT
type QueryResponse struct {
	Query    string    `json:"query"`    // Echo back the query
	Passages []Passage `json:"passages"` // Retrieved passages with source and title
}
