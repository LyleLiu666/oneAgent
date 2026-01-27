package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

type FilePreconditions struct {
	ExpectedExists *bool  `json:"expected_exists,omitempty"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
}

func fileSHA256Hex(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func checkFilePreconditions(path string, p *FilePreconditions) error {
	if p == nil {
		return nil
	}

	if p.ExpectedExists != nil {
		_, err := os.Stat(path)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if exists != *p.ExpectedExists {
			return fmt.Errorf("precondition failed: expected_exists=%v but exists=%v", *p.ExpectedExists, exists)
		}
	}

	if p.ExpectedSHA256 != "" {
		got, err := fileSHA256Hex(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("precondition failed: expected_sha256=%s but file does not exist", p.ExpectedSHA256)
			}
			return err
		}
		if got != p.ExpectedSHA256 {
			return fmt.Errorf("precondition failed: file changed (expected_sha256=%s got_sha256=%s)", p.ExpectedSHA256, got)
		}
	}

	return nil
}

