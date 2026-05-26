package kios

import (
	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// NewStokTool builds the stock-management tool.
func NewStokTool(store *Store) *StokTool { return &StokTool{store: store} }

// NewKasirTool builds the cashier tool.
func NewKasirTool(store *Store) *KasirTool { return &KasirTool{store: store} }

// NewLaporanTool builds the reporting tool.
func NewLaporanTool(store *Store) *LaporanTool { return &LaporanTool{store: store} }

// NewHargaTool builds the pricing tool.
func NewHargaTool(store *Store) *HargaTool { return &HargaTool{store: store} }

// AllTools returns the full set of kios tools backed by the given store,
// ready to register in picoclaw's tool registry.
func AllTools(store *Store) []tools.Tool {
	return []tools.Tool{
		NewStokTool(store),
		NewKasirTool(store),
		NewLaporanTool(store),
		NewHargaTool(store),
	}
}
