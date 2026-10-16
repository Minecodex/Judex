package model

import "context"

// A target is one provider request, with its own journal attempt and limits.
// The runner alone chooses another target after a confirmed quota error.
type Target struct {
	Provider                        Provider
	Name                            string
	MaxInputTokens, MaxOutputTokens int64
}
type QuotaCandidates interface{ QuotaTargets() []Target }
type quotaProvider struct{ targets []Target }

func (p *quotaProvider) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	return p.targets[0].Provider.Stream(ctx, req)
}
func (p *quotaProvider) QuotaTargets() []Target { return p.targets }
func WithQuotaFallbacks(primary Target, alternatives []Target) Provider {
	return &quotaProvider{targets: append([]Target{primary}, alternatives...)}
}
