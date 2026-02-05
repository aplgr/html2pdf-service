package chrome

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pdf-renderer/internal/config"
)

func testConfig(poolSize int) config.Config {
	var cfg config.Config
	cfg.PDF.ChromePoolSize = poolSize
	cfg.PDF.UserDataDir = filepath.Join(os.TempDir(), "html2pdf-chrome-tests")
	cfg.PDF.TimeoutSecs = 1
	return cfg
}

func TestCreateProfileDir_DefaultAndCustomBase(t *testing.T) {
	cfg := testConfig(1)
	cfg.PDF.UserDataDir = ""
	dir1, err := createProfileDir(cfg)
	if err != nil {
		t.Fatalf("createProfileDir default base failed: %v", err)
	}
	defer os.RemoveAll(dir1)
	if _, err := os.Stat(dir1); err != nil {
		t.Fatalf("expected created dir to exist: %v", err)
	}

	customBase := t.TempDir()
	cfg.PDF.UserDataDir = customBase
	dir2, err := createProfileDir(cfg)
	if err != nil {
		t.Fatalf("createProfileDir custom base failed: %v", err)
	}
	defer os.RemoveAll(dir2)
	if filepath.Dir(dir2) != customBase {
		t.Fatalf("expected profile dir under custom base %q, got %q", customBase, dir2)
	}
}

func TestPoolAcquireReleaseAndClose(t *testing.T) {
	p := &Pool{sem: make(chan struct{}, 1), browserCtx: context.Background()}
	p.sem <- struct{}{}

	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("expected acquire success, got %v", err)
	}
	if tab == nil {
		t.Fatalf("expected non-nil tab")
	}
	if len(p.sem) != 0 {
		t.Fatalf("expected token consumed after acquire")
	}

	p.Release(tab, nil)
	if len(p.sem) != 1 {
		t.Fatalf("expected token returned after release")
	}

	p.closed = true
	if _, err := p.Acquire(context.Background()); err == nil {
		t.Fatalf("expected acquire to fail when pool is closed")
	}
}

func TestPoolAcquireContextCanceled(t *testing.T) {
	p := &Pool{sem: make(chan struct{}, 1), browserCtx: context.Background()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestPoolAcquireTimesOutWhenNoCapacity(t *testing.T) {
	p := &Pool{sem: make(chan struct{}, 1), browserCtx: context.Background()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := p.Acquire(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected acquire deadline exceeded, got %v", err)
	}
}

func TestPoolStatsAndClose(t *testing.T) {
	p := &Pool{sem: make(chan struct{}, 2), cfg: testConfig(2), profileDir: t.TempDir(), browserCtx: context.Background()}
	p.sem <- struct{}{}
	p.sem <- struct{}{}

	st := p.Stats(1)
	if !st.Enabled || st.Capacity != 2 || st.Idle != 2 || st.InUse != 0 {
		t.Fatalf("unexpected stats before acquire: %+v", st)
	}

	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	st = p.Stats(1)
	if st.InUse != 1 {
		t.Fatalf("expected one in use, got %+v", st)
	}
	p.Release(tab, nil)

	p.Close()
	p.Close() // idempotent
	st = p.Stats(1)
	if st.Enabled {
		t.Fatalf("expected stats disabled after close: %+v", st)
	}
}

func TestPoolRestartClosed(t *testing.T) {
	p := &Pool{closed: true}
	if err := p.Restart(); err == nil {
		t.Fatalf("expected restart error when closed")
	}
}

func TestCreateProfileDir_InvalidBase(t *testing.T) {
	var cfg config.Config
	cfg.PDF.UserDataDir = "/dev/null/x"
	if _, err := createProfileDir(cfg); err == nil {
		t.Fatalf("expected error for invalid base dir")
	}
}

func TestNewPool_Disabled(t *testing.T) {
	_, err := NewPool(testConfig(0))
	if err == nil {
		t.Fatalf("expected disabled pool error")
	}
}

func TestPoolRestart_Success(t *testing.T) {
	cfg := testConfig(1)
	old := t.TempDir()
	p := &Pool{
		cfg:        cfg,
		sem:        make(chan struct{}, 1),
		profileDir: old,
	}
	p.sem <- struct{}{}

	if err := p.Restart(); err != nil {
		t.Fatalf("expected restart success, got %v", err)
	}
	if p.profileDir == "" || p.profileDir == old {
		t.Fatalf("expected new profile dir, got %q", p.profileDir)
	}
	if p.Stats(1).Restarts < 1 {
		t.Fatalf("expected restart counter increment")
	}
	p.Close()
}

func TestNewPool_WithDummyExecPath(t *testing.T) {
	cfg := testConfig(1)
	cfg.PDF.ChromePath = "/bin/true"

	p, err := NewPool(cfg)
	if err != nil {
		t.Fatalf("expected pool init success with dummy exec path, got %v", err)
	}
	if p == nil {
		t.Fatalf("expected non-nil pool")
	}
	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire should work: %v", err)
	}
	p.Release(tab, nil)
	p.Close()
}

func TestIsSessionInterrupted(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "target closed", err: errors.New("target closed"), want: true},
		{name: "normal error", err: errors.New("validation failed"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSessionInterrupted(tc.err); got != tc.want {
				t.Fatalf("IsSessionInterrupted(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
