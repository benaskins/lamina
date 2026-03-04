package photo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ImageStore handles saving and loading images from the filesystem.
type ImageStore struct {
	dir string
}

// NewImageStore creates a store backed by the given directory.
// Returns an error if the directory cannot be created.
func NewImageStore(dir string) (*ImageStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create image store directory: %w", err)
	}
	return &ImageStore{dir: dir}, nil
}

// validateID checks that an image ID does not contain path traversal sequences.
func validateID(id string) error {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid image ID")
	}
	return nil
}

// Save writes image data to a new file and returns its ID.
func (s *ImageStore) Save(data []byte) (string, error) {
	id := uuid.New().String()
	path := filepath.Join(s.dir, id+".png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}
	return id, nil
}

// SaveWithID writes image data to a file with the given ID.
func (s *ImageStore) SaveWithID(id string, data []byte) error {
	if err := validateID(id); err != nil {
		return err
	}
	path := filepath.Join(s.dir, id+".png")
	return os.WriteFile(path, data, 0644)
}

// validSizes maps query param values to file suffixes.
var validSizes = map[string]string{
	"thumb":  "_thumb",
	"medium": "_medium",
	"lg":     "_lg",
}

// Load reads image data by ID (full size).
func (s *ImageStore) Load(id string) ([]byte, error) {
	return s.LoadSize(id, "")
}

// LoadSize reads image data by ID at the given size variant.
// If the variant file doesn't exist, falls back to the original.
func (s *ImageStore) LoadSize(id, size string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}

	// Try the size variant first.
	if suffix, ok := validSizes[size]; ok {
		path := filepath.Join(s.dir, id+suffix+".png")
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		// Fall through to original if variant doesn't exist.
	}

	path := filepath.Join(s.dir, id+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("image not found: %w", err)
	}
	return data, nil
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ImageHandler returns an http.Handler that serves GET /api/images/{id}.
func ImageHandler(store *ImageStore) http.Handler {
	return &imageHandler{store: store}
}

type imageHandler struct {
	store *ImageStore
}

func (h *imageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "image ID required")
		return
	}

	size := r.URL.Query().Get("size")
	data, err := h.store.LoadSize(id, size)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}
