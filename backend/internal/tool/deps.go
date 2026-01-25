package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

const (
	contextKeySettingsDB contextKey = "settingsDB"
)

func ContextWithSettingsDB(ctx context.Context, db *settingsdb.DB) context.Context {
	return context.WithValue(ctx, contextKeySettingsDB, db)
}

func SettingsDBFromContext(ctx context.Context) *settingsdb.DB {
	if v, ok := ctx.Value(contextKeySettingsDB).(*settingsdb.DB); ok {
		return v
	}
	return nil
}
