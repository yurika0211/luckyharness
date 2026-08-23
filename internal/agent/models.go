package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yurika0211/luckyagent/internal/config"
	"github.com/yurika0211/luckyagent/internal/provider"
)

// ModelRef is a credential-free model description returned to command and API
// callers. Secrets always remain in config.ModelEndpointConfig.
type ModelRef struct {
	ID           string           `json:"id"`
	Kind         config.ModelKind `json:"kind"`
	Provider     string           `json:"provider,omitempty"`
	DisplayName  string           `json:"display_name,omitempty"`
	APIBase      string           `json:"api_base,omitempty"`
	Protocol     string           `json:"protocol,omitempty"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Current      bool             `json:"current"`
}

// SwitchModelOptions controls whether a selection is persisted. Pin is
// accepted by the unified command/API surface; model-router pinning is session
// scoped and deliberately handled by the caller rather than persisted here.
type SwitchModelOptions struct {
	Pin      bool
	Persist  bool
	Provider string
}

// CurrentModel returns the configured selection for a model purpose.
func (a *Agent) CurrentModel(kind config.ModelKind) (ModelRef, bool) {
	if a == nil || a.cfg == nil {
		return ModelRef{}, false
	}
	selection, ok := a.cfg.Get().ModelSelection(kind)
	if !ok {
		return ModelRef{}, false
	}
	ref := ModelRef{
		ID:       selection.ID,
		Kind:     kind,
		Provider: selection.Provider,
		APIBase:  selection.APIBase,
		Protocol: selection.Protocol,
		Current:  true,
	}
	if catalog := a.Catalog(); catalog != nil {
		if info, err := catalog.Get(selection.ID); err == nil {
			ref.DisplayName = info.DisplayName
			ref.Capabilities = append([]string(nil), info.Capabilities...)
			if ref.Provider == "" {
				ref.Provider = info.Provider
			}
		}
	}
	return ref, true
}

// ListModels lists catalog models alongside each currently selected non-chat
// model. A kind filter makes the result suitable for `/models <kind>` and the
// HTTP configuration center.
func (a *Agent) ListModels(kind *config.ModelKind) []ModelRef {
	if a == nil || a.cfg == nil {
		return nil
	}
	cfg := a.cfg.Get()
	result := make([]ModelRef, 0)
	for _, modelKind := range config.ModelKinds() {
		if kind != nil && *kind != modelKind {
			continue
		}
		selection, selected := cfg.ModelSelection(modelKind)
		if selected {
			result = append(result, ModelRef{
				ID:       selection.ID,
				Kind:     modelKind,
				Provider: selection.Provider,
				APIBase:  selection.APIBase,
				Protocol: selection.Protocol,
				Current:  true,
			})
		}
	}

	if catalog := a.Catalog(); catalog != nil {
		for _, info := range catalog.List() {
			for _, modelKind := range modelKindsForInfo(cfg, info) {
				if kind != nil && *kind != modelKind {
					continue
				}
				if selected, ok := cfg.ModelSelection(modelKind); ok && selected.ID == info.ID {
					continue
				}
				result = append(result, ModelRef{
					ID:           info.ID,
					Kind:         modelKind,
					Provider:     info.Provider,
					DisplayName:  info.DisplayName,
					Capabilities: append([]string(nil), info.Capabilities...),
				})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		if result[i].Current != result[j].Current {
			return result[i].Current
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// ModelProfiles returns a copy of the configured named selections.
func (a *Agent) ModelProfiles() map[string]map[config.ModelKind]string {
	if a == nil || a.cfg == nil {
		return nil
	}
	profiles := a.cfg.Get().Models.Profiles
	copyProfiles := make(map[string]map[config.ModelKind]string, len(profiles))
	for name, profile := range profiles {
		copyProfile := make(map[config.ModelKind]string, len(profile))
		for kind, modelID := range profile {
			copyProfile[kind] = modelID
		}
		copyProfiles[name] = copyProfile
	}
	return copyProfiles
}

// SwitchModelKind validates and applies a selection. When Persist is true,
// disk state changes before the provider snapshot is exposed, so a write
// failure leaves both the configured and active selection unchanged.
func (a *Agent) SwitchModelKind(kind config.ModelKind, modelID string, opts SwitchModelOptions) error {
	if a == nil || a.cfg == nil {
		return fmt.Errorf("agent configuration is unavailable")
	}
	if _, err := config.ParseModelKind(string(kind)); err != nil {
		return err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("model id is required")
	}

	current := a.cfg.Get()
	modelInfo, err := a.validateModelKind(current, kind, modelID)
	if err != nil {
		return err
	}
	next := config.Clone(current)
	endpoint := config.ModelEndpointConfig{Provider: strings.TrimSpace(opts.Provider)}
	if endpoint.Provider == "" && modelInfo != nil {
		endpoint.Provider = modelInfo.Provider
	}
	if err := next.SetModelSelection(kind, modelID, endpoint); err != nil {
		return err
	}
	if err := a.ValidateRuntimeConfig(next); err != nil {
		return fmt.Errorf("validate model selection: %w", err)
	}
	if opts.Persist {
		if err := a.cfg.Replace(next); err != nil {
			return fmt.Errorf("save model selection: %w", err)
		}
	}
	if err := a.ApplyRuntimeConfig(next); err != nil {
		return fmt.Errorf("apply model selection: %w", err)
	}
	return nil
}

// ApplyModelProfile applies an existing profile atomically. A profile only
// changes model IDs; each kind retains its independent endpoint credentials.
func (a *Agent) ApplyModelProfile(name string) error {
	if a == nil || a.cfg == nil {
		return fmt.Errorf("agent configuration is unavailable")
	}
	name = strings.TrimSpace(name)
	current := a.cfg.Get()
	profile, ok := current.Models.Profiles[name]
	if !ok {
		return fmt.Errorf("model profile %q does not exist", name)
	}
	next := config.Clone(current)
	for kind, modelID := range profile {
		if _, err := a.validateModelKind(next, kind, modelID); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
		if err := next.SetModelSelection(kind, modelID, config.ModelEndpointConfig{}); err != nil {
			return err
		}
	}
	if err := a.ValidateRuntimeConfig(next); err != nil {
		return fmt.Errorf("validate model profile: %w", err)
	}
	if err := a.cfg.Replace(next); err != nil {
		return fmt.Errorf("save model profile: %w", err)
	}
	if err := a.ApplyRuntimeConfig(next); err != nil {
		return fmt.Errorf("apply model profile: %w", err)
	}
	return nil
}

func (a *Agent) validateModelKind(cfg *config.Config, kind config.ModelKind, modelID string) (*provider.ModelInfo, error) {
	for _, custom := range cfg.CustomModels {
		if custom.ID != modelID {
			continue
		}
		kinds := custom.Kinds
		if len(kinds) == 0 {
			kinds = []config.ModelKind{config.ModelKindChat}
		}
		for _, allowed := range kinds {
			if allowed == kind {
				return &provider.ModelInfo{ID: custom.ID, Provider: custom.Provider, DisplayName: custom.DisplayName, Capabilities: append([]string(nil), custom.Capabilities...)}, nil
			}
		}
		return nil, fmt.Errorf("model %q is not configured for %s", modelID, kind)
	}

	if kind != config.ModelKindChat {
		return nil, nil
	}
	if catalog := a.Catalog(); catalog != nil {
		info, err := catalog.Get(modelID)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("chat model %q is not registered in the model catalog", modelID)
}

func modelKindsForInfo(cfg *config.Config, info provider.ModelInfo) []config.ModelKind {
	for _, custom := range cfg.CustomModels {
		if custom.ID != info.ID {
			continue
		}
		if len(custom.Kinds) == 0 {
			return []config.ModelKind{config.ModelKindChat}
		}
		return append([]config.ModelKind(nil), custom.Kinds...)
	}
	return []config.ModelKind{config.ModelKindChat}
}
