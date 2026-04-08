package agro

import (
	"strings"
	"sync"
	"time"
)

type animalLookupCache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	items map[string]animalLookupCacheEntry
}

type animalLookupCacheEntry struct {
	exists    bool
	expiresAt time.Time
}

func newAnimalLookupCache(ttl time.Duration) *animalLookupCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &animalLookupCache{
		ttl:   ttl,
		items: make(map[string]animalLookupCacheEntry),
	}
}

func (c *animalLookupCache) Get(farmID, animalCode string) (bool, bool) {
	if c == nil {
		return false, false
	}
	key := c.key(farmID, animalCode)
	if key == "" {
		return false, false
	}

	c.mu.RLock()
	entry, found := c.items[key]
	c.mu.RUnlock()
	if !found {
		return false, false
	}
	if time.Now().UTC().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return false, false
	}
	return entry.exists, true
}

func (c *animalLookupCache) Set(farmID, animalCode string, exists bool) {
	if c == nil {
		return
	}
	key := c.key(farmID, animalCode)
	if key == "" {
		return
	}

	c.mu.Lock()
	c.items[key] = animalLookupCacheEntry{
		exists:    exists,
		expiresAt: time.Now().UTC().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *animalLookupCache) key(farmID, animalCode string) string {
	farmID = strings.TrimSpace(farmID)
	animalCode = strings.TrimSpace(strings.ToUpper(animalCode))
	if farmID == "" || animalCode == "" {
		return ""
	}
	return farmID + ":" + animalCode
}
