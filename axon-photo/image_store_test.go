package photo_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	photo "github.com/benaskins/axon-photo"
)

func mustNewImageStore(t *testing.T, dir string) *photo.ImageStore {
	t.Helper()
	store, err := photo.NewImageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestImageStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	data := []byte("fake png data")
	id, err := store.Save(data)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	loaded, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded) != string(data) {
		t.Errorf("loaded data = %q, want %q", loaded, data)
	}
}

func TestImageStore_SaveWithID(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	data := []byte("image bytes")
	err := store.SaveWithID("custom-id", data)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load("custom-id")
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded) != string(data) {
		t.Errorf("loaded = %q, want %q", loaded, data)
	}
}

func TestImageStore_LoadSize_Variant(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	// Write original and thumb variant
	os.WriteFile(filepath.Join(dir, "img1.png"), []byte("original"), 0644)
	os.WriteFile(filepath.Join(dir, "img1_thumb.png"), []byte("thumb"), 0644)

	data, err := store.LoadSize("img1", "thumb")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "thumb" {
		t.Errorf("got %q, want %q", data, "thumb")
	}
}

func TestImageStore_LoadSize_FallsBackToOriginal(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	os.WriteFile(filepath.Join(dir, "img1.png"), []byte("original"), 0644)

	// Request a size variant that doesn't exist
	data, err := store.LoadSize("img1", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Errorf("got %q, want %q", data, "original")
	}
}

func TestImageStore_LoadSize_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	for _, id := range []string{"../etc/passwd", "foo/bar", "a\\b", "a..b/c"} {
		_, err := store.LoadSize(id, "")
		if err == nil {
			t.Errorf("expected error for ID %q", id)
		}
	}
}

func TestImageStore_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("expected error for missing image")
	}
}

func TestImageHandler_ServesImage(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)
	store.SaveWithID("test-img", []byte("png data"))

	handler := photo.ImageHandler(store)

	// Use a mux to set up PathValue
	mux := http.NewServeMux()
	mux.Handle("GET /api/images/{id}", handler)

	req := httptest.NewRequest("GET", "/api/images/test-img", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "png data" {
		t.Errorf("body = %q, want %q", w.Body.String(), "png data")
	}
}

func TestImageHandler_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	handler := photo.ImageHandler(store)
	mux := http.NewServeMux()
	mux.Handle("GET /api/images/{id}", handler)

	req := httptest.NewRequest("GET", "/api/images/missing", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestImageHandler_MethodNotAllowed(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	handler := photo.ImageHandler(store)

	req := httptest.NewRequest("POST", "/api/images/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestImageStore_SaveWithID_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := mustNewImageStore(t, dir)

	for _, id := range []string{"../etc/passwd", "foo/bar", "a\\b", "a..b/c"} {
		err := store.SaveWithID(id, []byte("data"))
		if err == nil {
			t.Errorf("expected error for ID %q", id)
		}
	}
}
