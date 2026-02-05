package tokens

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

type sequenceRepo struct {
	mu      sync.Mutex
	results []struct {
		m   map[string]Entry
		err error
	}
	idx int

	calls atomic.Int32
}

func (r *sequenceRepo) LoadTokens(ctx context.Context) (map[string]Entry, error) {
	r.calls.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.results) == 0 {
		return nil, nil
	}
	if r.idx >= len(r.results) {
		last := r.results[len(r.results)-1]
		return last.m, last.err
	}
	cur := r.results[r.idx]
	r.idx++
	return cur.m, cur.err
}

func TestReloader_Start_RefreshesTokens(t *testing.T) {
	c := NewCache()
	repo := &sequenceRepo{results: []struct {
		m   map[string]Entry
		err error
	}{
		{m: map[string]Entry{"k": {RateLimit: 1, Scope: Scope{"api": true}}}},
		{m: map[string]Entry{"k": {RateLimit: 5, Scope: Scope{"api": true}}}},
	}}

	r := NewReloader(repo, c, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got := c.RateLimit("k"); got == 5 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected reloader to refresh token rate limit to 5, got %d", c.RateLimit("k"))
}

func TestReloader_Start_DBDownKeepsExistingCache(t *testing.T) {
	c := NewCache()
	c.Replace(map[string]Entry{
		"keep": {RateLimit: 9, Scope: Scope{"api": true}},
	})
	repo := &sequenceRepo{results: []struct {
		m   map[string]Entry
		err error
	}{
		{err: errors.New("db unavailable")},
	}}

	r := NewReloader(repo, c, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()

	if got := c.RateLimit("keep"); got != 9 {
		t.Fatalf("expected existing cache to remain intact during DB outage, got %d", got)
	}
	if repo.calls.Load() == 0 {
		t.Fatalf("expected repo to be called at least once")
	}
}
