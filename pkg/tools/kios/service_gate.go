package kios

import (
	"os"
	"strconv"
	"strings"
)

// IsEkonomiMode returns true jika KIOS_EKONOMI_MODE=true (exact match, lowercase).
// Nilai lain (mis. "1", "yes") dianggap false agar perubahan eksplisit.
func IsEkonomiMode() bool {
	return strings.TrimSpace(os.Getenv("KIOS_EKONOMI_MODE")) == "true"
}

// IsJamAktif memeriksa apakah jam WITA saat ini masuk dalam window KIOS_JAM_AKTIF.
// Format: "HH-HH" (mis. "07-22" atau overnight "22-06").
// Jika env kosong atau format tidak valid → selalu true.
func IsJamAktif() bool {
	return isJamAktifAt(NowWITA().Hour())
}

// isJamAktifAt adalah versi testable dari IsJamAktif yang menerima jam eksplisit.
func isJamAktifAt(hour int) bool {
	val := strings.TrimSpace(os.Getenv("KIOS_JAM_AKTIF"))
	if val == "" {
		return true
	}
	parts := strings.SplitN(val, "-", 2)
	if len(parts) != 2 {
		return true
	}
	from, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	to, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || from < 0 || from > 23 || to < 0 || to > 23 {
		return true
	}
	if from <= to {
		return hour >= from && hour < to
	}
	// Overnight range: mis. 22-06 → aktif jam 22,23,0,1,...,5
	return hour >= from || hour < to
}

// IsServiceAktif returns false jika ekonomi mode aktif atau di luar jam aktif.
// Dipakai oleh EnsureDailyReportJob dan EnsureDailyBackupJob.
func IsServiceAktif() bool {
	if IsEkonomiMode() {
		return false
	}
	return IsJamAktif()
}
