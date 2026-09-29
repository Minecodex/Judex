package cli

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func materialDownloadCommand() *cobra.Command {
	var version, output, entry string
	cmd := &cobra.Command{Use: "download", Short: "下载固定版本并校验 SHA256", RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var v struct {
			ID, MaterialID, SHA256 string
			Size                   int64
			Entries                []struct {
				RelativePath, SHA256 string
				Size                 int64
			} `json:"entries"`
		}
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/material-versions/"+version, nil, &v, ""); err != nil {
			return emit(nil, err)
		}
		expected, size := v.SHA256, v.Size
		if entry != "" {
			found := false
			for _, item := range v.Entries {
				if item.RelativePath == entry {
					expected, size = item.SHA256, item.Size
					found = true
					break
				}
			}
			if !found {
				return emit(nil, fmt.Errorf("entry outside version manifest"))
			}
		}
		if _, err = os.Lstat(output); err == nil {
			return emit(nil, fmt.Errorf("output exists; choose a new path"))
		} else if !os.IsNotExist(err) {
			return emit(nil, err)
		}
		parent := filepath.Dir(output)
		file, err := os.CreateTemp(parent, ".judex-download-*")
		if err != nil {
			return emit(nil, err)
		}
		temp := file.Name()
		defer os.Remove(temp)
		hash := sha256.New()
		err = c.Download(cmd.Context(), project, v.MaterialID, version, entry, io.MultiWriter(file, hash))
		stat, statErr := file.Stat()
		closeErr := file.Close()
		if err != nil {
			return emit(nil, err)
		}
		if statErr != nil {
			return emit(nil, statErr)
		}
		if closeErr != nil {
			return emit(nil, closeErr)
		}
		if stat.Size() != size || hex.EncodeToString(hash.Sum(nil)) != expected {
			return emit(nil, fmt.Errorf("download checksum or length mismatch"))
		}
		if err = os.Link(temp, output); err != nil {
			return emit(nil, err)
		}
		return emit(map[string]any{"versionId": version, "output": output, "sha256": expected, "verified": true}, nil)
	}}
	cmd.Flags().StringVar(&version, "version", "", "不可变版本 ID")
	cmd.Flags().StringVar(&output, "output", "", "新输出文件路径")
	cmd.Flags().StringVar(&entry, "entry", "", "HTML 包中的单个条目（缺省下载原始包）")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

type bundleEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func bundleCommand() *cobra.Command {
	root := &cobra.Command{Use: "bundle", Short: "可审阅的 HTML 目录包"}
	var entry string
	var dry bool
	command := &cobra.Command{Use: "upload DIR", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		absolute, err := filepath.Abs(args[0])
		if err != nil {
			return emit(nil, err)
		}
		entries := []bundleEntry{}
		total := int64(0)
		err = filepath.WalkDir(absolute, func(file string, info os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(absolute, file)
			if err != nil {
				return err
			}
			if relative == "." {
				return nil
			}
			if info.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink not allowed in bundle: %s", relative)
			}
			if info.IsDir() {
				if info.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if info.Name() == ".env" || strings.HasPrefix(info.Name(), ".env.") || info.Name() == "id_rsa" || info.Name() == "credentials.json" {
				return fmt.Errorf("private file cannot enter bundle: %s", relative)
			}
			metadata, err := info.Info()
			if err != nil {
				return err
			}
			if !metadata.Mode().IsRegular() {
				return fmt.Errorf("regular files required")
			}
			total += metadata.Size()
			if total > 200<<20 || len(entries) >= 2000 {
				return fmt.Errorf("bundle exceeds file count or byte limit")
			}
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			h := sha256.New()
			_, err = io.Copy(h, f)
			f.Close()
			if err != nil {
				return err
			}
			entries = append(entries, bundleEntry{Path: filepath.ToSlash(relative), Size: metadata.Size(), SHA256: hex.EncodeToString(h.Sum(nil))})
			return nil
		})
		if err != nil {
			return emit(nil, err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		found := false
		for _, item := range entries {
			found = found || item.Path == entry
		}
		if !found {
			return emit(nil, fmt.Errorf("entrypoint is missing from bundle"))
		}
		manifest := map[string]any{"root": absolute, "entrypoint": entry, "files": entries, "totalBytes": total}
		if dry {
			return emit(manifest, nil)
		}
		fmt.Fprintln(os.Stderr, "将上传目录清单：", manifest)
		source, err := os.OpenRoot(absolute)
		if err != nil {
			return emit(nil, err)
		}
		defer source.Close()
		archive, err := os.CreateTemp("", "judex-bundle-*.zip")
		if err != nil {
			return emit(nil, err)
		}
		defer os.Remove(archive.Name())
		writer := zip.NewWriter(archive)
		for _, item := range entries {
			header := &zip.FileHeader{Name: item.Path, Method: zip.Deflate}
			header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
			output, err := writer.CreateHeader(header)
			if err != nil {
				writer.Close()
				archive.Close()
				return emit(nil, err)
			}
			file, err := source.Open(item.Path)
			if err != nil {
				writer.Close()
				archive.Close()
				return emit(nil, err)
			}
			hash := sha256.New()
			size, err := io.Copy(io.MultiWriter(output, hash), file)
			file.Close()
			if err != nil || size != item.Size || hex.EncodeToString(hash.Sum(nil)) != item.SHA256 {
				writer.Close()
				archive.Close()
				return emit(nil, fmt.Errorf("bundle file changed during packaging: %s", item.Path))
			}
		}
		err = writer.Close()
		closeErr := archive.Close()
		if err != nil {
			return emit(nil, err)
		}
		if closeErr != nil {
			return emit(nil, closeErr)
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		version, err := uploadFile(cmd.Context(), c, project, archive.Name(), "html_bundle", entry, filepath.Base(absolute)+".zip")
		return emit(version, err)
	}}
	command.Flags().StringVar(&entry, "entry", "index.html", "包内入口路径")
	command.Flags().BoolVar(&dry, "dry-run", false, "只生成清单，不上传")
	root.AddCommand(command)
	return root
}
