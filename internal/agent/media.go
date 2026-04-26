package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yurika0211/luckyharness/internal/gateway"
	"github.com/yurika0211/luckyharness/internal/multimodal"
	"github.com/yurika0211/luckyharness/internal/utils"
)

func (a *Agent) AnalyzeAttachments(ctx context.Context, attachments []gateway.Attachment) (string, error) {
	if a == nil || a.mediaProcessor == nil || len(attachments) == 0 {
		return "", nil
	}

	var sections []string
	for i, att := range attachments {
		input, title, err := buildMultimodalInput(att)
		if err != nil {
			sections = append(sections, fmt.Sprintf("%s\n- error: %s", attachmentTitle(att, i), err.Error()))
			continue
		}

		result, err := a.mediaProcessor.Analyze(ctx, input)
		if err != nil {
			sections = append(sections, fmt.Sprintf("%s\n- error: %s", title, err.Error()))
			continue
		}

		sections = append(sections, formatAttachmentAnalysis(title, result))
	}

	if len(sections) == 0 {
		return "", nil
	}

	return "[Multimodal Analysis]\n" + strings.Join(sections, "\n\n"), nil
}

func buildMultimodalInput(att gateway.Attachment) (*multimodal.Input, string, error) {
	modality := attachmentModality(att)
	title := attachmentTitle(att, 0)

	if modality == multimodal.ModalityText {
		return nil, title, fmt.Errorf("unsupported attachment type %q", att.Type)
	}

	if len(att.Data) > 0 {
		input := multimodal.NewInput(modality, att.MimeType, att.Data)
		if input.Metadata == nil {
			input.Metadata = make(map[string]string)
		}
		input.Metadata["filename"] = att.FileName
		input.Metadata["file_url"] = att.FileURL
		return input, title, nil
	}

	if strings.TrimSpace(att.FileURL) != "" {
		input := multimodal.NewInputFromURL(modality, att.FileURL)
		input.MimeType = att.MimeType
		input.Metadata = map[string]string{
			"filename": att.FileName,
			"file_url": att.FileURL,
		}
		return input, title, nil
	}

	return nil, title, fmt.Errorf("attachment has no downloadable data or url")
}

func attachmentModality(att gateway.Attachment) multimodal.Modality {
	switch att.Type {
	case gateway.AttachmentImage:
		return multimodal.ModalityImage
	case gateway.AttachmentAudio:
		return multimodal.ModalityAudio
	case gateway.AttachmentVideo:
		return multimodal.ModalityVideo
	case gateway.AttachmentDocument:
		if strings.EqualFold(strings.TrimSpace(att.MimeType), "application/pdf") || strings.EqualFold(filepath.Ext(att.FileName), ".pdf") {
			return multimodal.ModalityDocument
		}
		return multimodal.ModalityDocument
	default:
		return multimodal.ModalityText
	}
}

func attachmentTitle(att gateway.Attachment, idx int) string {
	name := strings.TrimSpace(att.FileName)
	if name == "" {
		name = "unnamed"
	}

	prefix := "Attachment"
	switch att.Type {
	case gateway.AttachmentImage:
		prefix = "Image"
	case gateway.AttachmentAudio:
		prefix = "Audio"
	case gateway.AttachmentVideo:
		prefix = "Video"
	case gateway.AttachmentDocument:
		prefix = "Document"
	}

	if idx > 0 {
		return fmt.Sprintf("%s %d: %s", prefix, idx, name)
	}
	return fmt.Sprintf("%s: %s", prefix, name)
}

func formatAttachmentAnalysis(title string, result *multimodal.AnalysisResult) string {
	if result == nil {
		return title + "\n- analysis: unavailable"
	}

	lines := []string{title}
	if summary := strings.TrimSpace(result.Summary); summary != "" {
		lines = append(lines, "- summary: "+utils.Truncate(summary, 400))
	}
	if text := strings.TrimSpace(result.Text); text != "" {
		lines = append(lines, "- extracted: "+utils.Truncate(text, 1200))
	}
	if len(result.Labels) > 0 {
		lines = append(lines, "- labels: "+strings.Join(result.Labels, ", "))
	}
	if result.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("- confidence: %.2f", result.Confidence))
	}
	if result.Metadata != nil {
		if model := strings.TrimSpace(result.Metadata["model"]); model != "" {
			lines = append(lines, "- model: "+model)
		}
		if source := strings.TrimSpace(result.Metadata["source"]); source != "" {
			lines = append(lines, "- source: "+source)
		}
	}
	return strings.Join(lines, "\n")
}
