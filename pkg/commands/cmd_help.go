package commands

import (
	"context"
	"fmt"
	"strings"
)

func helpCommand() Definition {
	return Definition{
		Name:        "help",
		Description: "Daftar semua perintah yang tersedia",
		Usage:       "/help",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			var defs []Definition
			if rt != nil && rt.ListDefinitions != nil {
				defs = rt.ListDefinitions()
			} else {
				defs = BuiltinDefinitions()
			}
			return req.Reply(formatHelpMessage(defs))
		},
	}
}

func formatHelpMessage(defs []Definition) string {
	if len(defs) == 0 {
		return "Belum ada perintah tersedia."
	}

	lines := make([]string, 0, len(defs)+2)
	lines = append(lines, "Perintah yang tersedia:")
	for _, def := range defs {
		usage := def.EffectiveUsage()
		if usage == "" {
			usage = "/" + def.Name
		}
		desc := def.Description
		if desc == "" {
			desc = "tanpa deskripsi"
		}
		lines = append(lines, fmt.Sprintf("%s — %s", usage, desc))
	}
	lines = append(lines, "\nKetik /bantuan untuk panduan lengkap kak 🙏")
	return strings.Join(lines, "\n")
}
