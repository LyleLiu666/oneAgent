package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

const contextKeySkillManager contextKey = "skillManager"

func ContextWithSkillManager(ctx context.Context, manager *skill.Manager) context.Context {
	return context.WithValue(ctx, contextKeySkillManager, manager)
}

func SkillManagerFromContext(ctx context.Context) *skill.Manager {
	if v, ok := ctx.Value(contextKeySkillManager).(*skill.Manager); ok {
		return v
	}
	return nil
}

