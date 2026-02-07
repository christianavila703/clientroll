package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// AllowedContentTypes lists the accepted image MIME types.
var AllowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// MaxFileSize is the maximum upload size (5MB).
const MaxFileSize = 5 * 1024 * 1024

// FileStore handles secure local file storage.
// Files are stored outside the web root in uploads/tenants/{tenant_id}/{type}/
type FileStore struct {
	basePath string // e.g. "/var/www/clientroll/uploads"
}

// NewFileStore creates a new file store.
func NewFileStore(basePath string) *FileStore {
	return &FileStore{basePath: basePath}
}

// Save stores a file and returns the stored path.
func (fs *FileStore) Save(tenantID, entityType string, originalFilename string, reader io.Reader) (string, string, error) {
	// Validate entity type (prevents path traversal)
	entityType = sanitisePath(entityType)

	// Generate unique filename
	ext := filepath.Ext(originalFilename)
	storedFilename := uuid.New().String() + ext
	dir := filepath.Join(fs.basePath, "tenants", tenantID, entityType)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", "", fmt.Errorf("creating directory: %w", err)
	}

	fullPath := filepath.Join(dir, storedFilename)
	relativePath := filepath.Join("tenants", tenantID, entityType, storedFilename)

	// Write file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	// Limit reader to max file size
	limited := io.LimitReader(reader, MaxFileSize+1)
	written, err := io.Copy(file, limited)
	if err != nil {
		os.Remove(fullPath)
		return "", "", fmt.Errorf("writing file: %w", err)
	}

	if written > MaxFileSize {
		os.Remove(fullPath)
		return "", "", fmt.Errorf("file exceeds maximum size of %d bytes", MaxFileSize)
	}

	return storedFilename, relativePath, nil
}

// GetFullPath returns the absolute path for a stored file.
func (fs *FileStore) GetFullPath(relativePath string) string {
	return filepath.Join(fs.basePath, relativePath)
}

// Delete removes a stored file.
func (fs *FileStore) Delete(relativePath string) error {
	fullPath := fs.GetFullPath(relativePath)
	return os.Remove(fullPath)
}

// IsAllowedContentType checks if a MIME type is allowed.
func IsAllowedContentType(contentType string) bool {
	return AllowedContentTypes[contentType]
}

func sanitisePath(s string) string {
	// Remove anything that could be a path traversal
	s = strings.ReplaceAll(s, "..", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	if s == "" {
		s = "general"
	}
	return s
}
