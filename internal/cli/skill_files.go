package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sort"
)

const receiptName = ".judex-install.json"

var skillFiles = []string{"SKILL.md", "manifest.json", "references/commands.md", "references/reporting.md", "references/human-decisions.md"}

type skillReceipt struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Root    string            `json:"root"`
	Files   map[string]string `json:"files"`
}

func fileHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func validReceipt(r skillReceipt, root string) bool {
	if r.Name != "judex" || r.Root != root || len(r.Files) == 0 {
		return false
	}
	allowed := map[string]bool{}
	for _, f := range skillFiles {
		allowed[f] = true
	}
	for name, hash := range r.Files {
		if !allowed[name] || len(hash) != 64 {
			return false
		}
	}
	return true
}
func rootedSkill(dest string) (*os.Root, string, error) {
	absolute, err := filepath.Abs(dest)
	if err != nil {
		return nil, "", err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(absolute)
	return root, absolute, err
}
func atomicSkillWrite(root *os.Root, name string, raw []byte) error {
	temp := ".judex-stage-" + uuid.NewString()
	if err := root.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	defer root.Remove(temp)
	if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	return root.Rename(temp, name)
}
func copySkill(source, dest string) error {
	src, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer src.Close()
	content := map[string][]byte{}
	for _, name := range skillFiles {
		info, err := src.Lstat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("skill file is not regular: %s", name)
		}
		raw, err := src.ReadFile(name)
		if err != nil {
			return err
		}
		content[name] = raw
	}
	var identity struct{ Name, Version, CLI string }
	if err = json.Unmarshal(content["manifest.json"], &identity); err != nil || identity.Name != "judex" || identity.CLI != "judex" || identity.Version == "" {
		return errors.New("invalid Judex skill manifest")
	}
	if err = os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	root, absolute, err := rootedSkill(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	old := skillReceipt{}
	raw, err := root.ReadFile(receiptName)
	if err == nil {
		if json.Unmarshal(raw, &old) != nil || !validReceipt(old, absolute) {
			return errors.New("invalid installation ownership receipt")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Validate every collision before writing any file. User edits are preserved.
	for _, name := range skillFiles {
		current, err := root.ReadFile(name)
		if err == nil {
			if old.Files[name] == "" || fileHash(current) != old.Files[name] {
				return fmt.Errorf("refusing to overwrite unmanaged or modified file: %s", name)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "Installing Judex skill into %s; files: %v\n", absolute, skillFiles)
	receipt := skillReceipt{Name: "judex", Version: identity.Version, Root: absolute, Files: map[string]string{}}
	for _, name := range skillFiles {
		if err = atomicSkillWrite(root, name, content[name]); err != nil {
			return err
		}
		receipt.Files[name] = fileHash(content[name])
	}
	raw, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return atomicSkillWrite(root, receiptName, raw)
}
func removeSkill(dest string) error {
	root, absolute, err := rootedSkill(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	raw, err := root.ReadFile(receiptName)
	if err != nil {
		return errors.New("installation receipt missing; refusing removal")
	}
	var receipt skillReceipt
	if json.Unmarshal(raw, &receipt) != nil || !validReceipt(receipt, absolute) {
		return errors.New("invalid installation ownership receipt")
	}
	for name, expected := range receipt.Files {
		current, err := root.ReadFile(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if fileHash(current) != expected {
			return fmt.Errorf("modified file preserved; uninstall aborted: %s", name)
		}
	}
	names := []string{}
	for name := range receipt.Files {
		names = append(names, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		if err = root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err = root.Remove(receiptName); err != nil {
		return err
	}
	_ = root.Remove("references")
	// Remove only the empty directory. Additional user files are retained.
	root.Close()
	_ = os.Remove(absolute)
	return nil
}
