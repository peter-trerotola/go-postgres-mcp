package postgres

import (
	"testing"
)

func TestNewPoolManager(t *testing.T) {
	pm := NewPoolManager()
	if pm == nil {
		t.Fatal("expected non-nil pool manager")
	}
}

func TestPoolManager_GetUnknown(t *testing.T) {
	pm := NewPoolManager()
	_, err := pm.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown database")
	}
}

func TestPoolManager_CloseEmpty(t *testing.T) {
	pm := NewPoolManager()
	pm.Close() // Should not panic
}

func TestPoolManager_CloseIdempotent(t *testing.T) {
	pm := NewPoolManager()
	pm.Close()
	pm.Close() // Should not panic on double close
}
