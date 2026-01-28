package runtime

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"sync"
	"time"
)

type PairingCode struct {
	Code        string
	PrincipalID string
	ExpiresAt   time.Time
	Used        bool
	UsedAt      time.Time
}

type PairingService struct {
	mu   sync.Mutex
	now  func() time.Time
	codes map[string]PairingCode
}

func NewPairingService() *PairingService {
	return &PairingService{
		now:   time.Now,
		codes: make(map[string]PairingCode),
	}
}

func (s *PairingService) SetNow(now func() time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if now == nil {
		s.now = time.Now
		return
	}
	s.now = now
}

func (s *PairingService) Create(principalID string, ttl time.Duration) (PairingCode, error) {
	if s == nil {
		return PairingCode{}, errors.New("pairing service not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return PairingCode{}, errors.New("principal_id is required")
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if ttl > 10*time.Minute {
		ttl = 10 * time.Minute
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	code := newPairingCode()
	now := s.now()
	pc := PairingCode{
		Code:        code,
		PrincipalID: principalID,
		ExpiresAt:   now.Add(ttl),
	}
	s.codes[code] = pc
	return pc, nil
}

func (s *PairingService) Exchange(code string) (PairingCode, error) {
	if s == nil {
		return PairingCode{}, errors.New("pairing service not initialized")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return PairingCode{}, errors.New("pairing code is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pc, ok := s.codes[code]
	if !ok || strings.TrimSpace(pc.Code) == "" {
		return PairingCode{}, errors.New("pairing code not found")
	}
	now := s.now()
	if !pc.ExpiresAt.IsZero() && now.After(pc.ExpiresAt) {
		delete(s.codes, code)
		return PairingCode{}, errors.New("pairing code expired")
	}
	if pc.Used {
		return PairingCode{}, errors.New("pairing code already used")
	}

	pc.Used = true
	pc.UsedAt = now
	s.codes[code] = pc
	return pc, nil
}

func newPairingCode() string {
	// 10 bytes -> 16 base32 chars (no padding) -> upper-case, unambiguous enough.
	var buf [10]byte
	_, _ = rand.Read(buf[:])
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	out := enc.EncodeToString(buf[:])
	out = strings.TrimSpace(out)
	out = strings.TrimRight(out, "=")
	return out
}

