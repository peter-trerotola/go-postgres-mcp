package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/petros/go-postgres-mcp/internal/config"
)

// PoolManager manages connection pools for multiple databases.
type PoolManager struct {
	mu    sync.RWMutex
	pools map[string]*pgxpool.Pool
}

func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]*pgxpool.Pool),
	}
}

// Connect creates a connection pool for a database config.
// The connection string includes default_transaction_read_only=on (Tier 2).
func (pm *PoolManager) Connect(ctx context.Context, dbCfg config.DatabaseConfig) error {
	connStr, err := dbCfg.ConnString()
	if err != nil {
		return fmt.Errorf("building connection string for %q: %w", dbCfg.Name, err)
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parsing pool config for %q: %w", dbCfg.Name, err)
	}

	poolCfg.MaxConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("creating pool for %q: %w", dbCfg.Name, err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("pinging %q: %w", dbCfg.Name, err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if old, ok := pm.pools[dbCfg.Name]; ok {
		old.Close()
	}
	pm.pools[dbCfg.Name] = pool
	return nil
}

// Get returns a pool for the named database.
func (pm *PoolManager) Get(name string) (*pgxpool.Pool, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pool, ok := pm.pools[name]
	if !ok {
		return nil, fmt.Errorf("no connection pool for database %q", name)
	}
	return pool, nil
}

// Close shuts down all connection pools.
func (pm *PoolManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, pool := range pm.pools {
		pool.Close()
	}
	pm.pools = make(map[string]*pgxpool.Pool)
}
