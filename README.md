# RAG API for ChatGPT Custom GPT

A Retrieval-Augmented Generation (RAG) API designed for integration with ChatGPT's custom GPT feature. This API accepts queries and returns relevant passages with source and title information for citation.

## Features

- **RAG Integration**: Retrieves relevant passages based on semantic query matching
- **ChatGPT Compatible**: Designed specifically for ChatGPT custom GPT Actions
- **CORS Enabled**: Supports cross-origin requests from ChatGPT
- **Structured Responses**: Returns passages with `title`, `source`, and `text` fields
- **Health Check**: Built-in health monitoring endpoint
- **Mock Implementation**: Ready-to-use mock data (easily replaceable with vector database)

## API Endpoints

### POST /query

Main RAG endpoint for retrieving passages based on a query. This endpoint is designed to be called by ChatGPT custom GPT Actions.

**Request:**
```json
{
  "query": "machine learning"
}
```

**Response:**
```json
{
  "query": "machine learning",
  "passages": [
    {
      "title": "Introduction to Machine Learning",
      "source": "https://docs.example.com/topics/machine-learning",
      "text": "Machine learning is a subset of artificial intelligence that enables systems to learn from data."
    },
    {
      "title": "Advanced Concepts: Machine Learning",
      "source": "https://docs.example.com/advanced/machine-learning",
      "text": "Advanced topics related to machine learning are covered in this section."
    }
  ]
}
```

### GET /health

Health check endpoint for monitoring API status.

**Response:**
```json
{
  "status": "healthy"
}
```

## Running the API

1. **Install dependencies:**
```bash
go mod download
```

2. **Run the server:**
```bash
go run main.go
```

The server will start on `http://localhost:8081`

## Testing the API

### Using curl:

```bash
# Health check
curl http://localhost:8081/health

# RAG query
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{"query": "RAG systems"}'
```

### Using the built binary:

```bash
# Build
go build -o rag-api main.go

# Run
./rag-api
```

## ChatGPT Custom GPT Configuration

To integrate this RAG API with a custom GPT in ChatGPT, follow these steps:

### Step 1: Deploy the API

Deploy this API to a publicly accessible server (e.g., AWS, Google Cloud, Azure, or any hosting service). Ensure:
- The API is accessible via HTTPS
- CORS is enabled (already included in the code)
- The server has proper SSL/TLS certificates

### Step 2: Configure Custom GPT Action

1. Go to [ChatGPT Custom GPTs](https://chat.openai.com/gpts)
2. Create a new GPT or edit an existing one
3. Navigate to the **Actions** section
4. Click **Create new action**
5. Configure the action as follows:

#### Action Configuration:

- **Name**: `search_knowledge_base` (or any descriptive name)
- **Description**: `Search the knowledge base for relevant information using RAG`

#### Authentication:
- Select **No Auth** (or configure API key if you add authentication)

#### Schema (OpenAPI 3.0):

```yaml
openapi: 3.0.0
info:
  title: RAG API
  version: 1.0.0
servers:
  - url: https://your-domain.com
paths:
  /query:
    post:
      summary: Search knowledge base using RAG
      operationId: searchKnowledgeBase
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                query:
                  type: string
                  description: The search query to find relevant passages
              required:
                - query
      responses:
        '200':
          description: Successful response with passages
          content:
            application/json:
              schema:
                type: object
                properties:
                  query:
                    type: string
                  passages:
                    type: array
                    items:
                      type: object
                      properties:
                        title:
                          type: string
                        source:
                          type: string
                        text:
                          type: string
```

#### Alternative JSON Schema (if using manual configuration):

**Request Body Schema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "The search query to find relevant passages from the knowledge base"
    }
  },
  "required": ["query"]
}
```

**Response Schema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string"
    },
    "passages": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": {
            "type": "string",
            "description": "Title of the passage"
          },
          "source": {
            "type": "string",
            "description": "Source URL or identifier"
          },
          "text": {
            "type": "string",
            "description": "The passage content"
          }
        },
        "required": ["title", "source", "text"]
      }
    }
  }
}
```

### Step 3: Test the Integration

1. Save your custom GPT configuration
2. In the ChatGPT interface, test the action by asking questions
3. The custom GPT will automatically call your RAG API and use the retrieved passages to generate responses

### Example Custom GPT Instructions

Add these instructions to your custom GPT to help it use the RAG API effectively:

```
You are a helpful assistant with access to a knowledge base through a RAG (Retrieval-Augmented Generation) API.

When users ask questions:
1. Use the search_knowledge_base action to retrieve relevant passages
2. Cite sources using the "source" field from the passages
3. Combine information from multiple passages when relevant
4. If no relevant passages are found, let the user know and provide general assistance

Always cite your sources using the format: [Source: title](source URL)
```

## Data Structure

### QueryRequest
- `query` (string, required): The search query

### Passage
- `title` (string): The title of the passage/document
- `source` (string): The source URL or identifier for citation
- `text` (string): The actual passage content

### QueryResponse
- `query` (string): Echo of the original query
- `passages` (array of Passage): Retrieved passages with source and title

## Production Implementation

The current implementation uses mock data for demonstration. To use in production:

1. **Replace Mock Service**: Update `service/query_service.go` to use a real vector database:
   - **Pinecone**: Popular managed vector database
   - **Weaviate**: Open-source vector database
   - **Qdrant**: High-performance vector search engine
   - **FAISS**: Facebook AI Similarity Search (for self-hosted)

2. **Add Embedding Model**: Integrate an embedding model to convert queries and documents to vectors:
   - OpenAI `text-embedding-ada-002` or `text-embedding-3-small`
   - Cohere embedding models
   - Open-source alternatives (Sentence Transformers)

3. **Document Indexing**: Set up a pipeline to:
   - Ingest documents from your knowledge base
   - Generate embeddings for each document chunk
   - Store embeddings and metadata in the vector database

4. **Semantic Search**: Replace the mock keyword matching with:
   - Vector similarity search (cosine similarity, dot product)
   - Hybrid search (combining keyword and semantic search)
   - Re-ranking for better relevance

5. **Add Authentication**: Consider adding API key authentication for production use

## Architecture

```
ChatGPT Custom GPT
    ↓ (HTTP POST)
RAG API (/query endpoint)
    ↓
QueryService (ProcessQuery)
    ↓
Vector Database / Knowledge Base
    ↓
Retrieved Passages
    ↓ (HTTP Response)
ChatGPT Custom GPT (with citations)
```

## Development

### Project Structure

```
.
├── main.go              # Application entry point
├── controller/          # HTTP request handlers
│   └── query_controller.go
├── service/            # Business logic
│   └── query_service.go
├── model/              # Data models
│   └── query.go
└── routes/             # Route registration
    └── routes.go
```

## Notes

- The API includes CORS headers to allow cross-origin requests from ChatGPT
- All endpoints return JSON responses
- Error responses follow standard HTTP status codes
- The mock implementation includes a sample knowledge base for testing
- Replace the mock service with actual vector database integration for production use

## License

This project is provided as-is for integration with ChatGPT custom GPTs.
