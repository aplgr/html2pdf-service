package ratelimit

import "testing"

func TestNewStore_AlwaysReturnsStorage(t *testing.T) {
	if s := NewStore(RedisConfig{}); s == nil {
		t.Fatalf("expected non-nil memory store when redis addr empty")
	}

	if s := NewStore(RedisConfig{Addr: "127.0.0.1:1", DB: 0}); s == nil {
		t.Fatalf("expected non-nil store even with redis config")
	}
}
