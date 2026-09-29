package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillInstallOwnsOnlyUnmodifiedPackageFiles(t *testing.T) {
	source := t.TempDir()
	for _, name := range skillFiles {
		target := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		content := []byte("fixture " + name)
		if name == "manifest.json" {
			content = []byte(`{"name":"judex","version":"1.0.0","cli":"judex"}`)
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(t.TempDir(), "judex")
	if err := copySkill(source, dest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "personal-notes.md"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(dest, "SKILL.md")
	if err := os.WriteFile(edited, []byte("user customization"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copySkill(source, dest); err == nil {
		t.Fatal("overwrote user skill")
	}
	if err := removeSkill(dest); err == nil {
		t.Fatal("removed user customization")
	}
	original, _ := os.ReadFile(filepath.Join(source, "SKILL.md"))
	if err := os.WriteFile(edited, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeSkill(dest); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(dest, "personal-notes.md")); err != nil || string(raw) != "keep" {
		t.Fatal("uninstall removed unrelated file")
	}
	// A copied/malicious manifest cannot authorize deletion of arbitrary files.
	receipt := skillReceipt{Name: "judex", Root: dest, Files: map[string]string{"personal-notes.md": fileHash([]byte("keep"))}}
	raw, _ := json.Marshal(receipt)
	if err := os.WriteFile(filepath.Join(dest, receiptName), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeSkill(dest); err == nil {
		t.Fatal("untrusted receipt escaped file ownership whitelist")
	}
}
