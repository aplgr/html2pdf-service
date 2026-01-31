package tokens

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	m   map[string]Entry
	err error
}

func (r fakeRepo) LoadTokens(ctx context.Context) (map[string]Entry, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]Entry, len(r.m))
	for k, v := range r.m {
		out[k] = v
	}
	return out, nil
}

func TestReloader_LoadOnce_Success(t *testing.T) {
	c := NewCache()
	r := NewReloader(fakeRepo{m: map[string]Entry{
		"k": {
			RateLimit: 3,
			Scope:     Scope{"api": true},
		},
	}}, c, time.Hour)

	if err := r.LoadOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.Ready() {
		t.Fatalf("expected cache ready after successful LoadOnce")
	}
	if got := c.RateLimit("k"); got != 3 {
		t.Fatalf("expected rate limit 3, got %d", got)
	}
}

func TestReloader_LoadOnce_Error_DoesNotReplace(t *testing.T) {
	c := NewCache()
	c.Replace(map[string]Entry{
		"keep": {
			RateLimit: 7,
			Scope:     Scope{"api": true},
		},
	})

	expectedErr := errors.New("boom")
	r := NewReloader(fakeRepo{err: expectedErr}, c, time.Hour)

	if err := r.LoadOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}

	if got := c.RateLimit("keep"); got != 7 {
		t.Fatalf("expected cache unchanged, got %d", got)
	}
}
