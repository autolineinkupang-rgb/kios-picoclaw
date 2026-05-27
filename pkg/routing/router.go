package routing

import (
	"github.com/sipeed/picoclaw/pkg/providers"
)

// defaultThreshold is used when the config threshold is zero or negative.
// At 0.15 only pure greetings / trivial Q&A fall into the light tier.
const defaultThreshold = 0.15

// defaultHeavyThreshold separates medium from heavy.
// At 0.50 messages with code blocks, long text, or attachment reach the heavy tier.
const defaultHeavyThreshold = 0.50

// RouterConfig holds the validated model routing settings.
// It mirrors config.RoutingConfig but lives in pkg/routing to keep the
// dependency graph simple: pkg/agent resolves config → routing, not the reverse.
type RouterConfig struct {
	// LightModel is the model_name (from model_list) used for simple tasks
	// (score < Threshold).
	LightModel string

	// MediumModel is the model_name used for medium tasks
	// (Threshold <= score < HeavyThreshold). Empty = no medium tier.
	MediumModel string

	// Threshold is the low complexity score cutoff in [0, 1].
	// score < Threshold  → light model.
	// score >= Threshold → medium (or primary when MediumModel is empty).
	Threshold float64

	// HeavyThreshold is the high complexity score cutoff in [0, 1].
	// score >= HeavyThreshold → primary (heavy) model.
	// 0 or negative means the primary tier is never selected via routing
	// (all non-light traffic goes to MediumModel).
	HeavyThreshold float64
}

// Router selects the appropriate model tier for each incoming message.
// It is safe for concurrent use from multiple goroutines.
type Router struct {
	cfg        RouterConfig
	classifier Classifier
}

// New creates a Router with the given config and the default RuleClassifier.
// Zero or negative thresholds are replaced with package defaults.
func New(cfg RouterConfig) *Router {
	if cfg.Threshold <= 0 {
		cfg.Threshold = defaultThreshold
	}
	if cfg.HeavyThreshold <= 0 && cfg.MediumModel != "" {
		cfg.HeavyThreshold = defaultHeavyThreshold
	}
	return &Router{
		cfg:        cfg,
		classifier: &RuleClassifier{},
	}
}

// newWithClassifier creates a Router with a custom Classifier.
// Intended for unit tests that need to inject a deterministic scorer.
func newWithClassifier(cfg RouterConfig, c Classifier) *Router {
	if cfg.Threshold <= 0 {
		cfg.Threshold = defaultThreshold
	}
	if cfg.HeavyThreshold <= 0 && cfg.MediumModel != "" {
		cfg.HeavyThreshold = defaultHeavyThreshold
	}
	return &Router{cfg: cfg, classifier: c}
}

// Tier constants returned by SelectModel.
const (
	TierLight   = "light"
	TierMedium  = "medium"
	TierPrimary = "primary"
)

// SelectModel returns the model to use for this conversation turn along with
// the selected tier and the computed complexity score.
//
//   - score < Threshold                       → (LightModel,   TierLight,   score)
//   - Threshold <= score < HeavyThreshold     → (MediumModel,  TierMedium,  score)  [when MediumModel set]
//   - score >= HeavyThreshold (or no medium)  → (primaryModel, TierPrimary, score)
func (r *Router) SelectModel(
	msg string,
	history []providers.Message,
	primaryModel string,
) (model string, tier string, score float64) {
	features := ExtractFeatures(msg, history)
	score = r.classifier.Score(features)

	if score < r.cfg.Threshold {
		return r.cfg.LightModel, TierLight, score
	}
	if r.cfg.MediumModel != "" && (r.cfg.HeavyThreshold <= 0 || score < r.cfg.HeavyThreshold) {
		return r.cfg.MediumModel, TierMedium, score
	}
	return primaryModel, TierPrimary, score
}

// LightModel returns the configured light model name.
func (r *Router) LightModel() string {
	return r.cfg.LightModel
}

// MediumModel returns the configured medium model name.
func (r *Router) MediumModel() string {
	return r.cfg.MediumModel
}

// Threshold returns the low complexity threshold in use.
func (r *Router) Threshold() float64 {
	return r.cfg.Threshold
}

// HeavyThreshold returns the high complexity threshold in use.
func (r *Router) HeavyThreshold() float64 {
	return r.cfg.HeavyThreshold
}
