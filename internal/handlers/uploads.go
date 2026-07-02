package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/storage"

	"github.com/go-chi/chi/v5"
)

// UploadHandler handles file uploads and serving.
type UploadHandler struct {
	fileStore *storage.FileStore
}

// NewUploadHandler creates a new upload handler.
func NewUploadHandler(fs *storage.FileStore) *UploadHandler {
	return &UploadHandler{fileStore: fs}
}

// RegisterRoutes registers upload routes.
func (h *UploadHandler) RegisterRoutes(r chi.Router) {
	r.Post("/upload", h.Upload)
}

// Upload handles file uploads (images only, max 5MB).
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxFileSize+1024)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File is required")
		return
	}
	defer file.Close()

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	if !storage.IsAllowedContentType(contentType) {
		writeError(w, http.StatusBadRequest, "Only images are allowed (JPEG, PNG, WebP, GIF)")
		return
	}

	entityType := r.FormValue("entity_type")
	if entityType == "" {
		entityType = "general"
	}

	storedFilename, relativePath, err := h.fileStore.Save(tenantID, entityType, header.Filename, file)
	if err != nil {
		log.Printf("Error uploading file: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"filename":      storedFilename,
		"original_name": header.Filename,
		"path":          relativePath,
		"content_type":  contentType,
		"size":          header.Size,
	})
}
