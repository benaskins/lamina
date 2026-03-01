package photo

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	tool "github.com/benaskins/axon-tool"
)

// Config holds dependencies for building photo tools.
type Config struct {
	TaskSubmitter TaskSubmitter
	PromptMerger  *PromptMerger  // nil = no prompt merging
	ImageStore    *ImageStore    // nil = no ref image loading
	GalleryStore  GalleryStore   // nil = no base image
	MessageStore  MessageStore   // nil = no recent message context
	OnTaskStarted func(taskID, taskType, description string)
	StartPoll     func(taskID string)
}

// TakePhotoTool returns a tool.ToolDef for the standard take_photo skill.
func TakePhotoTool(cfg *Config) tool.ToolDef {
	return photoTool(cfg, false)
}

// TakePrivatePhotoTool returns a tool.ToolDef for the unrestricted take_private_photo skill.
func TakePrivatePhotoTool(cfg *Config) tool.ToolDef {
	return photoTool(cfg, true)
}

func photoTool(cfg *Config, private bool) tool.ToolDef {
	name := "take_photo"
	description := "Take a photo or selfie. Only use when the user asks, you're in a new setting, or a significant visual moment happens. Most conversation turns should NOT include a photo."
	paramDesc := "A detailed description of the image to generate. Include subject, setting, lighting, mood, and composition details."
	if private {
		name = "take_private_photo"
		description = "Take a photo using the private unrestricted pipeline. Use instead of take_photo when the scene requires content that the standard pipeline would filter out. Only use when the user specifically requests this kind of content."
		paramDesc += " This pipeline has no content restrictions."
	}

	return tool.ToolDef{
		Name:        name,
		Description: description,
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"prompt"},
			Properties: map[string]tool.PropertySchema{
				"prompt": {Type: "string", Description: paramDesc},
			},
		},
		Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
			promptStr, _ := args["prompt"].(string)
			if promptStr == "" || cfg.TaskSubmitter == nil {
				label := "image"
				if private {
					label = "private image"
				}
				return tool.ToolResult{Content: fmt.Sprintf("Error: %s generation not available", label)}
			}
			return submitImageTask(cfg, ctx, promptStr, private)
		},
	}
}

func submitImageTask(cfg *Config, ctx *tool.ToolContext, promptStr string, private bool) tool.ToolResult {
	imageID := uuid.New().String()

	finalPrompt := promptStr
	if cfg.PromptMerger != nil {
		var recentMessages []Message
		if ctx.ConversationID != "" && cfg.MessageStore != nil {
			if msgs, err := cfg.MessageStore.GetRecentMessages(ctx.ConversationID, 5); err == nil {
				recentMessages = msgs
			}
		}

		var merged string
		var err error
		if private {
			merged, err = cfg.PromptMerger.MergePromptPrivate(ctx.SystemPrompt, recentMessages, promptStr)
		} else {
			merged, err = cfg.PromptMerger.MergePrompt(ctx.SystemPrompt, recentMessages, promptStr)
		}
		if err == nil {
			finalPrompt = merged
		} else {
			slog.Warn("prompt merge failed, using raw scene prompt", "error", err)
		}
	}

	var refImageB64 string
	if ctx.AgentSlug != "" && cfg.GalleryStore != nil && cfg.ImageStore != nil {
		if baseImg, err := cfg.GalleryStore.GetBaseImageByUser(ctx.UserID, ctx.AgentSlug); err == nil && baseImg != nil {
			if imgData, err := cfg.ImageStore.Load(baseImg.ID); err == nil {
				refImageB64 = base64.StdEncoding.EncodeToString(imgData)
			} else {
				slog.Warn("failed to load base image", "error", err, "base_image_id", baseImg.ID)
			}
		}
	}

	submission := &ImageTaskSubmission{
		Prompt:         finalPrompt,
		ReferenceImage: refImageB64,
		AgentSlug:      ctx.AgentSlug,
		UserID:         ctx.UserID,
		ConversationID: ctx.ConversationID,
		ImageID:        imageID,
		Private:        private,
	}

	_, err := cfg.TaskSubmitter.SubmitTask(context.Background(), NewImageTaskRequest(submission))
	if err != nil {
		slog.Error("failed to submit image task", "error", err, "image_id", imageID)
		return tool.ToolResult{Content: "Error: failed to submit image generation task"}
	}

	if cfg.OnTaskStarted != nil {
		cfg.OnTaskStarted(imageID, "image_generation", "Generating image...")
	}

	if cfg.StartPoll != nil {
		cfg.StartPoll(imageID)
	}

	pipelineLabel := "Image"
	if private {
		pipelineLabel = "Image (private pipeline)"
	}
	return tool.ToolResult{Content: fmt.Sprintf("%s generation started, it will appear shortly.", pipelineLabel)}
}
