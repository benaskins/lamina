package photo_test

import (
	"context"
	"testing"
	"time"

	photo "github.com/benaskins/axon-photo"
)

func TestGalleryImageFields(t *testing.T) {
	img := photo.GalleryImage{
		ID:        "img-1",
		AgentSlug: "helper",
		UserID:    "user-1",
		Prompt:    "a sunset",
		Model:     "sdxl",
		CreatedAt: time.Now(),
	}

	if img.ID != "img-1" {
		t.Errorf("ID = %q, want %q", img.ID, "img-1")
	}
	if img.AgentSlug != "helper" {
		t.Errorf("AgentSlug = %q, want %q", img.AgentSlug, "helper")
	}
}

func TestImageTaskSubmissionFields(t *testing.T) {
	sub := photo.ImageTaskSubmission{
		Prompt:    "a sunset over mountains",
		AgentSlug: "helper",
		UserID:    "user-1",
		ImageID:   "img-1",
		Private:   true,
	}

	if sub.Prompt != "a sunset over mountains" {
		t.Errorf("Prompt = %q, want %q", sub.Prompt, "a sunset over mountains")
	}
	if !sub.Private {
		t.Error("expected Private to be true")
	}
}

// Verify interfaces are satisfied by simple implementations.

type stubGalleryStore struct{}

func (s *stubGalleryStore) SaveGalleryImage(img photo.GalleryImage) error                  { return nil }
func (s *stubGalleryStore) GetGalleryImage(id string) (*photo.GalleryImage, error)          { return nil, nil }
func (s *stubGalleryStore) ListGalleryImagesByUser(userID, slug string) ([]photo.GalleryImage, error) {
	return nil, nil
}
func (s *stubGalleryStore) GetBaseImageByUser(userID, slug string) (*photo.GalleryImage, error) {
	return nil, nil
}
func (s *stubGalleryStore) SetBaseImage(userID, slug, imageID string) error { return nil }

type stubTaskSubmitter struct{}

func (s *stubTaskSubmitter) SubmitTask(ctx context.Context, req *photo.TaskSubmitRequest) (*photo.TaskSubmission, error) {
	return &photo.TaskSubmission{TaskID: "t-1", Status: "queued"}, nil
}

func TestGalleryStoreInterface(t *testing.T) {
	var store photo.GalleryStore = &stubGalleryStore{}
	if err := store.SaveGalleryImage(photo.GalleryImage{}); err != nil {
		t.Fatal(err)
	}
}

func TestTaskSubmitterInterface(t *testing.T) {
	var sub photo.TaskSubmitter = &stubTaskSubmitter{}
	result, err := sub.SubmitTask(context.Background(), &photo.TaskSubmitRequest{Type: "image_generation"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TaskID != "t-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "t-1")
	}
}
