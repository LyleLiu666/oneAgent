package sessionstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/model"
)

type Store struct {
	sessionsDir string
	muBySession sync.Map // map[string]*sync.Mutex
}

func New(sessionsDir string) (*Store, error) {
	if strings.TrimSpace(sessionsDir) == "" {
		return nil, errors.New("sessionsDir is required")
	}
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	return &Store{sessionsDir: sessionsDir}, nil
}

func (s *Store) SessionsDir() string {
	return s.sessionsDir
}

type sessionFile struct {
	Version       int               `json:"version"`
	Session       model.ChatSession `json:"session"`
	NextMessageID uint              `json:"next_message_id"`
}

func (s *Store) sessionDir(sessionID string) string {
	return filepath.Join(s.sessionsDir, sessionID)
}

func (s *Store) sessionPath(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "session.json")
}

func (s *Store) messagesPath(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "messages.jsonl")
}

func (s *Store) lock(sessionID string) *sync.Mutex {
	val, _ := s.muBySession.LoadOrStore(sessionID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func (s *Store) GetOrCreateSession(sessionID, userID, module, title string) (model.ChatSession, error) {
	if sessionID == "" {
		return model.ChatSession{}, errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	existing, _, err := s.loadSessionLocked(sessionID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return model.ChatSession{}, err
	}

	if err := os.MkdirAll(s.sessionDir(sessionID), 0o700); err != nil {
		return model.ChatSession{}, err
	}

	now := time.Now()
	session := model.ChatSession{
		ID:        sessionID,
		UserID:    userID,
		Title:     title,
		Module:    module,
		Metadata:  model.JSONB{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	sf := sessionFile{
		Version:       1,
		Session:       session,
		NextMessageID: 1,
	}
	if err := writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600); err != nil {
		return model.ChatSession{}, err
	}

	// Ensure messages file exists.
	if err := touchFile(s.messagesPath(sessionID), 0o600); err != nil {
		return model.ChatSession{}, err
	}

	return session, nil
}

func (s *Store) ListSessions(userID, module string) ([]model.ChatSession, error) {
	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []model.ChatSession{}, nil
		}
		return nil, err
	}

	sessions := make([]model.ChatSession, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionID := e.Name()
		mu := s.lock(sessionID)
		mu.Lock()
		session, _, err := s.loadSessionLocked(sessionID)
		mu.Unlock()
		if err != nil {
			continue
		}
		if userID != "" && session.UserID != userID {
			continue
		}
		if module != "" && session.Module != module {
			continue
		}
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func (s *Store) GetSessionWithMessages(sessionID, userID string) (model.ChatSession, []model.ChatMessage, error) {
	if sessionID == "" {
		return model.ChatSession{}, nil, errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, _, err := s.loadSessionLocked(sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ChatSession{}, nil, os.ErrNotExist
		}
		return model.ChatSession{}, nil, err
	}
	if userID != "" && session.UserID != userID {
		return model.ChatSession{}, nil, os.ErrNotExist
	}

	msgs, err := s.loadMessagesLocked(sessionID)
	if err != nil {
		return model.ChatSession{}, nil, err
	}
	return session, msgs, nil
}

func (s *Store) AppendMessage(sessionID string, msg model.ChatMessage) (model.ChatMessage, error) {
	if sessionID == "" {
		return model.ChatMessage{}, errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, sf, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return model.ChatMessage{}, err
	}

	msg.SessionID = sessionID
	msg.ID = sf.NextMessageID
	sf.NextMessageID++
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	if err := appendJSONLine(s.messagesPath(sessionID), msg, 0o600); err != nil {
		return model.ChatMessage{}, err
	}

	session.UpdatedAt = time.Now()
	sf.Session = session
	if err := writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600); err != nil {
		return model.ChatMessage{}, err
	}

	return msg, nil
}

func (s *Store) ReplaceMessages(sessionID string, msgs []model.ChatMessage, updatedAt time.Time, nextMessageID uint) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, sf, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return err
	}

	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	session.UpdatedAt = updatedAt
	sf.Session = session
	if nextMessageID == 0 {
		sf.NextMessageID = computeNextMessageID(msgs)
	} else {
		sf.NextMessageID = nextMessageID
	}

	if err := writeJSONLAtomic(s.messagesPath(sessionID), msgs, 0o600); err != nil {
		return err
	}
	return writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600)
}

func (s *Store) UpdateSessionMetadata(sessionID string, metadata model.JSONB) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, sf, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return err
	}
	session.Metadata = metadata
	session.UpdatedAt = time.Now()
	sf.Session = session
	return writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600)
}

func (s *Store) UpdateMessageTrace(sessionID string, messageID uint, trace model.TraceDataJSON) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	if messageID == 0 {
		return errors.New("messageID is required")
	}

	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, sf, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return err
	}

	msgs, err := s.loadMessagesLocked(sessionID)
	if err != nil {
		return err
	}

	found := false
	for i := range msgs {
		if msgs[i].ID == messageID {
			msgs[i].Trace = trace
			found = true
			break
		}
	}
	if !found {
		return os.ErrNotExist
	}

	session.UpdatedAt = time.Now()
	sf.Session = session

	if err := writeJSONLAtomic(s.messagesPath(sessionID), msgs, 0o600); err != nil {
		return err
	}
	return writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600)
}

func (s *Store) DeleteSession(sessionID, userID string) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, _, err := s.loadSessionLocked(sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if userID != "" && session.UserID != userID {
		return os.ErrNotExist
	}
	return os.RemoveAll(s.sessionDir(sessionID))
}

func (s *Store) TruncateFromMessageID(sessionID, userID string, fromID uint) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	mu := s.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	session, sf, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return err
	}
	if userID != "" && session.UserID != userID {
		return os.ErrNotExist
	}

	msgs, err := s.loadMessagesLocked(sessionID)
	if err != nil {
		return err
	}
	out := make([]model.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.ID < fromID {
			out = append(out, m)
		}
	}

	sf.NextMessageID = computeNextMessageID(out)
	session.UpdatedAt = time.Now()
	sf.Session = session

	if err := writeJSONLAtomic(s.messagesPath(sessionID), out, 0o600); err != nil {
		return err
	}
	return writeJSONAtomic(s.sessionPath(sessionID), sf, 0o600)
}

func (s *Store) loadSessionLocked(sessionID string) (model.ChatSession, sessionFile, error) {
	data, err := os.ReadFile(s.sessionPath(sessionID))
	if err != nil {
		return model.ChatSession{}, sessionFile{}, err
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return model.ChatSession{}, sessionFile{}, err
	}
	return sf.Session, sf, nil
}

func (s *Store) loadMessagesLocked(sessionID string) ([]model.ChatMessage, error) {
	path := s.messagesPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []model.ChatMessage{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var msgs []model.ChatMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m model.ChatMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return msgs, nil
}

func computeNextMessageID(msgs []model.ChatMessage) uint {
	var max uint
	for _, m := range msgs {
		if m.ID > max {
			max = m.ID
		}
	}
	return max + 1
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSONLAtomic(path string, msgs []model.ChatMessage, mode os.FileMode) error {
	var buf bytes.Buffer
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func appendJSONLine(path string, v any, mode os.FileMode) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func touchFile(path string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE, mode)
	if err != nil {
		return err
	}
	return f.Close()
}

func CopyMessages(dst io.Writer, msgs []model.ChatMessage) error {
	enc := json.NewEncoder(dst)
	for _, m := range msgs {
		if err := enc.Encode(m); err != nil {
			return err
		}
	}
	return nil
}
