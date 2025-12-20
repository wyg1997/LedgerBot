package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Cache interface for caching system
type Cache interface {
	// Get retrieves a value from cache
	Get(key string, value interface{}) error

	// Set sets a value in cache with TTL
	Set(key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from cache
	Delete(key string) error

	// Exists checks if a key exists
	Exists(key string) bool

	// Clear clears all cache
	Clear() error
}

// userMappingCache implements Cache for user mappings
type userMappingCache struct {
	items map[string]*cacheItem
	mu    sync.RWMutex
	file  string
}

type cacheItem struct {
	Value     interface{}   `json:"value"`
	ExpiredAt time.Time     `json:"expired_at"`
}

// NewUserMappingCache creates a new user mapping cache with file persistence
func NewUserMappingCache(file string) Cache {
	cache := &userMappingCache{
		items: make(map[string]*cacheItem),
		file:  file,
	}

	// Try to load from file
	if err := cache.load(); err != nil {
		fmt.Printf("Failed to load cache from file: %v\n", err)
	}

	// Start cleanup routine
	go cache.cleanup()

	return cache
}

// Get retrieves a value from cache
func (c *userMappingCache) Get(key string, value interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	// Check if expired
	if time.Now().After(item.ExpiredAt) {
		delete(c.items, key)
		c.save() // Save to file
		return fmt.Errorf("key expired: %s", key)
	}

	// Marshal and unmarshal to copy the value
	data, err := json.Marshal(item.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	return json.Unmarshal(data, value)
}

// Set sets a value in cache with TTL
func (c *userMappingCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create cache item
	item := &cacheItem{
		Value:     value,
		ExpiredAt: time.Now().Add(ttl),
	}

	// Store in map
	c.items[key] = item

	// Save to file
	return c.save()
}

// Delete removes a value from cache
func (c *userMappingCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return c.save()
}

// Exists checks if a key exists
func (c *userMappingCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	// Check if not expired
	return time.Now().Before(item.ExpiredAt)
}

// Clear clears all cache
func (c *userMappingCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	return c.save()
}

// load loads cache from file
func (c *userMappingCache) load() error {
	if c.file == "" {
		return nil
	}

	data, err := os.ReadFile(c.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, which is OK
		}
		return fmt.Errorf("failed to read cache file: %v", err)
	}

	if len(data) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return json.Unmarshal(data, &c.items)
}

// save saves cache to file
func (c *userMappingCache) save() error {
	if c.file == "" {
		return nil
	}

	// Create directory if needed
	if err := os.MkdirAll(getDir(c.file), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	data, err := json.MarshalIndent(c.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %v", err)
	}

	return os.WriteFile(c.file, data, 0644)
}

// cleanup runs periodically to remove expired items
func (c *userMappingCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C

		c.mu.Lock()
		changed := false
		now := time.Now()

		for key, item := range c.items {
			if now.After(item.ExpiredAt) {
				delete(c.items, key)
				changed = true
			}
		}

		if changed {
			c.save()
		}
		c.mu.Unlock()
	}
}

// getDir extracts directory from file path
func getDir(path string) string {
	if idx := len(path) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if path[i] == '/' || path[i] == '\\' {
				return path[:i]
			}
		}
	}
	return "."
}