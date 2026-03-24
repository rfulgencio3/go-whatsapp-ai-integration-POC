package httpapi

import (
	"embed"
	"net/http"
)

//go:embed docs/openapi.json docs/swagger.html
var swaggerAssets embed.FS

func (h *Handler) handleSwaggerUI(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/swagger" && request.URL.Path != "/swagger/" {
		http.NotFound(responseWriter, request)
		return
	}

	content, err := swaggerAssets.ReadFile("docs/swagger.html")
	if err != nil {
		writeError(responseWriter, http.StatusInternalServerError, "swagger ui is not available")
		return
	}

	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(content)
}

func (h *Handler) handleSwaggerRoute(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/swagger/" {
		h.handleSwaggerUI(responseWriter, request)
		return
	}

	if request.URL.Path == "/swagger/openapi.json" {
		h.handleOpenAPIDocument(responseWriter, request)
		return
	}

	http.NotFound(responseWriter, request)
}

func (h *Handler) handleOpenAPIDocument(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	content, err := swaggerAssets.ReadFile("docs/openapi.json")
	if err != nil {
		writeError(responseWriter, http.StatusInternalServerError, "openapi document is not available")
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(content)
}
