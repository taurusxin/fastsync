package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SecureJoin(root, unsafePath string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.Clean(unsafePath))
	if !strings.HasPrefix(path, root) {
		return "", fmt.Errorf("path traversal attempt: %s", unsafePath)
	}
	return path, nil
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
