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

// NewUserTool builds the user/RBAC management tool.
func NewUserTool(store *Store) *UserTool { return &UserTool{store: store} }

// NewSupplierTool builds the supplier tool.
func NewSupplierTool(store *Store) *SupplierTool { return &SupplierTool{store: store} }

// NewPromoTool builds the promo/discount tool.
func NewPromoTool(store *Store) *PromoTool { return &PromoTool{store: store} }

// NewPustakaTool builds the knowledge-base tool.
func NewPustakaTool(store *Store) *PustakaTool { return &PustakaTool{store: store} }

// NewPasarTool builds the market-intelligence tool.
func NewPasarTool(store *Store) *PasarTool { return &PasarTool{store: store} }

// NewBelajarTool builds the learning-memory tool.
func NewBelajarTool(store *Store) *BelajarTool { return &BelajarTool{store: store} }

// NewUploadTool builds the chat-file (Excel/CSV) import tool.
func NewUploadTool(store *Store) *UploadTool { return &UploadTool{store: store} }

// NewRestoreTool builds the backup-JSON restore tool.
func NewRestoreTool(store *Store) *RestoreTool { return &RestoreTool{store: store} }

// AllTools returns the full set of kios tools backed by the given store,
// ready to register in picoclaw's tool registry. Every tool is wrapped with
// withSchemaCompat so strict providers (e.g. Groq) accept loosely-typed model
// output without a HTTP 400 "tool call validation failed" — see schema_compat.go.
func AllTools(store *Store) []tools.Tool {
	raw := []tools.Tool{
		NewStokTool(store),
		NewKasirTool(store),
		NewLaporanTool(store),
		NewHargaTool(store),
		NewUserTool(store),
		NewSupplierTool(store),
		NewPromoTool(store),
		NewPustakaTool(store),
		NewPasarTool(store),
		NewBelajarTool(store),
		NewUploadTool(store),
		NewRestoreTool(store),
		NewBonTool(store),
	}
	out := make([]tools.Tool, len(raw))
	for i, t := range raw {
		out[i] = withSchemaCompat(t)
	}
	return out
}
