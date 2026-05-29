package agent

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	kios "github.com/sipeed/picoclaw/pkg/tools/kios"
)

// kiosModelOverrideHook implements LLMInterceptor so that the model AI
// configured by the owner (KiosConfig.ModelUtama, stored in Redis) takes
// precedence over the default routing in config.json for every LLM call.
// When ModelUtama is empty the hook is a no-op and routing proceeds normally.
type kiosModelOverrideHook struct {
	store *kios.Store
	cfg   *config.Config
}

func (h *kiosModelOverrideHook) BeforeLLM(ctx context.Context, req *LLMHookRequest) (*LLMHookRequest, HookDecision, error) {
	model := strings.TrimSpace(h.store.GetConfig(ctx).ModelUtama)
	if model == "" {
		return req, HookDecision{}, nil
	}
	// Validate that model is a known model_name in the config. Without this,
	// a stale/invalid value in Redis (e.g. "claude-opus-4-6" instead of the
	// stable "claude-opus") causes lookupModelConfigByRef to fail the provider
	// check, leaving activeProvider pointing at whatever the router selected
	// (e.g. Gemini), which then receives the Claude model ID and returns 404.
	if h.cfg != nil {
		if _, err := h.cfg.GetModelConfig(model); err != nil {
			logger.WarnCF("kios", "ModelUtama tidak dikenal di config, override diabaikan — pilih ulang model via /model atau dashboard",
				map[string]any{"model_utama": model})
			return req, HookDecision{}, nil
		}
	}
	cloned := req.Clone()
	cloned.Model = model
	return cloned, HookDecision{}, nil
}

func (h *kiosModelOverrideHook) AfterLLM(_ context.Context, resp *LLMHookResponse) (*LLMHookResponse, HookDecision, error) {
	return resp, HookDecision{}, nil
}
