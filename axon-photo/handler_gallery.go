package photo

import (
	"context"
	"encoding/json"
	"net/http"
)

// UserIDFunc extracts the authenticated user ID from a request context.
type UserIDFunc func(ctx context.Context) string

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// GalleryListHandler returns an http.Handler for GET /api/agents/{slug}/gallery.
func GalleryListHandler(store GalleryStore, userID UserIDFunc) http.Handler {
	return &galleryListHandler{store: store, userID: userID}
}

type galleryListHandler struct {
	store  GalleryStore
	userID UserIDFunc
}

func (h *galleryListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid := h.userID(r.Context())
	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug required")
		return
	}

	images, err := h.store.ListGalleryImagesByUser(uid, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list images")
		return
	}

	if images == nil {
		images = []GalleryImage{}
	}

	type imageResponse struct {
		ID             string  `json:"id"`
		URL            string  `json:"url"`
		ThumbnailURL   string  `json:"thumbnail_url"`
		Prompt         string  `json:"prompt"`
		Model          string  `json:"model"`
		ConversationID *string `json:"conversation_id"`
		IsBase         bool    `json:"is_base"`
		NSFWDetected   bool    `json:"nsfw_detected"`
		CreatedAt      string  `json:"created_at"`
	}

	response := make([]imageResponse, len(images))
	for i, img := range images {
		response[i] = imageResponse{
			ID:             img.ID,
			URL:            "/api/images/" + img.ID,
			ThumbnailURL:   "/api/images/" + img.ID + "?size=thumb",
			Prompt:         img.Prompt,
			Model:          img.Model,
			ConversationID: img.ConversationID,
			IsBase:         img.IsBase,
			NSFWDetected:   img.NSFWDetected,
			CreatedAt:      img.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"images": response})
}

// GetBaseImageHandler returns an http.Handler for GET /api/agents/{slug}/gallery/base.
func GetBaseImageHandler(store GalleryStore, userID UserIDFunc) http.Handler {
	return &getBaseImageHandler{store: store, userID: userID}
}

type getBaseImageHandler struct {
	store  GalleryStore
	userID UserIDFunc
}

func (h *getBaseImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid := h.userID(r.Context())
	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug required")
		return
	}

	img, err := h.store.GetBaseImageByUser(uid, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get base image")
		return
	}

	if img == nil {
		writeError(w, http.StatusNotFound, "no base image set")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":      img.ID,
		"url":     "/api/images/" + img.ID,
		"is_base": true,
	})
}

// SetBaseImageHandler returns an http.Handler for PUT /api/agents/{slug}/gallery/{id}/base.
func SetBaseImageHandler(store GalleryStore, userID UserIDFunc) http.Handler {
	return &setBaseImageHandler{store: store, userID: userID}
}

type setBaseImageHandler struct {
	store  GalleryStore
	userID UserIDFunc
}

func (h *setBaseImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid := h.userID(r.Context())
	slug := r.PathValue("slug")
	imageID := r.PathValue("id")

	if slug == "" || imageID == "" {
		writeError(w, http.StatusBadRequest, "slug and id required")
		return
	}

	// Verify image belongs to this user's agent
	img, err := h.store.GetGalleryImage(imageID)
	if err != nil || img == nil || img.AgentSlug != slug || img.UserID != uid {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}

	if err := h.store.SetBaseImage(uid, slug, imageID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set base image")
		return
	}

	w.WriteHeader(http.StatusOK)
}
