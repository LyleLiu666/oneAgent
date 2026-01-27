package fsutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// AtomicWriteFile writes file content by writing a temp file in the same directory
// and then renaming it over the target path.
//
// Notes:
// - Same-dir rename is required for atomicity.
// - This is best-effort: it will try to clean up temp on failure.
func AtomicWriteFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}

	tmp := path + ".tmp." + uuid.NewString()
	if err := os.WriteFile(tmp, content, mode); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
