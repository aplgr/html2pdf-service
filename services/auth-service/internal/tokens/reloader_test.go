package tokens

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	m   map[string]int
	err error
}

func (r fakeRepo) LoadTokens(ctx context.Context) (map[string]int, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]int, len(r.m))
	for k, v := range r.m {
		out[k] = v
	}
	return out, nil
}

func TestReloader_LoadOnce_Success(t *testing.T) {
	c := NewCache()
	r := NewReloader(fakeRepo{m: map[string]int{"k": 3}}, c, time.Hour)

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
	c.Replace(map[string]int{"keep": 7})

	expectedErr := errors.New("boom")
	r := NewReloader(fakeRepo{err: expectedErr}, c, time.Hour)

	if err := r.LoadOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}

	if got := c.RateLimit("keep"); got != 7 {
		t.Fatalf("expected cache unchanged, got %d", got)
	}
}
