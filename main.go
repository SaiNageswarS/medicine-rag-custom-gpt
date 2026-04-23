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
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/middleware"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const ASSISTANT_INSTRUCTIONS = `You are a homeopathy assistant to Qualified Homeopathic Physicians. You have access to a curated materia medica knowledge base. You MUST use the provided tools to look up remedy information before responding. Do NOT answer from memory or training data. Do not use web browsing.

## Workflow

1. Call get_current_date to obtain today's date.
2. Call list_documents to see all available medicines — do this on every new question about remedies, symptoms, or medicines.
3. Call get_document_structure on relevant medicines to review their section tree and summaries.
4. Call get_page_content to read full text of matching sections.
5. Synthesize your answer strictly from the retrieved content. If the knowledge base does not contain relevant information, say so explicitly.

You may call get_document_structure and get_page_content multiple times for different medicines or sections. Give small/rare remedies equal weight as polychrests. Deprioritize Carcinosin unless clear keynotes are present.

## Clinical Reasoning

- Hierarchy: Mentals > Generals > Particulars > Modalities.
- Reasoning per Hahnemann, Vithoulkas, Ghegas.
- Show step-by-step reasoning. Extract Ghegas practical tips when present. Note miasmatic stage.
- General medical knowledge is acceptable for medical terms and case framing only.

## Case-Taking

- Use open neutral questions, Ghegas style (free anamnesis, contradiction flagging, essence hints).
- If patient mentions overthinking/anxiety/stress: explore thought themes, fears, triggers with bilingual follow-up before suggesting remedies.
- Use your tools in parallel while asking questions.

## Output Format

- Date every interaction: "Date: DD-MM-YYYY" (IST) — use get_current_date for this.
- Output order: symptom summary → remedies + indications → most probable remedy ✅ → differentials 🧩 table → Ghegas notes 💬 → citations 📚.
- Cite every claim: medicine name and section title (e.g. "ALUMINA — Mind").
- When comparing remedies, use a table or side-by-side format.
- End with a concise summary and differential considerations.
- Suggest potency only if explicitly asked.`

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
		WithMCPMiddleware(middleware.APIKeyAuthHandler).
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
