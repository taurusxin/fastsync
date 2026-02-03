package sync

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/taurusxin/fastsync/pkg/protocol"
)

// Helper to check if file matches any exclude pattern
func isExcluded(path string, root string, excludes []string) bool {
	if len(excludes) == 0 {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	// Normalize separators
	rel = filepath.ToSlash(rel)

	for _, pattern := range excludes {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		// Simple match: if pattern matches the relative path or part of it
		// Usually rsync exclude is complex. We will implement simple glob match on the name or path.
		// If pattern contains /, match against full relative path.
		// If pattern does not contain /, match against basename.

		matchName := filepath.Base(path)
		if strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, rel); matched {
				return true
			}
		} else {
			if matched, _ := filepath.Match(pattern, matchName); matched {
				return true
			}
		}
	}
	return false
}

func Scan(root string, excludes []string, calcHash bool) ([]protocol.FileInfo, error) {
	var files []protocol.FileInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if isExcluded(path, root, excludes) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// We want relative paths in the FileInfo
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		fi := protocol.FileInfo{
			Path:    filepath.ToSlash(rel),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			Mode:    uint32(info.Mode()),
			IsDir:   info.IsDir(),
		}

		if calcHash && !info.IsDir() {
			hash, err := CalculateHash(path)
			if err == nil {
				fi.Hash = hash
			}
		}

		files = append(files, fi)
		return nil
	})
	return files, err
}

func CalculateHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
