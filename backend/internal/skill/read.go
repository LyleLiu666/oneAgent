package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/builtinskills"
)

const builtinPathPrefix = "builtin:"

func builtinPath(relPath string) string {
	rel := strings.TrimSpace(relPath)
	rel = path.Clean(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		return builtinPathPrefix
	}
	return builtinPathPrefix + rel
}

func IsBuiltinPath(p string) bool {
	_, ok := builtinFSPath(p)
	return ok
}

func ReadSkillFile(p string, maxBytes int64) ([]byte, error) {
	return readSkillFile(p, maxBytes)
}

func builtinFSPath(fullPath string) (string, bool) {
	p := strings.TrimSpace(fullPath)
	if !strings.HasPrefix(p, builtinPathPrefix) {
		return "", false
	}
	rel := strings.TrimPrefix(p, builtinPathPrefix)
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "./")
	rel = path.Clean(rel)
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

func readSkillFileFS(fsys fs.ReadFileFS, path string, maxBytes int64) ([]byte, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file too large: %s", path)
	}
	if len(data) == 0 {
		return nil, errors.New("empty file")
	}
	return data, nil
}

func readSkillFile(path string, maxBytes int64) ([]byte, error) {
	if rel, ok := builtinFSPath(path); ok {
		return readSkillFileFS(builtinskills.FS, rel, maxBytes)
	}
	data, err := osReadFileLimited(path, maxBytes)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty file")
	}
	return data, nil
}
