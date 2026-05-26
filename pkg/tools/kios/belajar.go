package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// LearnPattern maps an input phrase to a learned intent/target.
type LearnPattern struct {
	Intent string `json:"intent"`
	Target string `json:"target"`
	Count  int    `json:"count"`
}

// Habits captures usage patterns.
type Habits struct {
	PeakHours   map[string]int `json:"peak_hours"`
	TopProducts map[string]int `json:"top_products"`
	ReportTimes []string       `json:"report_times"`
}

// --- learning store methods ---

func (s *Store) SavePattern(ctx context.Context, input, intent, target string) error {
	key := strings.ToLower(strings.TrimSpace(input))
	p := LearnPattern{Intent: intent, Target: target}
	if v, err := s.rdb.HGet(ctx, keyLearnPat, key).Result(); err == nil {
		_ = json.Unmarshal([]byte(v), &p)
		if intent != "" {
			p.Intent = intent
		}
		if target != "" {
			p.Target = target
		}
	}
	p.Count++
	b, _ := json.Marshal(p)
	return s.rdb.HSet(ctx, keyLearnPat, key, string(b)).Err()
}

func (s *Store) GetPattern(ctx context.Context, input string) *LearnPattern {
	v, err := s.rdb.HGet(ctx, keyLearnPat, strings.ToLower(strings.TrimSpace(input))).Result()
	if err != nil {
		return nil
	}
	var p LearnPattern
	if json.Unmarshal([]byte(v), &p) != nil {
		return nil
	}
	return &p
}

func (s *Store) SaveAlias(ctx context.Context, alias, target string) error {
	return s.rdb.HSet(ctx, keyLearnAls, strings.ToLower(strings.TrimSpace(alias)), target).Err()
}

func (s *Store) ResolveAlias(ctx context.Context, alias string) string {
	v, _ := s.rdb.HGet(ctx, keyLearnAls, strings.ToLower(strings.TrimSpace(alias))).Result()
	return v
}

func (s *Store) SaveShortcut(ctx context.Context, name string, items []string) error {
	b, _ := json.Marshal(items)
	return s.rdb.HSet(ctx, keyLearnShc, strings.ToLower(strings.TrimSpace(name)), string(b)).Err()
}

func (s *Store) GetShortcut(ctx context.Context, name string) []string {
	v, err := s.rdb.HGet(ctx, keyLearnShc, strings.ToLower(strings.TrimSpace(name))).Result()
	if err != nil {
		return nil
	}
	var items []string
	_ = json.Unmarshal([]byte(v), &items)
	return items
}

func (s *Store) AllShortcuts(ctx context.Context) map[string][]string {
	m, _ := s.rdb.HGetAll(ctx, keyLearnShc).Result()
	out := make(map[string][]string, len(m))
	for k, v := range m {
		var items []string
		if json.Unmarshal([]byte(v), &items) == nil {
			out[k] = items
		}
	}
	return out
}

func (s *Store) TrackHabit(ctx context.Context, tipe, value string) error {
	h := s.GetHabits(ctx)
	jam := NowWITA().Format("15")
	switch tipe {
	case "sale":
		h.PeakHours[jam]++
		if value != "" {
			h.TopProducts[value]++
		}
	case "report_request":
		found := false
		for _, t := range h.ReportTimes {
			if t == jam {
				found = true
			}
		}
		if !found {
			h.ReportTimes = append(h.ReportTimes, jam)
		}
	}
	b, _ := json.Marshal(h)
	return s.rdb.Set(ctx, keyLearnHab, string(b), 0).Err()
}

func (s *Store) GetHabits(ctx context.Context) *Habits {
	h := &Habits{PeakHours: map[string]int{}, TopProducts: map[string]int{}}
	if v, err := s.rdb.Get(ctx, keyLearnHab).Result(); err == nil {
		_ = json.Unmarshal([]byte(v), h)
		if h.PeakHours == nil {
			h.PeakHours = map[string]int{}
		}
		if h.TopProducts == nil {
			h.TopProducts = map[string]int{}
		}
	}
	return h
}

func (s *Store) AddUnknown(ctx context.Context, cmd string) error {
	return s.rdb.HIncrBy(ctx, keyLearnUnk, strings.TrimSpace(cmd), 1).Err()
}

func (s *Store) ListUnknowns(ctx context.Context) map[string]int {
	m, _ := s.rdb.HGetAll(ctx, keyLearnUnk).Result()
	out := make(map[string]int, len(m))
	for k, v := range m {
		var n int
		fmt.Sscanf(v, "%d", &n)
		out[k] = n
	}
	return out
}

func (s *Store) ResolveUnknown(ctx context.Context, cmd string) error {
	return s.rdb.HDel(ctx, keyLearnUnk, strings.TrimSpace(cmd)).Err()
}

// --- tool ---

// BelajarTool persists learned aliases, shortcuts, habits, patterns, and
// unrecognized commands (ports learning-engine + self-learner).
type BelajarTool struct{ store *Store }

func (t *BelajarTool) Name() string { return "kios_belajar" }

func (t *BelajarTool) Description() string {
	return "Memori belajar kios: alias produk, shortcut paket barang, catat kebiasaan (jam ramai/" +
		"produk laris), pola perintah, dan antrian perintah tak dikenal. Berguna untuk personalisasi."
}

func (t *BelajarTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"alias_set", "alias_get", "shortcut_set", "shortcut_get", "shortcut_list",
					"habit_track", "habit", "pattern_save", "pattern_get", "unknown_add", "unknown_list", "unknown_resolve"},
				"description": "Aksi belajar.",
			},
			"alias":  map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"nama":   map[string]any{"type": "string", "description": "nama shortcut"},
			"items":  map[string]any{"type": "string", "description": "isi shortcut, pisah koma"},
			"tipe":   map[string]any{"type": "string", "enum": []string{"sale", "report_request"}},
			"value":  map[string]any{"type": "string"},
			"input":  map[string]any{"type": "string"},
			"intent": map[string]any{"type": "string"},
			"cmd":    map[string]any{"type": "string"},
		},
		"required": []string{"action"},
	}
}

func (t *BelajarTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	if _, _, refusal := resolveRole(ctx, t.store); refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "alias_set":
		if argStr(args, "alias") == "" || argStr(args, "target") == "" {
			return tools.ErrorResult("alias dan target-nya diisi dulu ya kak 🙏")
		}
		t.store.SaveAlias(ctx, argStr(args, "alias"), argStr(args, "target"))
		return tools.NewToolResult(fmt.Sprintf("Alias '%s' → '%s' disimpan.", argStr(args, "alias"), argStr(args, "target")))
	case "alias_get":
		v := t.store.ResolveAlias(ctx, argStr(args, "alias"))
		if v == "" {
			return tools.NewToolResult("Alias nggak ketemu kak 🔍")
		}
		return tools.NewToolResult(fmt.Sprintf("'%s' → '%s'", argStr(args, "alias"), v))
	case "shortcut_set":
		nama := argStr(args, "nama")
		items := splitItems(argStr(args, "items"))
		if nama == "" || len(items) == 0 {
			return tools.ErrorResult("nama dan items (pisah koma)-nya diisi dulu ya kak 🙏")
		}
		t.store.SaveShortcut(ctx, nama, items)
		return tools.NewToolResult(fmt.Sprintf("Shortcut '%s' = %s.", nama, strings.Join(items, ", ")))
	case "shortcut_get":
		items := t.store.GetShortcut(ctx, argStr(args, "nama"))
		if len(items) == 0 {
			return tools.NewToolResult("Shortcut nggak ketemu kak 🔍")
		}
		return tools.NewToolResult(fmt.Sprintf("'%s' = %s", argStr(args, "nama"), strings.Join(items, ", ")))
	case "shortcut_list":
		all := t.store.AllShortcuts(ctx)
		if len(all) == 0 {
			return tools.NewToolResult("Belum ada shortcut.")
		}
		var b strings.Builder
		b.WriteString("Shortcut:\n")
		for k, v := range all {
			fmt.Fprintf(&b, "- %s: %s\n", k, strings.Join(v, ", "))
		}
		return tools.NewToolResult(b.String())
	case "habit_track":
		t.store.TrackHabit(ctx, argStr(args, "tipe"), argStr(args, "value"))
		return tools.SilentResult("habit dicatat")
	case "habit":
		return t.habitSummary(ctx)
	case "pattern_save":
		if argStr(args, "input") == "" {
			return tools.ErrorResult("input-nya diisi dulu ya kak 🙏")
		}
		t.store.SavePattern(ctx, argStr(args, "input"), argStr(args, "intent"), argStr(args, "target"))
		return tools.NewToolResult("Pola disimpan.")
	case "pattern_get":
		p := t.store.GetPattern(ctx, argStr(args, "input"))
		if p == nil {
			return tools.NewToolResult("Pola nggak ketemu kak 🔍")
		}
		return tools.NewToolResult(fmt.Sprintf("intent=%s target=%s (x%d)", p.Intent, p.Target, p.Count))
	case "unknown_add":
		t.store.AddUnknown(ctx, argStr(args, "cmd"))
		return tools.SilentResult("perintah tak dikenal dicatat")
	case "unknown_list":
		return t.unknownList(ctx)
	case "unknown_resolve":
		t.store.ResolveUnknown(ctx, argStr(args, "cmd"))
		return tools.NewToolResult("Perintah tak dikenal ditandai selesai.")
	default:
		return tools.ErrorResult("Hmm, aksi belajar belum dikenal kak 🤔")
	}
}

func splitItems(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func topN(m map[string]int, n int) string {
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(m))
	for k, v := range m {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	var parts []string
	for i := 0; i < len(arr) && i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s (%d)", arr[i].k, arr[i].v))
	}
	return strings.Join(parts, ", ")
}

func (t *BelajarTool) habitSummary(ctx context.Context) *tools.ToolResult {
	h := t.store.GetHabits(ctx)
	if len(h.PeakHours) == 0 && len(h.TopProducts) == 0 {
		return tools.NewToolResult("Belum ada data kebiasaan.")
	}
	var b strings.Builder
	b.WriteString("🧠 Kebiasaan kios:\n")
	if s := topN(h.PeakHours, 3); s != "" {
		fmt.Fprintf(&b, "Jam ramai: %s\n", s)
	}
	if s := topN(h.TopProducts, 5); s != "" {
		fmt.Fprintf(&b, "Produk laris: %s\n", s)
	}
	if len(h.ReportTimes) > 0 {
		fmt.Fprintf(&b, "Biasa minta laporan jam: %s\n", strings.Join(h.ReportTimes, ", "))
	}
	return tools.NewToolResult(b.String())
}

func (t *BelajarTool) unknownList(ctx context.Context) *tools.ToolResult {
	m := t.store.ListUnknowns(ctx)
	if len(m) == 0 {
		return tools.NewToolResult("Tidak ada perintah tak dikenal.")
	}
	return tools.NewToolResult("Perintah tak dikenal (paling sering): " + topN(m, 10))
}
