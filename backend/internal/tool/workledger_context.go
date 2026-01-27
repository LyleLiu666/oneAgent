package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type contextKeyWorkLedger struct{}

func ContextWithWorkLedger(ctx context.Context, ledger *workledger.Store) context.Context {
	return context.WithValue(ctx, contextKeyWorkLedger{}, ledger)
}

func WorkLedgerFromContext(ctx context.Context) *workledger.Store {
	if v, ok := ctx.Value(contextKeyWorkLedger{}).(*workledger.Store); ok {
		return v
	}
	return nil
}

