package kios

import (
	"net/url"
	"regexp"
	"strings"
)

// trustedDomains are well-known safe Indonesian sources (instant +40).
var trustedDomains = map[string]bool{
	"panelharga.badanpangan.go.id": true, "bi.go.id": true, "ews.kemendag.go.id": true,
	"ntt.bps.go.id": true, "bps.go.id": true, "kemendag.go.id": true, "badanpangan.go.id": true,
	"setda.nttprov.go.id": true, "nttprov.go.id": true, "rotendaokab.go.id": true,
	"kupang.antaranews.com": true, "antaranews.com": true, "kompas.com": true, "tempo.co": true,
	"detik.com": true, "bisnis.com": true, "cnnindonesia.com": true, "okezone.com": true,
	"tribunnews.com": true, "jpnn.com": true, "korantimor.com": true, "onlinentt.com": true,
	"tokopedia.com": true, "bukalapak.com": true, "shopee.co.id": true,
	"facebook.com": true, "instagram.com": true,
}

var (
	reShortener = regexp.MustCompile(
		`(?i)^(bit\.ly|tinyurl\.com|goo\.gl|ow\.ly|t\.co|tiny\.cc|rb\.gy|cutt\.ly|shorturl\.at)$`,
	)
	reIPHost     = regexp.MustCompile(`(\d{1,3}\.){3}\d{1,3}`)
	reCyrillic   = regexp.MustCompile(`[\x{0400}-\x{04FF}]`)
	reSuspPath   = regexp.MustCompile(`(?i)(phish|malware|hack|exploit|payload|shell|cmd=|exec=|eval\()`)
	reManyHyphen = regexp.MustCompile(`(?i)(-{3,}|([a-z0-9]+-){5,})`)
)

// URLSafety is the result of a static URL safety assessment.
type URLSafety struct {
	Skor   int    `json:"skor"`
	Aman   bool   `json:"aman"`
	Alasan string `json:"alasan"`
}

// SkorAman computes a 0–100 safety score for a URL without making any network
// request (deterministic, no SSRF risk). >= 60 is considered safe.
func SkorAman(raw string) URLSafety {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return URLSafety{0, false, "URL kosong"}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return URLSafety{0, false, "URL tidak valid"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return URLSafety{0, false, "skema bukan http/https"}
	}

	host := strings.ToLower(u.Hostname())
	pathQuery := u.EscapedPath() + "?" + u.RawQuery
	skor := 50
	var notes []string

	if u.Scheme == "https" {
		skor += 10
		notes = append(notes, "HTTPS +10")
	} else {
		skor -= 20
		notes = append(notes, "non-HTTPS -20")
	}

	base := strings.TrimPrefix(host, "www.")
	if trustedDomains[base] || trustedDomains[host] {
		skor += 40
		notes = append(notes, "domain terpercaya +40")
	}

	switch {
	case strings.HasSuffix(host, ".go.id"):
		skor += 20
		notes = append(notes, ".go.id +20")
	case strings.HasSuffix(host, ".id"):
		skor += 10
		notes = append(notes, ".id +10")
	}

	if reShortener.MatchString(base) {
		skor -= 40
		notes = append(notes, "URL shortener -40")
	}
	if reIPHost.MatchString(host) || reCyrillic.MatchString(host) || strings.Contains(host, "xn--") || len(host) > 60 {
		skor -= 50
		notes = append(notes, "host mencurigakan -50")
	}
	if reSuspPath.MatchString(pathQuery) {
		skor -= 60
		notes = append(notes, "path mencurigakan -60")
	}
	if reManyHyphen.MatchString(host) {
		skor -= 20
		notes = append(notes, "banyak hyphen -20")
	}
	if len(host) > 50 {
		skor -= 15
		notes = append(notes, "domain terlalu panjang -15")
	}

	if skor < 0 {
		skor = 0
	}
	if skor > 100 {
		skor = 100
	}
	alasan := strings.Join(notes, ", ")
	if alasan == "" {
		alasan = "standar"
	}
	return URLSafety{skor, skor >= 60, alasan}
}
