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
