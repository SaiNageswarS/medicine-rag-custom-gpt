# Homeopathy Knowledge Base — RAG API for ChatGPT Custom GPT

A Retrieval-Augmented Generation API for homeopathy materia medica, designed for integration with ChatGPT Custom GPT Actions. Uses [PageIndex](https://github.com/VectifyAI/PageIndex) for vectorless, reasoning-based retrieval.

## Architecture

```
┌─────────────────┐         ┌──────────────────────────────────────┐
│  Python Ingestion│         │        Go API Server                 │
│                  │         │                                      │
│  PageIndex       │         │  GET /documents                      │
│  md_to_tree()   ─┼──write──▶  GET /documents/{id}/structure      │
│                  │  Mongo  │  GET /documents/{id}/content?lines=  │
│  articles/*.md   │         │                                      │
└─────────────────┘         │  GET /search        (hybrid search)  │
                            └──────────┬───────────────────────────┘
                                       │
                            ┌──────────▼───────────────────────────┐
                            │  ChatGPT Custom GPT                  │
                            │                                      │
                            │  1. /documents → pick medicines      │
                            │  2. /documents/{id}/structure →      │
                            │     read summaries, pick sections    │
                            │  3. /documents/{id}/content →        │
                            │     get full text for grounding      │
                            └──────────────────────────────────────┘
```

### Retrieval Approaches

**PageIndex (reasoning-based, vectorless)** — ChatGPT navigates a hierarchical tree index with AI-generated summaries to locate relevant sections. No vector DB or embedding needed for this path.

**Hybrid Search (vector + keyword)** — Traditional RAG using MongoDB Atlas Vector Search with Jina AI embeddings, combined with BM25 keyword search via Reciprocal Rank Fusion.

## API Endpoints

| Endpoint | Auth | Description |
|---|---|---|
| `GET /documents` | Yes | List all medicines with AI-generated descriptions |
| `GET /documents/{id}/structure` | Yes | Tree structure with section titles and summaries |
| `GET /documents/{id}/content?lines=10-25` | Yes | Full text for specific line ranges |
| `GET /search?query=...` | Yes | Hybrid vector + keyword search |
| `GET /metadata/sources` | Yes | List indexed sources |
| `GET /privacy-policy` | No | Privacy policy (required by OpenAI) |

**Authentication:** API key via `X-API-Key` header or `Authorization: Bearer <key>`.

## Quick Start

### 1. Run the Go API server

```bash
# Set environment variables (or use .env file)
export API_KEY="your-secret-api-key"
export MONGO_URI="mongodb+srv://..."

go mod download
go run main.go
# Server starts on http://localhost:8081
```

### 2. Ingest articles with PageIndex

```bash
# Install Python dependencies
pip3 install --upgrade pageindex python-dotenv pymongo

# Set env vars
export OPENAI_API_KEY="your-openai-key"
export MONGO_URI="mongodb+srv://..."

# Build index on all articles and write to MongoDB
python3 ingestion/build_pageindex.py --with-summaries --with-text

# Or process a single article
python3 ingestion/build_pageindex.py --with-summaries --with-text --single ACONITUM.md

# Or test locally without MongoDB
python3 ingestion/build_pageindex.py --json-only --single ACONITUM.md
```

### 3. Configure ChatGPT Custom GPT

1. Create a Custom GPT at [chat.openai.com/gpts](https://chat.openai.com/gpts)
2. In **Actions**, paste the contents of `openapi-schema.json`
3. Update the `servers.url` to your deployed API URL
4. Set the API key under Authentication → API Key → `X-API-Key`

## Ingestion via GitHub Actions

The ingestion pipeline can be triggered manually via GitHub Actions:

1. Go to **Actions → Build PageIndex & Ingest to MongoDB**
2. Click **Run workflow**
3. Choose `all` or a specific filename (e.g. `ACONITUM.md`)

The workflow reads `OPENAI_API_KEY` and `MONGO_URI` from repository secrets. Ingestion is idempotent — re-running on the same file overwrites the existing index.

## Project Structure

```
.
├── main.go                  # Entry point, DI wiring
├── controller/
│   ├── pageindex_controller.go  # /documents endpoints (PageIndex tree navigation)
│   ├── query_controller.go      # /search endpoint (hybrid search)
│   ├── metadata_controller.go   # /metadata/sources
│   └── privacy_controller.go    # /privacy-policy
├── db/
│   ├── pageindex_model.go       # PageIndex document + node tree model
│   ├── chunk_model.go           # Chunk model for hybrid search
│   └── chunk_ann_model.go       # Vector embedding model
├── mcp/
│   └── search.go                # Hybrid search (vector + BM25 + RRF)
├── ingestion/
│   ├── build_pageindex.py       # PageIndex tree builder + MongoDB ingester
│   ├── add_headings.py          # Markdown heading normalizer
│   └── split_materia_medica.py  # Splits source book into per-medicine files
├── articles/                    # Markdown articles (one per medicine)
├── openapi-schema.json          # OpenAPI spec for ChatGPT Custom GPT
├── .github/workflows/
│   └── build-pageindex.yml      # Manual workflow for ingestion
└── config.ini                   # App config
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `API_KEY` | Yes | API key for authenticating requests |
| `MONGO_URI` | Yes | MongoDB connection string |
| `OPENAI_API_KEY` | Ingestion only | OpenAI key for PageIndex summary generation |
| `JINA_API_KEY` | Hybrid search | Jina AI key for embeddings |

## Security

- API key authentication on all data endpoints
- HTTPS required in production
- Ingestion script validates filenames to prevent path traversal
- Store secrets in environment variables, never in code
