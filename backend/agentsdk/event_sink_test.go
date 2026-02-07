package agentsdk

import (
	"context"
	"testing"
)

func TestEventSinkFunc_IsEventSink(t *testing.T) {
	called := false
	var got Event

	var sink EventSink = EventSinkFunc(func(ctx context.Context, ev Event) {
		_ = ctx
		called = true
		got = ev
	})

	sink.OnEvent(context.Background(), Event{Kind: EventKindTrace, Protocol: "xml", Step: 1})

	if !called {
		t.Fatalf("expected sink to be called")
	}
	if got.Kind != EventKindTrace || got.Protocol != "xml" || got.Step != 1 {
		t.Fatalf("unexpected event: %#v", got)
	}
}
