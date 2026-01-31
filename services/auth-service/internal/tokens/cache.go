package tokens

import "sync"

type Scope map[string]bool

type Entry struct {
	RateLimit int
	Scope     Scope
}

// Cache keeps token -> entry in memory for fast lookup.
type Cache struct {
	mu sync.RWMutex
	m  map[string]Entry
}

func NewCache() *Cache {
	return &Cache{}
}

func (c *Cache) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m != nil
}

func (c *Cache) Validate(token string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.m == nil {
		return false
	}
	_, ok := c.m[token]
	return ok
}

func (c *Cache) RateLimit(token string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.m == nil {
		return 0
	}
	return c.m[token].RateLimit
}

func (c *Cache) HasScope(token, scope string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.m == nil {
		return false
	}
	entry, ok := c.m[token]
	if !ok {
		return false
	}
	return entry.Scope[scope]
}

func (c *Cache) Replace(all map[string]Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = all
}
