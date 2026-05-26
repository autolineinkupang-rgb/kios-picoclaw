package commands

import "sync"

// Extra command definitions contributed by other packages (e.g. kios tools)
// so channels can advertise them in their command menu without importing those
// packages. Only Name/Description are used for platform menu registration.
var (
	extraMu   sync.RWMutex
	extraDefs []Definition
)

// SetExtraDefinitions replaces the set of extra (non-builtin) command
// definitions advertised in channel command menus. Safe to call repeatedly.
func SetExtraDefinitions(defs []Definition) {
	extraMu.Lock()
	defer extraMu.Unlock()
	extraDefs = append([]Definition(nil), defs...)
}

// MenuDefinitions returns builtin definitions plus any registered extras.
// Channels use this to populate their platform command menu.
func MenuDefinitions() []Definition {
	extraMu.RLock()
	defer extraMu.RUnlock()
	out := BuiltinDefinitions()
	return append(out, extraDefs...)
}
