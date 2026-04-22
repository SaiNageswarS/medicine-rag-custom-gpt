package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/SaiNageswarS/go-api-boot/config"
	"github.com/SaiNageswarS/go-api-boot/dotenv"
	"github.com/SaiNageswarS/go-api-boot/embed"
	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/appconfig"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/controller"
	mcptools "github.com/SaiNageswarS/medicine-rag-custom-gpt/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const ASSISTANT_INSTRUCTIONS = `You are an assistant to a Homeopathy Doctor with access to a curated materia medica knowledge base. You MUST use the provided tools to answer questions — do NOT rely on your training data or memory for homeopathic medicine information.

## Workflow

1. Call get_current_date to obtain today's date.
2. Call list_documents to identify candidate medicines relevant to the query.
3. For each candidate, call get_document_structure to review its table of contents and section summaries.
4. Call get_page_content for the sections that are relevant to read the actual materia medica text.
5. Synthesize your answer strictly from the retrieved content. If the knowledge base does not contain relevant information, say so explicitly.

## Constraints

- Never answer from memory. Every clinical claim must originate from tool results.
- You may call get_document_structure and get_page_content multiple times for different medicines or sections.

## Output Format

- Begin with the current date.
- Provide a detailed analysis referencing specific materia medica content.
- Cite every claim: medicine name, section title, and line numbers (e.g. "ALUMINA — Mind, Lines 10–25").
- When comparing remedies, use a table or side-by-side format.
- End with a concise summary and differential considerations.`

func main() {
	dotenv.LoadEnv()

	// load config file
	ccfgg := &appconfig.AppConfig{}
	err := config.LoadConfig("config.ini", ccfgg)

	boot, err := server.New().
		GRPCPort(":50051").
		HTTPPort(":8081").
		Provide(ccfgg).
		ProvideFunc(odm.ProvideMongoClient).
		ProvideFunc(embed.ProvideJinaAIEmbeddingClient).
		AddRestController(controller.ProvideQueryController).
		AddRestController(controller.ProvidePrivacyController).
		AddRestController(controller.ProvideMetadataController).
		AddRestController(controller.ProvidePageIndexController).
		WithMCP(&mcp.Implementation{
			Name:    "medicine-rag-pageindex",
			Version: "1.0.0",
		}, &mcp.ServerOptions{
			Instructions: ASSISTANT_INSTRUCTIONS,
		}).
		AddMCPConfigurator(mcptools.ProvidePageIndexMcp).
		Build()

	if err != nil {
		logger.Fatal("Dependency Injection Failed", zap.Error(err))
	}

	ctx := getCancellableContext()
	boot.Serve(ctx)
}

func getCancellableContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sig
		cancel()
	}()

	return ctx
}
