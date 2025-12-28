package app

import (
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"
    "time"

    "github.com/gofiber/fiber/v2"
    u "html2pdf/internal/utils"
)

type memStore struct {
    sync.RWMutex
    m map[string][]byte
}

func newMemStore() *memStore {
    return &memStore{m: make(map[string][]byte)}
}

func (s *memStore) Get(key string) ([]byte, error) {
    s.RLock()
    defer s.RUnlock()
    val, ok := s.m[key]
    if !ok {
        return nil, nil
    }
    return val, nil
}

func (s *memStore) Set(key string, val []byte, exp time.Duration) error {
    s.Lock()
    s.m[key] = val
    s.Unlock()
    return nil
}

func (s *memStore) Delete(key string) error {
    s.Lock()
    delete(s.m, key)
    s.Unlock()
    return nil
}

func (s *memStore) Reset() error {
    s.Lock()
    s.m = make(map[string][]byte)
    s.Unlock()
    return nil
}

func (s *memStore) Close() error { return nil }

func TestUserRateLimitMiddleware(t *testing.T) {
    cfg := u.Config{}
    cfg.RateLimiter.EnableUserLimiter = true
    cfg.RateLimiter.UserLimit = 2
    cfg.RateLimiter.Interval = time.Hour

    rateLimitStore = newMemStore()

    app := fiber.New()
    app.Use(userRateLimitMiddleware(cfg))
    app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

    makeReq := func() *http.Request {
        req := httptest.NewRequest("GET", "/", nil)
        req.Header.Set("User-Agent", "test-agent")
        req.RemoteAddr = "1.2.3.4:5678"
        return req
    }

    for i := 0; i < 2; i++ {
        resp, err := app.Test(makeReq(), -1)
        if err != nil {
            t.Fatalf("request %d failed: %v", i+1, err)
        }
        if resp.StatusCode != fiber.StatusOK {
            t.Fatalf("expected 200 but got %d", resp.StatusCode)
        }
    }

    resp, err := app.Test(makeReq(), -1)
    if err != nil {
        t.Fatalf("third request failed: %v", err)
    }
    if resp.StatusCode != fiber.StatusTooManyRequests {
        t.Fatalf("expected 429 but got %d", resp.StatusCode)
    }
}

