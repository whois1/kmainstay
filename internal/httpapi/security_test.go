package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestLoginLimiter_NormalisesEmailAndBoundsStorage(t *testing.T) {
	limiter := newLoginLimiter(5, time.Minute, 8)
	now := time.Unix(1000, 0)
	for i := 0; i < 5; i++ {
		if !limiter.allow(" User@Example.COM ", now) {
			t.Fatalf("attempt %d unexpectedly denied", i+1)
		}
	}
	if limiter.allow("user@example.com", now) {
		t.Fatal("normalised sixth attempt was allowed")
	}
	for i := 0; i < 20; i++ {
		limiter.allow(fmt.Sprintf("user-%d@example.com", i), now)
	}
	if got := limiter.size(); got > 8 {
		t.Fatalf("limiter entries = %d, want at most 8", got)
	}
	if !limiter.allow("fresh@example.com", now.Add(2*time.Minute)) {
		t.Fatal("fresh attempt denied after expiry")
	}
	if got := limiter.size(); got != 1 {
		t.Fatalf("expired entries not cleaned up: %d remain", got)
	}
}
