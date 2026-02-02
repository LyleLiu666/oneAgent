package handler

import (
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/model"
)

func TestStreamBroadcaster_RecordStreamMsg_FinalClearsActiveForToolCall(t *testing.T) {
	sb := &StreamBroadcaster{}
	streamID := "stream-1"

	sb.recordStreamMsg(streamMsg{
		Op:      "start",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeText,
	})
	sb.recordStreamMsg(streamMsg{
		Op:      "delta",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeText,
		Delta:   "hello",
	})

	// Tool loops often "final" with msg_type=tool_call after starting as assistant text.
	sb.recordStreamMsg(streamMsg{
		Op:      "final",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeToolCall,
	})

	if got := sb.snapshotActiveLocked(); len(got) != 0 {
		t.Fatalf("expected no active snapshot entries, got %v", got)
	}
}

func TestStreamBroadcaster_RecordStreamMsg_FinalClearsActiveForText(t *testing.T) {
	sb := &StreamBroadcaster{}
	streamID := "stream-2"

	sb.recordStreamMsg(streamMsg{
		Op:      "start",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeText,
	})
	sb.recordStreamMsg(streamMsg{
		Op:      "delta",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeText,
		Delta:   "world",
	})
	sb.recordStreamMsg(streamMsg{
		Op:      "final",
		ID:      streamID,
		Role:    model.MessageRoleAssistant,
		MsgType: model.MessageTypeText,
	})

	if got := sb.snapshotActiveLocked(); len(got) != 0 {
		t.Fatalf("expected no active snapshot entries, got %v", got)
	}
}

