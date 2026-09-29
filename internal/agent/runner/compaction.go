package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// RequestBytes is a conservative token upper bound, including tool arguments
// and schemas. It avoids treating a large tool call as zero context cost.
func RequestBytes(messages []model.Message, tools []model.ToolSchema) int64 {
	raw, _ := json.Marshal(map[string]any{"messages": messages, "tools": tools})
	return int64(len(raw))
}

// Compact replaces only exploratory tool payloads with durable references.
// Human requests, formal context, assistant reasoning and position opinions
// remain verbatim. Raw tool results stay in PostgreSQL; no source is deleted.
func Compact(messages []model.Message, run string, limit int64, schemas []model.ToolSchema) ([]model.Message, bool) {
	if RequestBytes(messages, schemas) <= limit {
		return messages, false
	}
	out := append([]model.Message{}, messages...)
	names := map[string]string{}
	for _, m := range messages {
		for _, call := range m.ToolCalls {
			names[call.ID] = call.Name
		}
	}
	changed := false
	for i, m := range out {
		if m.Role != "tool" || len(m.Content) < 1024 {
			continue
		}
		switch names[m.ToolCallID] {
		case "bash", "read", "read_material", "query_work", "read_context":
		default:
			continue
		}
		hash := sha256.Sum256([]byte(m.Content))
		out[i].Content = fmt.Sprintf(`{"archived":true,"runId":%q,"toolCallId":%q,"sha256":%q,"bytes":%d,"note":"原始探索结果已持久保存。需要细节时用 read_context 分段回读；此索引不表示结果已被采纳或批准。"}`, run, m.ToolCallID, hex.EncodeToString(hash[:]), len(m.Content))
		changed = true
		if RequestBytes(out, schemas) <= limit {
			break
		}
	}
	return out, changed
}
