package kios

import (
	"os"
	"testing"
)

func TestIsJamAktifKosong(t *testing.T) {
	os.Unsetenv("KIOS_JAM_AKTIF")
	for hour := 0; hour < 24; hour++ {
		if !isJamAktifAt(hour) {
			t.Errorf("isJamAktifAt(%d) = false, want true (env kosong)", hour)
		}
	}
}

func TestIsJamAktifNormal(t *testing.T) {
	os.Setenv("KIOS_JAM_AKTIF", "07-22")
	defer os.Unsetenv("KIOS_JAM_AKTIF")

	aktif := []int{7, 10, 15, 21}
	for _, h := range aktif {
		if !isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = false, want true (07-22)", h)
		}
	}
	nonAktif := []int{0, 3, 6, 22, 23}
	for _, h := range nonAktif {
		if isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = true, want false (07-22)", h)
		}
	}
}

func TestIsJamAktifOvernight(t *testing.T) {
	os.Setenv("KIOS_JAM_AKTIF", "22-06")
	defer os.Unsetenv("KIOS_JAM_AKTIF")

	aktif := []int{22, 23, 0, 3, 5}
	for _, h := range aktif {
		if !isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = false, want true (22-06)", h)
		}
	}
	nonAktif := []int{6, 10, 15, 21}
	for _, h := range nonAktif {
		if isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = true, want false (22-06)", h)
		}
	}
}

func TestIsEkonomiMode(t *testing.T) {
	os.Unsetenv("KIOS_EKONOMI_MODE")
	if IsEkonomiMode() {
		t.Error("IsEkonomiMode() = true, want false (env kosong)")
	}

	os.Setenv("KIOS_EKONOMI_MODE", "true")
	defer os.Unsetenv("KIOS_EKONOMI_MODE")
	if !IsEkonomiMode() {
		t.Error("IsEkonomiMode() = false, want true")
	}

	os.Setenv("KIOS_EKONOMI_MODE", "1")
	if IsEkonomiMode() {
		t.Error("IsEkonomiMode() = true for '1', want false (hanya 'true' yang valid)")
	}
}

func TestIsServiceAktif(t *testing.T) {
	os.Unsetenv("KIOS_EKONOMI_MODE")
	os.Unsetenv("KIOS_JAM_AKTIF")
	if !IsServiceAktif() {
		t.Error("IsServiceAktif() = false tanpa env var, want true")
	}

	os.Setenv("KIOS_EKONOMI_MODE", "true")
	defer os.Unsetenv("KIOS_EKONOMI_MODE")
	if IsServiceAktif() {
		t.Error("IsServiceAktif() = true saat KIOS_EKONOMI_MODE=true, want false")
	}
}
