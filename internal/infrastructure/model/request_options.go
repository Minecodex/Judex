package model

import "fmt"

// RequestOptions are non-secret operator settings, separate from token caps.
// No arbitrary JSON is forwarded to a provider.
type RequestOptions struct {
	ThinkingType    string `json:"thinkingType,omitempty" yaml:"thinkingType,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty" yaml:"reasoningEffort,omitempty"`
}

func (o RequestOptions) Validate(protocol string) error {
	if o.ThinkingType == "" && o.ReasoningEffort == "" {
		return nil
	}
	if protocol != "" && protocol != ProtocolOpenAI {
		return fmt.Errorf("requestOptions require openai-compatible")
	}
	if o.ThinkingType != "" && o.ThinkingType != "enabled" && o.ThinkingType != "disabled" {
		return fmt.Errorf("thinkingType requires openai-compatible and enabled/disabled")
	}
	if o.ReasoningEffort != "" {
		switch o.ReasoningEffort {
		case "low", "medium", "high", "max":
		default:
			return fmt.Errorf("reasoningEffort requires low/medium/high/max supported by the selected model")
		}
	}
	return nil
}

// ConfiguredOptions lets the runner include actual settings in its hashed
// request before durable reservation. It never exposes credentials.
type ConfiguredOptions interface{ RequestOptions() RequestOptions }

func (p *OpenAIProvider) RequestOptions() RequestOptions { return p.cfg.RequestOptions }
