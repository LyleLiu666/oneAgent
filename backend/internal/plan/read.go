package plan

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

func fileContains(path string, needle string, maxBytes int64) (bool, error) {
	if needle == "" {
		return true, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("expected file but got directory: %s", path)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return false, fmt.Errorf("file too large for must_contain check: %s", path)
	}

	reader := bufio.NewReaderSize(f, 32*1024)
	needleBytes := []byte(needle)
	if len(needleBytes) == 0 {
		return true, nil
	}

	overlap := len(needleBytes) - 1
	if overlap < 0 {
		overlap = 0
	}
	var prev []byte

	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			buf := chunk
			if len(prev) > 0 {
				buf = append(prev, chunk...)
			}
			if bytes.Contains(buf, needleBytes) {
				return true, nil
			}
			if overlap > 0 {
				if len(buf) >= overlap {
					prev = append([]byte(nil), buf[len(buf)-overlap:]...)
				} else {
					prev = append([]byte(nil), buf...)
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return false, err
		}
	}

	return false, nil
}

