package collaboration

import (
	"regexp"
	"strings"
	"unicode/utf8"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// A transport-complete model response can still be an unresolved template.
// Reject placeholder-only summaries; literal examples inside real prose remain
// valid. This is a syntax guard, not a claim to judge the analysis's correctness.
var placeholderSummary = regexp.MustCompile(`^(?:(?:__[A-Za-z0-9_]+__|\{\{[^{}]+\}\}|\[\[[^\[\]]+\]\]|<\|[^<>]+\|>)[\s\p{P}]*)+$`)

func ValidateAnalysisSummary(summary string) error {
	value := strings.TrimSpace(summary)
	if value == "" || utf8.RuneCountInString(value) > 8000 {
		return apierrors.Fields("summary", "public summary must contain 1 to 8000 characters")
	}
	if placeholderSummary.MatchString(value) {
		return apierrors.Fields("summary", "unresolved placeholder; provide a public conclusion, disagreements and missing evidence from the actual source")
	}
	return nil
}
