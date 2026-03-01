// Package photo provides image generation tools for LLM-powered agents.
// It handles prompt merging, image storage, gallery management, and
// task submission for image generation pipelines.
package photo

import (
	"context"
	"time"
)

// GalleryImage represents a generated image with metadata.
type GalleryImage struct {
	ID             string    `json:"id"`
	AgentSlug      string    `json:"agent_slug"`
	UserID         string    `json:"user_id"`
	ConversationID *string   `json:"conversation_id"`
	Prompt         string    `json:"prompt"`
	Model          string    `json:"model"`
	IsBase         bool      `json:"is_base"`
	NSFWDetected   bool      `json:"nsfw_detected"`
	CreatedAt      time.Time `json:"created_at"`
}

// ImageTaskSubmission holds the parameters for an image generation task.
type ImageTaskSubmission struct {
	Prompt         string `json:"prompt"`
	ReferenceImage string `json:"reference_image,omitempty"`
	AgentSlug      string `json:"agent_slug"`
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	ImageID        string `json:"image_id"`
	Private        bool   `json:"private,omitempty"`
}

// TaskSubmitRequest is the request body for submitting a task.
type TaskSubmitRequest struct {
	Type   string `json:"type"`
	Params any    `json:"params"`
}

// TaskSubmission is the response from submitting a task.
type TaskSubmission struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// NewImageTaskRequest creates a TaskSubmitRequest for image generation.
func NewImageTaskRequest(params *ImageTaskSubmission) *TaskSubmitRequest {
	return &TaskSubmitRequest{
		Type:   "image_generation",
		Params: params,
	}
}

// GalleryStore persists gallery images and base image selections.
type GalleryStore interface {
	SaveGalleryImage(img GalleryImage) error
	GetGalleryImage(id string) (*GalleryImage, error)
	ListGalleryImagesByUser(userID string, agentSlug string) ([]GalleryImage, error)
	GetBaseImageByUser(userID string, agentSlug string) (*GalleryImage, error)
	SetBaseImage(userID string, agentSlug string, imageID string) error
}

// MessageStore provides access to recent conversation messages
// for prompt merging context.
type MessageStore interface {
	GetRecentMessages(conversationID string, limit int) ([]Message, error)
}

// Message is a minimal message type for prompt merging context.
type Message struct {
	Role    string
	Content string
}

// TaskSubmitter submits image generation tasks to an external runner.
type TaskSubmitter interface {
	SubmitTask(ctx context.Context, req *TaskSubmitRequest) (*TaskSubmission, error)
}

// CameraPrompt returns the system prompt section for the take_photo skill.
func CameraPrompt() string {
	return `## Camera
You have a camera and can take photos. Use it when there's a visual moment worth sharing — a new setting, something you made, or when the user asks to see something.`
}

// PrivateCameraPrompt returns the system prompt section for the take_private_photo skill.
func PrivateCameraPrompt() string {
	return `## Private Camera
You have access to a private camera with no content restrictions. Use take_private_photo instead of take_photo when the user requests content that requires the unrestricted pipeline.`
}
