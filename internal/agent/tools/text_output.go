package tools

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"unicode/utf8"
)

// Binary command output is evidence, not JSONB-safe natural language.
// Preserve bounded bytes reversibly and make the representation explicit.
func commandText(raw []byte) (string, map[string]any) {
	if utf8.Valid(raw) && !bytes.ContainsRune(raw, 0) {
		return string(raw), nil
	}
	hash := sha256.Sum256(raw)
	binary := map[string]any{"encoding": "base64", "size": len(raw), "sha256": hex.EncodeToString(hash[:]), "note": "二进制输出，不能作为原文解读；固定材料用 analyze_material 读取结构。"}
	if len(raw) <= 256<<10 {
		binary["data"] = base64.StdEncoding.EncodeToString(raw)
	} else {
		binary["omitted"] = true
		binary["note"] = "二进制输出超过内联上限，保留大小与 SHA；按需在沙箱分段读取，不作为原文解读。"
	}
	return "", binary
}
