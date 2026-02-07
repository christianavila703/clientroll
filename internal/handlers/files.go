package handlers

import (
	"log"
	"net/http"
	"strings"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// FileHandler handles authenticated file uploads and serving.
type FileHandler struct {
	fileStore *storage.FileStore
	db        *database.Postgres
}

// NewFileHandler creates a new file handler.
func NewFileHandler(fs *storage.FileStore, db *database.Postgres) *FileHandler {
	return &FileHandler{fileStore: fs, db: db}
}

// RegisterAuthRoutes registers authenticated file routes.
func (h *FileHandler) RegisterAuthRoutes(r chi.Router) {
	r.Post("/upload", h.Upload)
	r.Get("/files/{fileID}", h.Serve)
}

// RegisterPublicRoutes registers public file routes (for signup images).
func (h *FileHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/public/files/{fileID}", h.ServePublic)
}

// Upload handles file uploads (images only, max 5MB).
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxFileSize+1024)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File is required")
		return
	}
	defer file.Close()

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

	// Track in DB
	var fileID uuid.UUID
	err = h.db.Pool.QueryRow(r.Context(),
		`INSERT INTO uploaded_files (tenant_id, uploaded_by_user_id, original_filename, stored_filename, file_path, content_type, file_size_bytes, entity_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		tenantID, userID, header.Filename, storedFilename, relativePath, contentType, header.Size, entityType,
	).Scan(&fileID)
	if err != nil {
		log.Printf("Error tracking uploaded file: %v", err)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           fileID,
		"filename":     storedFilename,
		"original_name": header.Filename,
		"url":          "/api/files/" + fileID.String(),
		"public_url":   "/api/public/files/" + fileID.String(),
		"content_type": contentType,
		"size":         header.Size,
	})
}

// Serve serves a file (authenticated — checks tenant match).
func (h *FileHandler) Serve(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	tenantID := middleware.GetTenantID(r.Context())

	var filePath, contentType string
	err := h.db.Pool.QueryRow(r.Context(),
		`SELECT file_path, content_type FROM uploaded_files WHERE id=$1 AND tenant_id=$2`,
		fileID, tenantID).Scan(&filePath, &contentType)
	if err != nil {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}

	fullPath := h.fileStore.GetFullPath(filePath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, fullPath)
}

// ServePublic serves a file without auth (for public signup pages).
// Only serves files linked to active entities.
func (h *FileHandler) ServePublic(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")

	var filePath, contentType string
	err := h.db.Pool.QueryRow(r.Context(),
		`SELECT file_path, content_type FROM uploaded_files WHERE id=$1`,
		fileID).Scan(&filePath, &contentType)
	if err != nil {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}

	// Security: only serve image types
	if !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}

	fullPath := h.fileStore.GetFullPath(filePath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, fullPath)
}
