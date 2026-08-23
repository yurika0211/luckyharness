package agent

import (
	"strings"

	"github.com/yurika0211/luckyagent/internal/config"
	"github.com/yurika0211/luckyagent/internal/multimodal"
	"github.com/yurika0211/luckyagent/internal/tool"
)

type mediaRuntime struct {
	processor            *multimodal.Processor
	imageGenerator       multimodal.ImageGenerator
	imageDefaults        tool.ImageGenerationDefaults
	speechSynthesizer    multimodal.SpeechSynthesizer
	ttsDefaults          tool.TTSDefaults
	defaultImageProvider string
}

func buildMediaRuntime(c *config.Config) mediaRuntime {
	runtime := mediaRuntime{processor: multimodal.NewProcessor()}
	_ = runtime.processor.RegisterProvider(multimodal.NewLocalProvider(
		multimodal.ModalityText,
		multimodal.ModalityImage,
		multimodal.ModalityAudio,
		multimodal.ModalityVideo,
		multimodal.ModalityDocument,
	), true)

	mmCfg, mmOK := resolveOpenAIMultimodalConfig(c)
	if mmOK {
		if openaiMedia, err := multimodal.NewOpenAIMediaProvider(multimodal.OpenAIMediaConfig{
			APIKey:             mmCfg.APIKey,
			APIBase:            mmCfg.APIBase,
			ResponsesModel:     mmCfg.ImageModel,
			TranscriptionModel: mmCfg.TranscriptionModel,
		}); err == nil {
			_ = runtime.processor.RegisterProvider(openaiMedia, true)
			runtime.imageGenerator = openaiMedia
		}
	}

	if imageCfg, ok := resolveImageGenerationConfig(c); ok {
		switch imageCfg.Provider {
		case "gemini":
			if generator, err := multimodal.NewGeminiImageProvider(multimodal.GeminiImageConfig{
				APIKey: imageCfg.APIKey, APIBase: imageCfg.APIBase, AuthMode: imageCfg.AuthMode,
			}); err == nil {
				runtime.imageGenerator = generator
			}
		case "openai":
			if generator, err := multimodal.NewOpenAIMediaProvider(multimodal.OpenAIMediaConfig{
				APIKey:             imageCfg.APIKey,
				APIBase:            imageCfg.APIBase,
				ResponsesModel:     mmCfg.ImageModel,
				TranscriptionModel: mmCfg.TranscriptionModel,
			}); err == nil {
				runtime.imageGenerator = generator
			}
		}
	}

	if ttsCfg, ok := resolveTTSConfig(c); ok && ttsCfg.Provider == "openai" {
		if synthesizer, err := multimodal.NewOpenAITTSProvider(multimodal.OpenAITTSConfig{
			APIKey: ttsCfg.APIKey, APIBase: ttsCfg.APIBase, AuthMode: ttsCfg.AuthMode,
		}); err == nil {
			runtime.speechSynthesizer = synthesizer
		}
	}

	runtime.imageDefaults = tool.ImageGenerationDefaults{
		Model:             strings.TrimSpace(c.ImageGeneration.Model),
		Size:              strings.TrimSpace(c.ImageGeneration.Size),
		Quality:           strings.TrimSpace(c.ImageGeneration.Quality),
		Background:        strings.TrimSpace(c.ImageGeneration.Background),
		OutputFormat:      strings.TrimSpace(c.ImageGeneration.OutputFormat),
		OutputCompression: c.ImageGeneration.OutputCompression,
		Count:             c.ImageGeneration.Count,
	}
	runtime.ttsDefaults = tool.TTSDefaults{
		Model: strings.TrimSpace(c.TTS.Model), Voice: strings.TrimSpace(c.TTS.Voice),
		Format: strings.TrimSpace(c.TTS.Format), Speed: c.TTS.Speed,
	}
	runtime.defaultImageProvider = c.Multimodal.ImageProvider
	return runtime
}

func (a *Agent) reloadMediaRuntime(c *config.Config) {
	if a == nil || c == nil {
		return
	}
	next := buildMediaRuntime(c)
	a.mediaMu.Lock()
	a.mediaProcessor = next.processor
	a.mediaMu.Unlock()
	if a.toolServices != nil {
		a.toolServices.ReloadMediaTools(a.tools, next.defaultImageProvider, next.processor, next.imageGenerator, next.imageDefaults, next.speechSynthesizer, next.ttsDefaults)
	}
}
