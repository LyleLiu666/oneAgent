package skill

import (
	"context"
	"sync"
	"time"
)

type Manager struct {
	mu sync.Mutex

	ttl time.Duration
	byWorkspace map[string]cachedCatalog
}

type cachedCatalog struct {
	loadedAt time.Time
	catalog  *Catalog
	err      error
}

func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Manager{
		ttl:        ttl,
		byWorkspace: make(map[string]cachedCatalog),
	}
}

func (m *Manager) Load(ctx context.Context, workspaceRoot string) (*Catalog, error) {
	if m == nil {
		return Discover(ctx, DiscoverOptions{WorkspaceRoot: workspaceRoot})
	}

	now := time.Now()

	m.mu.Lock()
	cached, ok := m.byWorkspace[workspaceRoot]
	if ok && cached.catalog != nil && now.Sub(cached.loadedAt) < m.ttl {
		cat := cached.catalog
		m.mu.Unlock()
		return cat, nil
	}
	m.mu.Unlock()

	cat, err := Discover(ctx, DiscoverOptions{WorkspaceRoot: workspaceRoot})

	m.mu.Lock()
	m.byWorkspace[workspaceRoot] = cachedCatalog{
		loadedAt: now,
		catalog:  cat,
		err:      err,
	}
	m.mu.Unlock()

	return cat, err
}

