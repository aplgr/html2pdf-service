package tokens

import "testing"

func TestCache_ReadyValidateRateLimit(t *testing.T) {
	c := NewCache()
	if c.Ready() {
		t.Fatalf("expected cache not ready initially")
	}
	if c.Validate("abc") {
		t.Fatalf("expected validate false when not ready")
	}
	if got := c.RateLimit("abc"); got != 0 {
		t.Fatalf("expected rate limit 0 when not ready, got %d", got)
	}

	c.Replace(map[string]Entry{
		"abc": {
			RateLimit: 10,
			Scope:     Scope{"api": true, "ops": false},
		},
	})
	if !c.Ready() {
		t.Fatalf("expected cache ready after Replace")
	}
	if !c.Validate("abc") {
		t.Fatalf("expected validate true for existing token")
	}
	if c.Validate("missing") {
		t.Fatalf("expected validate false for missing token")
	}
	if got := c.RateLimit("abc"); got != 10 {
		t.Fatalf("expected rate limit 10, got %d", got)
	}
	if got := c.RateLimit("missing"); got != 0 {
		t.Fatalf("expected rate limit 0 for missing token, got %d", got)
	}
	if c.HasScope("abc", "ops") {
		t.Fatalf("expected missing ops scope to be false")
	}
	if !c.HasScope("abc", "api") {
		t.Fatalf("expected api scope to be true")
	}
}
