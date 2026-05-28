package kios

import (
	"testing"
	"time"
)

func TestServiceAuthHeader(t *testing.T) {
	got := serviceAuthHeader("test-secret-32-chars-minimum-abc", time.Unix(1700000000, 0))
	want := "1700000000.2921ba316deb8d4c36b55729661097c6f8b3af2e2c02f3c85e3c2051153d922c"
	if got != want {
		t.Fatalf("serviceAuthHeader = %q, want %q", got, want)
	}
}
