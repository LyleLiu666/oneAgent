package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrToolNotFound = errors.New("tool not found")

type ToolCall struct {
	ID     string
	Name   string
	Fields map[string]string
	Raw    string
}

type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (any, error)
}

type ToolFunc func(ctx context.Context, call ToolCall) (any, error)

type ToolRegistry struct {
	mu sync.RWMutex

	tools map[string]ToolFunc

	// OnMissing is called when no tool is registered for call.Name.
	// When nil, Execute returns ErrToolNotFound.
	OnMissing ToolFunc
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolFunc),
	}
}

func (r *ToolRegistry) Register(name string, fn ToolFunc) error {
	if r == nil {
		return errors.New("nil ToolRegistry")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty tool name")
	}
	if fn == nil {
		return errors.New("nil tool func")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = make(map[string]ToolFunc)
	}
	if _, ok := r.tools[name]; ok {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = fn
	return nil
}

func (r *ToolRegistry) Execute(ctx context.Context, call ToolCall) (any, error) {
	if r == nil {
		return nil, errors.New("nil ToolRegistry")
	}
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return nil, errors.New("empty tool name")
	}

	r.mu.RLock()
	fn := r.tools[name]
	missing := r.OnMissing
	r.mu.RUnlock()

	if fn == nil {
		if missing != nil {
			return missing(ctx, call)
		}
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return fn(ctx, call)
}
