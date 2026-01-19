package controller

import (
	"encoding/json"
	"net/http"

	"github.com/SaiNageswarS/go-api-boot/odm"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/db"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/middleware"
)

type MetadataController struct {
	mongo *odm.MongoClient
}

func ProvideMetadataController(mongo odm.MongoClient) *MetadataController {
	return &MetadataController{
		mongo: &mongo,
	}
}

func (mc *MetadataController) ListSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var distinctSources []string
	err := odm.CollectionOf[db.ChunkModel](*mc.mongo, "devinderhealthcare").DistinctInto(ctx, "sourceUri", nil, &distinctSources)
	if err != nil {
		http.Error(w, "Failed to fetch sources", http.StatusInternalServerError)
		return
	}

	// Return the list of distinct sources as JSON
	w.Header().Set("Content-Type", "application/json")
	response := map[string][]string{"sources": distinctSources}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (mc *MetadataController) Routes() []server.Route {
	return []server.Route{
		{
			Pattern: "/metadata/sources",
			Method:  http.MethodGet,
			Handler: middleware.APIKeyAuthMiddleware(mc.ListSources),
		},
	}
}
