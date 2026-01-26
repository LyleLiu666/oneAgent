package skill

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func osReadFileLimited(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return os.ReadFile(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("expected file but got directory: %s", path)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file too large: %s", path)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return data, nil
		}
		return nil, err
	}
	return data, nil
}

