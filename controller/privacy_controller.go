package controller

import (
	"html/template"
	"net/http"
	"time"

	"github.com/SaiNageswarS/go-api-boot/logger"
	"github.com/SaiNageswarS/go-api-boot/server"
	"github.com/SaiNageswarS/medicine-rag-custom-gpt/templates"
	"go.uber.org/zap"
)

type PrivacyController struct {
}

func ProvidePrivacyController() *PrivacyController {
	return &PrivacyController{}
}

func (pc *PrivacyController) HandlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	// Parse the embedded template
	tmpl, err := template.ParseFS(templates.FS, "privacy_policy.html")
	if err != nil {
		logger.Error("Failed to parse privacy policy template", zap.Error(err))
		http.Error(w, "Failed to load privacy policy", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := struct {
		LastUpdated string
	}{
		LastUpdated: time.Now().Format("January 2006"),
	}

	// Set response headers
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Execute template
	if err := tmpl.Execute(w, data); err != nil {
		logger.Error("Failed to execute privacy policy template", zap.Error(err))
		// Note: Can't call http.Error here as headers may already be written
		return
	}
}

func (pc *PrivacyController) Routes() []server.Route {
	return []server.Route{
		{
			Pattern: "/privacy-policy",
			Method:  http.MethodGet,
			Handler: pc.HandlePrivacyPolicy,
		},
	}
}
