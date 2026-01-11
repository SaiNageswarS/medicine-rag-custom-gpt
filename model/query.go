package model

// QueryRequest represents the incoming query request from ChatGPT custom GPT
type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// QueryResponse represents the response containing passages for ChatGPT custom GPT
type QueryResponse struct {
	Query    string   `json:"query"`    // Echo back the query
	Passages []string `json:"passages"` // Retrieved passages with source and title
}
