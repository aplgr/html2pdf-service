package tokens

import "sync"

// Cache keeps token -> rateLimit in memory for fast lookup.
type Cache struct {
	mu sync.RWMutex
	m  map[string]int
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
	return c.m[token]
}

func (c *Cache) Replace(all map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = all
}
