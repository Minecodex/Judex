package context

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Private configuration may guide an opinion, but is never a public label or
// quoted source. This guard blocks exact private text and opaque leading
// labels before an assistant message/tool argument reaches any journal or tool.
// It does not claim to recognize paraphrases or infer arbitrary sensitive facts.
var ErrPrivateContextDisclosure = errors.New("private_context_disclosure: model output echoed private context; review required")

var privateLabel = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]{11,})[:：]`)

type PublicOutputGuard struct {
	fragments map[string]struct{}
}

// Add retains prior private values for this run, including when a fresh model
// boundary rereads a revised preference. Nothing here is persisted or exported.
func (g *PublicOutputGuard) Add(manifest Manifest) {
	for _, layer := range manifest.Layers {
		if !layer.Private {
			continue
		}
		value := strings.TrimSpace(strings.ReplaceAll(layer.Content, "\r\n", "\n"))
		if g.fragments == nil {
			g.fragments = map[string]struct{}{}
		}
		// Short ordinary style words are not secrets merely because they occur
		// in a preference. Full private sentences and opaque labels are guarded.
		if utf8.RuneCountInString(value) >= 16 {
			g.fragments[strings.ToLower(value)] = struct{}{}
		}
		if match := privateLabel.FindStringSubmatch(value); len(match) == 2 && strings.ContainsAny(match[1], "_-") {
			g.fragments[strings.ToLower(match[1])] = struct{}{}
		}
	}
}

func (g *PublicOutputGuard) Check(text string) error {
	value := strings.ToLower(strings.ReplaceAll(text, "\r\n", "\n"))
	for fragment := range g.fragments {
		if strings.Contains(value, fragment) {
			return ErrPrivateContextDisclosure
		}
	}
	return nil
}

// Decode tool arguments first: JSON escaping must not bypass a check. Malformed
// JSON still receives a raw check; the existing tool validator handles syntax.
func (g *PublicOutputGuard) CheckJSON(raw string) error {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return g.Check(raw)
	}
	var check func(any) error
	check = func(value any) error {
		switch current := value.(type) {
		case string:
			return g.Check(current)
		case []any:
			for _, item := range current {
				if err := check(item); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, item := range current {
				if err := g.Check(key); err != nil {
					return err
				}
				if err := check(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check(value)
}
