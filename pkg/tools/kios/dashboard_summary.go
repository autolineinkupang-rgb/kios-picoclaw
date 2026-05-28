package kios

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// serviceAuthHeader builds the "<ts>.<hexsig>" verification token the dashboard
// expects in the X-Kios-Service header. The bot generates a fresh one per
// request: sig = HMAC_SHA256(secret, "kios-service:" + ts).
func serviceAuthHeader(secret string, now time.Time) string {
	ts := now.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "kios-service:%d", ts)
	return fmt.Sprintf("%d.%s", ts, hex.EncodeToString(mac.Sum(nil)))
}
