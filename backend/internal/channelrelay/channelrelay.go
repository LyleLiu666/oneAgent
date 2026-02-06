package channelrelay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ProviderWebhookV1 = "webhook.v1"

	TraceFileName = "channel_relay.jsonl"

	TaskLinkEventType = "task.channel_relay.linked"
)

type InboundMessage struct {
	Provider     string
	PrincipalID  string
	ChannelID    string
	ThreadID     string
	MessageID    string
	Workspace    string
	Content      string
	ReceivedAt   time.Time
	SignatureHex string
}

type TraceEntry struct {
	TS string `json:"ts"`

	Direction  string `json:"direction"` // inbound|outbound
	Provider   string `json:"provider,omitempty"`
	Principal  string `json:"principal_id,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	ThreadID   string `json:"thread_id,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	AttemptID  string `json:"attempt_id,omitempty"`
	TaskStatus string `json:"task_status,omitempty"`

	Idempotent bool   `json:"idempotent,omitempty"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

func VerifySignature(secret string, body []byte, sigHex string) bool {
	secret = strings.TrimSpace(secret)
	sigHex = strings.TrimSpace(sigHex)
	if secret == "" || sigHex == "" {
		return false
	}
	want := SignBody(secret, body)
	gotBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	return hmac.Equal(wantBytes, gotBytes)
}

func SignBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func DeriveSessionID(provider, principalID, channelID, threadID string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = ProviderWebhookV1
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		principalID = "local"
	}
	channelID = strings.TrimSpace(channelID)
	threadID = strings.TrimSpace(threadID)

	sum := sha256.Sum256([]byte(provider + "|" + principalID + "|" + channelID + "|" + threadID))
	// Keep it directory-name safe and stable.
	return "relay_" + hex.EncodeToString(sum[:16])
}

func InboundIdempotencyKey(provider, principalID, channelID, threadID, messageID string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = ProviderWebhookV1
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		principalID = "local"
	}
	channelID = strings.TrimSpace(channelID)
	threadID = strings.TrimSpace(threadID)
	messageID = strings.TrimSpace(messageID)
	return provider + "|" + principalID + "|" + channelID + "|" + threadID + "|" + messageID
}

func MarkInboundProcessed(dataDir string, idempotencyKey string) (idempotent bool, err error) {
	dataDir = strings.TrimSpace(dataDir)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if dataDir == "" {
		return false, errors.New("dataDir is required")
	}
	if idempotencyKey == "" {
		return false, errors.New("idempotencyKey is required")
	}

	seenDir := filepath.Join(dataDir, "channel_relay", "seen")
	if err := os.MkdirAll(seenDir, 0o700); err != nil {
		return false, err
	}
	sum := sha256.Sum256([]byte(idempotencyKey))
	path := filepath.Join(seenDir, hex.EncodeToString(sum[:]))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return true, nil
		}
		return false, err
	}
	_ = f.Close()
	return false, nil
}

func AppendTrace(traceDir string, entry TraceEntry) error {
	traceDir = strings.TrimSpace(traceDir)
	if traceDir == "" {
		return errors.New("traceDir is required")
	}
	if strings.TrimSpace(entry.TS) == "" {
		entry.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}

	path := filepath.Join(traceDir, TraceFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}
