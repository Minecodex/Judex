package runner

import (
	"encoding/json"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// The final successfully persisted coordinator record is its public result.
// No extra model call is needed merely to repeat it. Pending/unknown effects
// and failed position opinions are handled before this boundary.
func completedAnalysis(calls []model.ToolCall, results []model.Message) (string, bool) {
	if len(calls) == 0 || len(results) != len(calls) {
		return "", false
	}
	last := calls[len(calls)-1]
	if last.Name != "record_task_analysis" {
		return "", false
	}
	var result struct {
		Error any `json:"error"`
		Data  struct {
			AnalysisID string `json:"analysisId"`
		} `json:"data"`
	}
	var args struct {
		Summary string `json:"summary"`
	}
	if json.Unmarshal([]byte(results[len(results)-1].Content), &result) != nil || result.Error != nil || result.Data.AnalysisID == "" || json.Unmarshal([]byte(last.Arguments), &args) != nil || args.Summary == "" {
		return "", false
	}
	return args.Summary, true
}
