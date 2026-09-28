// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"net/http"
	"time"
)

// Protocol identifiers persisted in the model catalog (02 §8).
const (
	ProtocolOpenAI    = "openai-compatible"
	ProtocolAnthropic = "anthropic-compatible"
)

// GatewayConfig is the protocol-agnostic gateway entry resolved from the
// catalog (baseUrl + key already injected from the Secret/env reference).
type GatewayConfig struct {
	Protocol string // openai-compatible | anthropic-compatible
	BaseURL  string
	APIKey   string
	Timeout  time.Duration // default 10m
}

// NewProvider builds the right adapter for a catalog entry; unknown
// protocols fail loudly instead of falling back to a wrong wire format.
func NewProvider(cfg GatewayConfig) (Provider, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	httpClient := &http.Client{Timeout: cfg.Timeout}
	switch cfg.Protocol {
	case ProtocolOpenAI, "": // historical default
		return NewOpenAIProvider(OpenAIConfig{
			BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, HTTP: httpClient,
		}), nil
	case ProtocolAnthropic:
		return NewAnthropicProvider(AnthropicConfig{
			BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, HTTP: httpClient,
		}), nil
	default:
		return nil, fmt.Errorf("model: unknown protocol %q", cfg.Protocol)
	}
}
