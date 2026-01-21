package handler

import (
	"sync"
	"testing"
	"time"
)

func TestStreamManager_GetOrCreate(t *testing.T) {
	sm := NewStreamManager()
	sessionID := "session-1"

	sb1 := sm.GetOrCreate(sessionID)
	if sb1 == nil {
		t.Fatal("Expected StreamBroadcaster, got nil")
	}

	sb2 := sm.GetOrCreate(sessionID)
	if sb1 != sb2 {
		t.Fatal("Expected same StreamBroadcaster instance")
	}
}

func TestStreamBroadcaster_Broadcast(t *testing.T) {
	sm := NewStreamManager()
	sb := sm.GetOrCreate("session-1")

	// Client 1
	ch1 := sb.Subscribe()
	// Client 2
	ch2 := sb.Subscribe()

	var wg sync.WaitGroup
	wg.Add(2)

	received1 := ""
	received2 := ""

	go func() {
		defer wg.Done()
		for event := range ch1 {
			received1 = event.Data
		}
	}()

	go func() {
		defer wg.Done()
		for event := range ch2 {
			received2 = event.Data
		}
	}()

	// Broadcast event
	testData := "test-message"
	sb.Broadcast(StreamEvent{Type: "content", Data: testData})

	// Allow time for channel receive
	time.Sleep(100 * time.Millisecond)
	sb.Finish() // This closes channels

	wg.Wait()

	if received1 != testData {
		t.Errorf("Client 1 expected %s, got %s", testData, received1)
	}
	if received2 != testData {
		t.Errorf("Client 2 expected %s, got %s", testData, received2)
	}
}

func TestStreamBroadcaster_StartGeneration(t *testing.T) {
	sm := NewStreamManager()
	sb := sm.GetOrCreate("session-1")

	if !sb.StartGeneration() {
		t.Error("First call to StartGeneration should return true")
	}

	if sb.StartGeneration() {
		t.Error("Second call to StartGeneration should return false")
	}
}
