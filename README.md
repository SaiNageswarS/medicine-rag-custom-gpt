# RAG API for ChatGPT Custom GPT

A Retrieval-Augmented Generation (RAG) API designed for integration with ChatGPT's custom GPT feature. This API accepts queries and returns relevant passages with source and title information for citation.

## Features

- **RAG Integration**: Retrieves relevant passages based on semantic query matching
- **ChatGPT Compatible**: Designed specifically for ChatGPT custom GPT Actions
- **API Key Authentication**: Secure API access with configurable API keys
- **Privacy Policy**: Built-in privacy policy endpoint for OpenAI compliance
- **Structured Responses**: Returns formatted passages with source and title information
- **Vector Database**: Integrated with MongoDB for vector similarity search

## API Endpoints

### POST /query

Main RAG endpoint for retrieving passages based on a query. This endpoint is designed to be called by ChatGPT custom GPT Actions.

**Authentication:** Required - API key must be provided in `Authorization: Bearer <key>` header or `X-API-Key` header.

**Request:**
```json
{
  "query": "homeopathy remedies for headache"
}
```

**Response:**
```json
{
  "query": "homeopathy remedies for headache",
  "passages": [
    "**Belladonna**\n\nSource: Homeopathy Materia Medica\n\nBelladonna is indicated for sudden, intense headaches with throbbing pain, especially in the temples...",
    "**Bryonia**\n\nSource: Homeopathy Materia Medica\n\nBryonia is useful for headaches that worsen with movement and improve with rest..."
  ]
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request payload or missing query
- `401 Unauthorized`: Missing or invalid API key
- `500 Internal Server Error`: Server error processing the request

### GET /privacy-policy

Privacy policy endpoint that serves an HTML page with the API's privacy policy. This endpoint is required for OpenAI Custom GPT public actions.

**Authentication:** Not required

**Response:** HTML page with privacy policy content

## Configuration

### Environment Variables

Create a `.env` file in the project root with the following variables:

```bash
# Required: API key for authenticating requests
API_KEY=your-secret-api-key-here

# MongoDB connection string (if using MongoDB)
MONGO_URI=mongodb://localhost:27017

# Other environment variables as needed by your dependencies
```

**Important:** The `API_KEY` environment variable is required. The API will return a 500 error if it's not set.

## Running the API

1. **Install dependencies:**
```bash
go mod download
```

2. **Set environment variables:**
```bash
export API_KEY="your-secret-api-key-here"
# Or use a .env file (loaded automatically via dotenv)
```

3. **Run the server:**
```bash
go run main.go
```

The server will start on `http://localhost:8081`

## Testing the API

### Using curl:

```bash
# Privacy policy (no authentication required)
curl http://localhost:8081/privacy-policy

# RAG query with API key in X-API-Key header
curl -X POST http://localhost:8081/query \
  -H "X-API-Key: your-secret-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"query": "homeopathy remedies for cold"}'

# RAG query with API key in Authorization header
curl -X POST http://localhost:8081/query \
  -H "Authorization: Bearer your-secret-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"query": "homeopathy remedies for cold"}'
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
- Select **API Key** authentication
- **Auth Type**: API Key
- **Header Name**: `X-API-Key` (or use `Authorization` header with Bearer token)
- **API Key**: Enter the API key you configured in your environment (`API_KEY`)

#### Schema:

**Recommended:** Use the provided `openapi-schema.json` file directly. It includes:
- Complete OpenAPI 3.1.0 specification
- API key authentication configuration
- Privacy policy URL
- All request/response schemas

1. Copy the contents of `openapi-schema.json` from this repository
2. Update the `servers.url` field with your deployed API URL
3. Update the `x-privacy-policy-url` in the `info` section if needed
4. Paste the schema into ChatGPT's action configuration

**Manual Configuration (Alternative):**

If you prefer to configure manually, use the following schema:

**Request Body Schema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Query string to search regarding the health condition"
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
      "type": "string",
      "description": "Echo of the original query"
    },
    "passages": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Retrieved passages with source and title information (formatted as markdown)"
    }
  }
}
```

**Security Scheme:**
- **Type**: API Key
- **In**: Header
- **Name**: `X-API-Key`
- **Description**: API key for authentication. Can also be provided in Authorization header as Bearer token.

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
- `query` (string, required): The search query regarding the health condition

### QueryResponse
- `query` (string): Echo of the original query
- `passages` (array of string): Retrieved passages formatted as markdown strings, each containing source and title information

**Note:** The passages are returned as formatted markdown strings that include the remedy name, source information, and relevant text content. This format is optimized for ChatGPT's display and citation capabilities.

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

4. **Semantic Search**: The current implementation uses:
   - Vector similarity search via MongoDB Atlas Vector Search
   - Hybrid search (combining keyword and semantic search)
   - Embedding-based retrieval using Jina AI embeddings

5. **Authentication**: API key authentication is already implemented. Ensure you:
   - Set a strong `API_KEY` environment variable
   - Use HTTPS in production
   - Rotate API keys periodically
   - Monitor API usage for security

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

## Security Notes

- **API Key**: Always use a strong, randomly generated API key. Never commit API keys to version control.
- **HTTPS**: Always use HTTPS in production. The API should only be accessible over encrypted connections.
- **Environment Variables**: Store sensitive configuration (API keys, database credentials) in environment variables, not in code.
- **Privacy Policy**: The `/privacy-policy` endpoint is required for OpenAI Custom GPT public actions and must be publicly accessible.

## Notes

- The API includes CORS headers to allow cross-origin requests from ChatGPT
- All endpoints return JSON responses (except `/privacy-policy` which returns HTML)
- Error responses follow standard HTTP status codes
- The implementation uses MongoDB with vector search capabilities
- Passages are formatted as markdown strings for optimal ChatGPT integration
- The OpenAPI schema file (`openapi-schema.json`) is ready to use for ChatGPT Custom GPT configuration

## License

This project is provided as-is for integration with ChatGPT custom GPTs.
