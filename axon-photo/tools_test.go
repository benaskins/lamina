package photo_test

import (
	"context"
	"testing"

	photo "github.com/benaskins/axon-photo"
	tool "github.com/benaskins/axon-tool"
)

type mockTaskSubmitter struct {
	lastReq *photo.TaskSubmitRequest
}

func (m *mockTaskSubmitter) SubmitTask(ctx context.Context, req *photo.TaskSubmitRequest) (*photo.TaskSubmission, error) {
	m.lastReq = req
	return &photo.TaskSubmission{TaskID: "task-1", Status: "queued"}, nil
}

func TestTakePhotoTool_Schema(t *testing.T) {
	cfg := &photo.Config{TaskSubmitter: &mockTaskSubmitter{}}
	td := photo.TakePhotoTool(cfg)

	if td.Name != "take_photo" {
		t.Errorf("Name = %q, want %q", td.Name, "take_photo")
	}
	if _, ok := td.Parameters.Properties["prompt"]; !ok {
		t.Error("expected prompt parameter")
	}
}

func TestTakePrivatePhotoTool_Schema(t *testing.T) {
	cfg := &photo.Config{TaskSubmitter: &mockTaskSubmitter{}}
	td := photo.TakePrivatePhotoTool(cfg)

	if td.Name != "take_private_photo" {
		t.Errorf("Name = %q, want %q", td.Name, "take_private_photo")
	}
}

func TestTakePhotoTool_SubmitsTask(t *testing.T) {
	submitter := &mockTaskSubmitter{}
	var startedTaskID, startedType string
	cfg := &photo.Config{
		TaskSubmitter: submitter,
		OnTaskStarted: func(taskID, taskType, desc string) {
			startedTaskID = taskID
			startedType = taskType
		},
	}

	td := photo.TakePhotoTool(cfg)
	ctx := &tool.ToolContext{
		Ctx:       context.Background(),
		UserID:    "user-1",
		AgentSlug: "bot",
	}

	result := td.Execute(ctx, map[string]any{"prompt": "a sunset"})
	if result.Content == "" {
		t.Error("expected non-empty result")
	}

	if submitter.lastReq == nil {
		t.Fatal("expected task to be submitted")
	}
	if submitter.lastReq.Type != "image_generation" {
		t.Errorf("Type = %q, want %q", submitter.lastReq.Type, "image_generation")
	}

	if startedTaskID == "" {
		t.Error("expected OnTaskStarted to be called")
	}
	if startedType != "image_generation" {
		t.Errorf("startedType = %q, want %q", startedType, "image_generation")
	}
}

func TestTakePhotoTool_EmptyPrompt(t *testing.T) {
	cfg := &photo.Config{TaskSubmitter: &mockTaskSubmitter{}}
	td := photo.TakePhotoTool(cfg)

	result := td.Execute(&tool.ToolContext{Ctx: context.Background()}, map[string]any{"prompt": ""})
	if result.Content != "Error: image generation not available" {
		t.Errorf("result = %q", result.Content)
	}
}

func TestTakePrivatePhotoTool_EmptyPrompt(t *testing.T) {
	cfg := &photo.Config{TaskSubmitter: &mockTaskSubmitter{}}
	td := photo.TakePrivatePhotoTool(cfg)

	result := td.Execute(&tool.ToolContext{Ctx: context.Background()}, map[string]any{"prompt": ""})
	if result.Content != "Error: private image generation not available" {
		t.Errorf("result = %q", result.Content)
	}
}

func TestTakePhotoTool_NoSubmitter(t *testing.T) {
	cfg := &photo.Config{} // no TaskSubmitter
	td := photo.TakePhotoTool(cfg)

	result := td.Execute(&tool.ToolContext{Ctx: context.Background()}, map[string]any{"prompt": "test"})
	if result.Content != "Error: image generation not available" {
		t.Errorf("result = %q", result.Content)
	}
}

func TestTakePhotoTool_WithPromptMerger(t *testing.T) {
	submitter := &mockTaskSubmitter{}
	gen := fakeGenerator("merged prompt")
	merger := photo.NewPromptMerger(gen, &photo.ImageGenConfig{
		MergeInstruction: "{scene}",
	})

	cfg := &photo.Config{
		TaskSubmitter: submitter,
		PromptMerger:  merger,
	}

	td := photo.TakePhotoTool(cfg)
	result := td.Execute(&tool.ToolContext{Ctx: context.Background()}, map[string]any{"prompt": "raw scene"})

	if result.Content == "" {
		t.Error("expected non-empty result")
	}
	// Verify the merged prompt was used (check submission params)
	if submitter.lastReq == nil {
		t.Fatal("expected task submission")
	}
	params := submitter.lastReq.Params.(*photo.ImageTaskSubmission)
	if params.Prompt != "merged prompt" {
		t.Errorf("Prompt = %q, want %q", params.Prompt, "merged prompt")
	}
}

func TestTakePhotoTool_WithBaseImage(t *testing.T) {
	submitter := &mockTaskSubmitter{}
	store := newMemGalleryStore()
	store.SaveGalleryImage(photo.GalleryImage{
		ID:        "base-img",
		AgentSlug: "bot",
		UserID:    "user-1",
	})
	store.SetBaseImage("user-1", "bot", "base-img")

	dir := testing.TB.TempDir(t)
	imgStore, err := photo.NewImageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	imgStore.SaveWithID("base-img", []byte("ref image data"))

	cfg := &photo.Config{
		TaskSubmitter: submitter,
		GalleryStore:  store,
		ImageStore:    imgStore,
	}

	td := photo.TakePhotoTool(cfg)
	ctx := &tool.ToolContext{
		Ctx:       context.Background(),
		UserID:    "user-1",
		AgentSlug: "bot",
	}
	td.Execute(ctx, map[string]any{"prompt": "test"})

	if submitter.lastReq == nil {
		t.Fatal("expected task submission")
	}
	params := submitter.lastReq.Params.(*photo.ImageTaskSubmission)
	if params.ReferenceImage == "" {
		t.Error("expected reference image to be populated")
	}
}

func TestTakePhotoTool_StartPollCalled(t *testing.T) {
	var polledID string
	cfg := &photo.Config{
		TaskSubmitter: &mockTaskSubmitter{},
		StartPoll: func(taskID string) {
			polledID = taskID
		},
	}

	td := photo.TakePhotoTool(cfg)
	td.Execute(&tool.ToolContext{Ctx: context.Background()}, map[string]any{"prompt": "test"})

	if polledID == "" {
		t.Error("expected StartPoll to be called")
	}
}
