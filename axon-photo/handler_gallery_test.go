package photo_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	photo "github.com/benaskins/axon-photo"
)

func testUserID(ctx context.Context) string {
	return "user-1"
}

// memGalleryStore is a minimal in-memory GalleryStore for testing.
type memGalleryStore struct {
	images    map[string]photo.GalleryImage
	baseImage map[string]string // key: userID+slug → imageID
}

func newMemGalleryStore() *memGalleryStore {
	return &memGalleryStore{
		images:    make(map[string]photo.GalleryImage),
		baseImage: make(map[string]string),
	}
}

func (s *memGalleryStore) SaveGalleryImage(img photo.GalleryImage) error {
	s.images[img.ID] = img
	return nil
}

func (s *memGalleryStore) GetGalleryImage(id string) (*photo.GalleryImage, error) {
	img, ok := s.images[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &img, nil
}

func (s *memGalleryStore) ListGalleryImagesByUser(userID, slug string) ([]photo.GalleryImage, error) {
	var result []photo.GalleryImage
	for _, img := range s.images {
		if img.UserID == userID && img.AgentSlug == slug {
			result = append(result, img)
		}
	}
	return result, nil
}

func (s *memGalleryStore) GetBaseImageByUser(userID, slug string) (*photo.GalleryImage, error) {
	key := userID + ":" + slug
	id, ok := s.baseImage[key]
	if !ok {
		return nil, nil
	}
	img, ok := s.images[id]
	if !ok {
		return nil, nil
	}
	return &img, nil
}

func (s *memGalleryStore) SetBaseImage(userID, slug, imageID string) error {
	key := userID + ":" + slug
	s.baseImage[key] = imageID
	return nil
}

func TestGalleryListHandler_ReturnsImages(t *testing.T) {
	store := newMemGalleryStore()
	store.SaveGalleryImage(photo.GalleryImage{
		ID:        "img-1",
		AgentSlug: "bot",
		UserID:    "user-1",
		Prompt:    "sunset",
		Model:     "sdxl",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	handler := photo.GalleryListHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("GET /api/agents/{slug}/gallery", handler)

	req := httptest.NewRequest("GET", "/api/agents/bot/gallery", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !contains(body, "img-1") {
		t.Errorf("body missing image ID: %s", body)
	}
	if !contains(body, "/api/images/img-1") {
		t.Errorf("body missing image URL: %s", body)
	}
}

func TestGalleryListHandler_EmptyGallery(t *testing.T) {
	store := newMemGalleryStore()
	handler := photo.GalleryListHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("GET /api/agents/{slug}/gallery", handler)

	req := httptest.NewRequest("GET", "/api/agents/bot/gallery", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !contains(body, `"images":[]`) {
		t.Errorf("expected empty images array, got: %s", body)
	}
}

func TestGetBaseImageHandler_ReturnsBaseImage(t *testing.T) {
	store := newMemGalleryStore()
	store.SaveGalleryImage(photo.GalleryImage{
		ID:        "img-1",
		AgentSlug: "bot",
		UserID:    "user-1",
	})
	store.SetBaseImage("user-1", "bot", "img-1")

	handler := photo.GetBaseImageHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("GET /api/agents/{slug}/gallery/base", handler)

	req := httptest.NewRequest("GET", "/api/agents/bot/gallery/base", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !contains(body, "img-1") {
		t.Errorf("body missing image ID: %s", body)
	}
}

func TestGetBaseImageHandler_NoBaseImage(t *testing.T) {
	store := newMemGalleryStore()
	handler := photo.GetBaseImageHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("GET /api/agents/{slug}/gallery/base", handler)

	req := httptest.NewRequest("GET", "/api/agents/bot/gallery/base", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetBaseImageHandler_SetsBase(t *testing.T) {
	store := newMemGalleryStore()
	store.SaveGalleryImage(photo.GalleryImage{
		ID:        "img-1",
		AgentSlug: "bot",
		UserID:    "user-1",
	})

	handler := photo.SetBaseImageHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("PUT /api/agents/{slug}/gallery/{id}/base", handler)

	req := httptest.NewRequest("PUT", "/api/agents/bot/gallery/img-1/base", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify base was set
	base, _ := store.GetBaseImageByUser("user-1", "bot")
	if base == nil || base.ID != "img-1" {
		t.Error("expected base image to be set")
	}
}

func TestSetBaseImageHandler_WrongUser(t *testing.T) {
	store := newMemGalleryStore()
	store.SaveGalleryImage(photo.GalleryImage{
		ID:        "img-1",
		AgentSlug: "bot",
		UserID:    "other-user", // different user
	})

	handler := photo.SetBaseImageHandler(store, testUserID) // user-1
	mux := http.NewServeMux()
	mux.Handle("PUT /api/agents/{slug}/gallery/{id}/base", handler)

	req := httptest.NewRequest("PUT", "/api/agents/bot/gallery/img-1/base", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// nilGetGalleryStore wraps memGalleryStore but returns (nil, nil) for GetGalleryImage
// to simulate a store that returns no error but also no image.
type nilGetGalleryStore struct {
	*memGalleryStore
}

func (s *nilGetGalleryStore) GetGalleryImage(id string) (*photo.GalleryImage, error) {
	return nil, nil
}

func TestSetBaseImageHandler_NilImage(t *testing.T) {
	store := &nilGetGalleryStore{newMemGalleryStore()}

	handler := photo.SetBaseImageHandler(store, testUserID)
	mux := http.NewServeMux()
	mux.Handle("PUT /api/agents/{slug}/gallery/{id}/base", handler)

	req := httptest.NewRequest("PUT", "/api/agents/bot/gallery/img-1/base", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
