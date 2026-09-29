package objectstore

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ArchiveEntry struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func (s *Store) inventory(ctx context.Context) ([]ArchiveEntry, error) {
	pager := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket)})
	out := []ArchiveEntry{}
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			out = append(out, ArchiveEntry{Key: aws.ToString(obj.Key), Size: aws.ToInt64(obj.Size)})
		}
	}
	return out, nil
}

// Export writes object bytes and their hashes, never a best-effort inventory.
func (s *Store) Export(ctx context.Context, w io.Writer) error {
	entries, err := s.inventory(ctx)
	if err != nil {
		return err
	}
	archive := tar.NewWriter(w)
	for i, item := range entries {
		if !safeArchiveKey(item.Key) {
			return fmt.Errorf("unsafe object key in archive")
		}
		if err = archive.WriteHeader(&tar.Header{Name: "objects/" + item.Key, Mode: 0600, Size: item.Size, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		body, err := s.Get(ctx, item.Key)
		if err != nil {
			return err
		}
		hash := sha256.New()
		n, err := io.Copy(io.MultiWriter(archive, hash), body)
		body.Close()
		if err != nil {
			return err
		}
		if n != item.Size {
			return fmt.Errorf("object size changed during backup")
		}
		entries[i].SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	manifest, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if err = archive.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifest)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err = archive.Write(manifest); err != nil {
		return err
	}
	return archive.Close()
}
func safeArchiveKey(key string) bool {
	return key != "" && !strings.HasPrefix(key, "/") && !strings.ContainsAny(key, "\\\x00\r\n") && path.Clean(key) == key && key != ".." && !strings.HasPrefix(key, "../")
}

// Import requires a fresh destination bucket, restores bytes and verifies
// every restored object against the archive manifest before reporting success.
func (s *Store) Import(ctx context.Context, r io.Reader) error {
	existing, err := s.inventory(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("restore requires an empty destination bucket")
	}
	archive := tar.NewReader(r)
	seen := map[string]ArchiveEntry{}
	var manifest []ArchiveEntry
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > 1<<30 {
			return fmt.Errorf("unsupported archive entry")
		}
		if header.Name == "manifest.json" {
			if manifest != nil {
				return fmt.Errorf("duplicate manifest")
			}
			raw, err := io.ReadAll(io.LimitReader(archive, 16<<20))
			if err != nil {
				return err
			}
			if err = json.Unmarshal(raw, &manifest); err != nil {
				return err
			}
			continue
		}
		key := strings.TrimPrefix(header.Name, "objects/")
		if key == header.Name || !safeArchiveKey(key) {
			return fmt.Errorf("unsafe archive entry")
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate archive object")
		}
		entry, err := s.importEntry(ctx, key, header.Size, archive)
		if err != nil {
			return err
		}
		seen[key] = entry
	}
	if manifest == nil || len(manifest) != len(seen) {
		return fmt.Errorf("archive missing complete manifest")
	}
	for _, expected := range manifest {
		actual, ok := seen[expected.Key]
		if !ok || actual.Size != expected.Size || actual.SHA256 != expected.SHA256 {
			return fmt.Errorf("archive checksum mismatch")
		}
		body, err := s.Get(ctx, expected.Key)
		if err != nil {
			return err
		}
		hash := sha256.New()
		n, err := io.Copy(hash, body)
		body.Close()
		if err != nil {
			return err
		}
		if n != expected.Size || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
			return fmt.Errorf("restored object checksum mismatch")
		}
	}
	return nil
}
func (s *Store) importEntry(ctx context.Context, key string, size int64, r io.Reader) (ArchiveEntry, error) {
	out := ArchiveEntry{Key: key, Size: size}
	file, err := os.CreateTemp("", "judex-object-restore-*")
	if err != nil {
		return out, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(r, size))
	if err != nil {
		return out, err
	}
	if n != size {
		return out, io.ErrUnexpectedEOF
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return out, err
	}
	if err = s.Put(ctx, key, file, size, "application/octet-stream"); err != nil {
		return out, err
	}
	out.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return out, nil
}
