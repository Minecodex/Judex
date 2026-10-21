package peer

import (
	"encoding/json"
	"strings"

	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Tool protocol is a private recovery log, not a second knowledge store. Once
// every effect has a paired result, earlier calls can be represented by their
// immutable public references. The full journal remains in PostgreSQL.
func compactExecution(r *run, capacity int64) bool {
	raw, _ := json.Marshal(r.Checkpoint)
	if collab.Tokens(string(raw)) <= capacity/5 {
		return false
	}
	answered := map[string]bool{}
	for _, m := range r.Checkpoint {
		if m.Role == "tool" {
			answered[m.ToolCallID] = true
		}
	}
	refs := []string{}
	for _, m := range r.Checkpoint {
		for _, c := range m.ToolCalls {
			if !answered[c.ID] {
				return false
			}
			refs = append(refs, c.Name+" -> tool / "+r.Publication.RunID.String()+":"+c.ID)
		}
	}
	if len(refs) == 0 {
		return false
	}
	if len(refs) > 20 {
		refs = refs[len(refs)-20:]
	}
	r.Checkpoint = []model.Message{{Role: "user", Content: "Execution protocol compacted after all tool results were recorded. Completed actions must not be repeated merely because their protocol was compacted. Their results are part of the shared group knowledge; use read_context to inspect exact originals. Recent original references:\n" + strings.Join(refs, "\n")}}
	return true
}
