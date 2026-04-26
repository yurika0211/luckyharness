package telegram

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type outboundMediaKind string

const (
	outboundMediaPhoto    outboundMediaKind = "photo"
	outboundMediaDocument outboundMediaKind = "document"
)

type outboundMedia struct {
	Kind    outboundMediaKind
	Source  string
	Caption string
}

var (
	tgMediaDirectivePattern = regexp.MustCompile(`(?i)^tg://(photo|document)\s+(\S+)(?:\s+(.*))?$`)
	markdownImagePattern    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	markdownLinkPattern     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	fencedCodePattern       = regexp.MustCompile("(?s)```([A-Za-z0-9_+.-]*)\\n(.*?)\\n```")
	filenameHintPattern     = regexp.MustCompile("(?i)(?:保存为|存为|另存为|save as|saved as|filename[:：]?|file[:：]?)\\s*`?([A-Za-z0-9._/-]+\\.[A-Za-z0-9]+)`?")
)

func resolveOutboundMediaResponse(response string) (string, []outboundMedia, error) {
	text, media := parseOutboundMediaResponse(response)
	if len(media) > 0 {
		return text, media, nil
	}
	return materializeGeneratedDocuments(response)
}

func parseOutboundMediaResponse(response string) (string, []outboundMedia) {
	text := strings.TrimSpace(response)
	if text == "" {
		return "", nil
	}

	var media []outboundMedia
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := tgMediaDirectivePattern.FindStringSubmatch(trimmed); m != nil {
			media = append(media, outboundMedia{
				Kind:    outboundMediaKind(strings.ToLower(m[1])),
				Source:  strings.TrimSpace(m[2]),
				Caption: strings.TrimSpace(m[3]),
			})
			continue
		}
		kept = append(kept, line)
	}

	text = strings.TrimSpace(strings.Join(kept, "\n"))
	if strings.Contains(text, "![") {
		text = markdownImagePattern.ReplaceAllStringFunc(text, func(match string) string {
			m := markdownImagePattern.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			media = append(media, outboundMedia{
				Kind:    outboundMediaPhoto,
				Source:  strings.TrimSpace(m[2]),
				Caption: strings.TrimSpace(m[1]),
			})
			return ""
		})
		text = strings.TrimSpace(text)
	}

	if strings.Contains(text, "](") {
		text = markdownLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
			m := markdownLinkPattern.FindStringSubmatch(match)
			if m == nil {
				return match
			}

			source := strings.TrimSpace(m[2])
			kind, ok := inferMediaKind(source)
			if !ok {
				return match
			}

			media = append(media, outboundMedia{
				Kind:    kind,
				Source:  source,
				Caption: strings.TrimSpace(m[1]),
			})
			return ""
		})
		text = strings.TrimSpace(text)
	}

	if len(media) == 0 {
		if kind, ok := detectImplicitMedia(text); ok {
			return "", []outboundMedia{{
				Kind:   kind,
				Source: strings.TrimSpace(text),
			}}
		}
	}

	return normalizeOutboundText(text), media
}

func detectImplicitMedia(text string) (outboundMediaKind, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return "", false
	}
	return inferMediaKind(trimmed)
}

func inferMediaKind(source string) (outboundMediaKind, bool) {
	ext := strings.ToLower(mediaSourceExt(source))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return outboundMediaPhoto, true
	case ".pdf", ".txt", ".md", ".json", ".csv", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".zip", ".svg", ".xml", ".html", ".htm", ".js", ".ts", ".py", ".go", ".yaml", ".yml":
		return outboundMediaDocument, true
	default:
		return "", false
	}
}

func mediaSourceExt(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(source), "sandbox:/") {
		source = strings.TrimPrefix(source, "sandbox:")
	}
	if strings.HasPrefix(strings.ToLower(source), "file://") {
		if u, err := url.Parse(source); err == nil {
			return path.Ext(u.Path)
		}
	}
	if u, err := url.Parse(source); err == nil && u.Scheme != "" {
		return path.Ext(u.Path)
	}
	return filepath.Ext(source)
}

func normalizeOutboundText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		blank := strings.TrimSpace(line) == ""
		if blank {
			if prevBlank {
				continue
			}
			prevBlank = true
			out = append(out, "")
			continue
		}
		prevBlank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func summarizeOutboundMedia(media []outboundMedia) string {
	if len(media) == 0 {
		return "📎 已发送附件"
	}
	photos := 0
	docs := 0
	for _, item := range media {
		switch item.Kind {
		case outboundMediaPhoto:
			photos++
		case outboundMediaDocument:
			docs++
		}
	}

	parts := make([]string, 0, 2)
	if photos > 0 {
		if photos == 1 {
			parts = append(parts, "1 张图片")
		} else {
			parts = append(parts, strconv.Itoa(photos)+" 张图片")
		}
	}
	if docs > 0 {
		if docs == 1 {
			parts = append(parts, "1 个文件")
		} else {
			parts = append(parts, strconv.Itoa(docs)+" 个文件")
		}
	}
	if len(parts) == 0 {
		return "📎 已发送附件"
	}
	return "📎 已发送 " + strings.Join(parts, "、")
}

func materializeGeneratedDocuments(response string) (string, []outboundMedia, error) {
	text := strings.TrimSpace(response)
	if text == "" {
		return "", nil, nil
	}

	matches := fencedCodePattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return "", nil, nil
	}

	var built strings.Builder
	cursor := 0
	media := make([]outboundMedia, 0, len(matches))
	docIndex := 0

	for _, match := range matches {
		built.WriteString(text[cursor:match[0]])
		cursor = match[1]

		lang := strings.TrimSpace(text[match[2]:match[3]])
		body := strings.TrimSpace(text[match[4]:match[5]])
		if body == "" {
			built.WriteString(text[match[0]:match[1]])
			continue
		}

		filename := detectGeneratedFilenameNear(text, match[0], match[1])
		ext := strings.ToLower(filepath.Ext(filename))
		if ext == "" {
			ext = extensionFromFenceLang(lang)
		}
		if ext == "" {
			built.WriteString(text[match[0]:match[1]])
			continue
		}

		docIndex++
		if strings.TrimSpace(filename) == "" {
			filename = fmt.Sprintf("generated-%d%s", docIndex, ext)
		}

		tmpFile, err := os.CreateTemp("", "luckyharness-tg-*"+ext)
		if err != nil {
			return "", nil, fmt.Errorf("create temp artifact: %w", err)
		}
		if _, err := tmpFile.WriteString(body + "\n"); err != nil {
			tmpFile.Close()
			return "", nil, fmt.Errorf("write temp artifact: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return "", nil, fmt.Errorf("close temp artifact: %w", err)
		}

		media = append(media, outboundMedia{
			Kind:    outboundMediaDocument,
			Source:  tmpFile.Name(),
			Caption: filepath.Base(filename),
		})
	}

	built.WriteString(text[cursor:])
	if len(media) == 0 {
		return "", nil, nil
	}

	return normalizeOutboundText(built.String()), media, nil
}

func detectGeneratedFilenameNear(text string, blockStart int, blockEnd int) string {
	prefix := text[:blockStart]
	matches := filenameHintPattern.FindAllStringSubmatch(prefix, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if len(last) >= 2 {
			return filepath.Base(strings.TrimSpace(last[1]))
		}
	}

	suffix := text[blockEnd:]
	matches = filenameHintPattern.FindAllStringSubmatch(suffix, -1)
	if len(matches) > 0 {
		first := matches[0]
		if len(first) >= 2 {
			return filepath.Base(strings.TrimSpace(first[1]))
		}
	}

	return ""
}

func extensionFromFenceLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "svg":
		return ".svg"
	case "json":
		return ".json"
	case "html":
		return ".html"
	case "xml":
		return ".xml"
	case "csv":
		return ".csv"
	case "md", "markdown":
		return ".md"
	case "txt", "text":
		return ".txt"
	case "yaml", "yml":
		return ".yaml"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "python", "py":
		return ".py"
	case "go", "golang":
		return ".go"
	default:
		return ""
	}
}
