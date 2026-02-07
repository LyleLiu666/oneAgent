package agentsdk

import (
	"context"
	"errors"
	"testing"
)

func TestToolRegistry_RegisterAndExecute(t *testing.T) {
	reg := NewToolRegistry()

	if err := reg.Register("echo", func(ctx context.Context, call ToolCall) (any, error) {
		_ = ctx
		if call.Name != "echo" {
			t.Fatalf("unexpected tool name: %q", call.Name)
		}
		if call.Fields["msg"] != "hi" {
			t.Fatalf("unexpected fields: %#v", call.Fields)
		}
		return map[string]any{"echo": call.Fields["msg"]}, nil
	}); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	out, err := reg.Execute(context.Background(), ToolCall{
		ID:     "t_1",
		Name:   "echo",
		Fields: map[string]string{"msg": "hi"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", out)
	}
	if m["echo"] != "hi" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestToolRegistry_DuplicateRegisterFails(t *testing.T) {
	reg := NewToolRegistry()
	if err := reg.Register("echo", func(context.Context, ToolCall) (any, error) { return nil, nil }); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if err := reg.Register("echo", func(context.Context, ToolCall) (any, error) { return nil, nil }); err == nil {
		t.Fatalf("expected duplicate register error")
	}
}

func TestToolRegistry_UnknownToolReturnsErrToolNotFound(t *testing.T) {
	reg := NewToolRegistry()
	_, err := reg.Execute(context.Background(), ToolCall{Name: "nope"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected ErrToolNotFound, got %v", err)
	}
}

func TestToolRegistry_OnMissingOverridesNotFound(t *testing.T) {
	reg := NewToolRegistry()
	reg.OnMissing = func(ctx context.Context, call ToolCall) (any, error) {
		_ = ctx
		return map[string]any{"missing": call.Name}, nil
	}

	out, err := reg.Execute(context.Background(), ToolCall{Name: "nope"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", out)
	}
	if m["missing"] != "nope" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

