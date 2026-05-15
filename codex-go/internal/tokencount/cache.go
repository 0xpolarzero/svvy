// Package tokencount provides token counting functionality with caching.
package tokencount

import (
	"container/list"
	"sync"
)

// LRUCache is a thread-safe LRU cache for token counts.
// It provides O(1) lookups and evictions with configurable size limits.
type LRUCache struct {
	maxSize int
	mu      sync.RWMutex
	cache   map[string]*list.Element
	lru     *list.List
}

// cacheEntry stores a key-value pair in the LRU list
type cacheEntry struct {
	key   string
	value int
}

// NewLRUCache creates a new LRU cache with the specified maximum size.
// When the cache exceeds maxSize, the least recently used item is evicted.
// A maxSize of 0 or negative creates an unbounded cache (not recommended for production).
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize: maxSize,
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
	}
}

// Get retrieves a value from the cache and marks it as recently used.
// Returns the value and true if found, or 0 and false if not found.
func (c *LRUCache) Get(key string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		// Move to front (most recently used)
		c.lru.MoveToFront(elem)
		return elem.Value.(*cacheEntry).value, true
	}
	return 0, false
}

// Put adds or updates a value in the cache.
// If the cache is full, the least recently used item is evicted.
func (c *LRUCache) Put(key string, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.cache[key]; ok {
		// Update existing entry and move to front
		c.lru.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = value
		return
	}

	// Add new entry
	entry := &cacheEntry{key: key, value: value}
	elem := c.lru.PushFront(entry)
	c.cache[key] = elem

	// Evict least recently used if over capacity
	if c.maxSize > 0 && c.lru.Len() > c.maxSize {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used item from the cache.
// Must be called with lock held.
func (c *LRUCache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.lru.Remove(elem)
		entry := elem.Value.(*cacheEntry)
		delete(c.cache, entry.key)
	}
}

// Len returns the current number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Clear removes all items from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*list.Element)
	c.lru = list.New()
}

// Stats returns cache statistics.
type CacheStats struct {
	Size    int
	MaxSize int
	Hits    int64
	Misses  int64
}

// CachedCounter wraps a TokenCounter with an LRU cache for performance.
type CachedCounter struct {
	counter TokenCounter
	cache   *LRUCache
	hits    int64
	misses  int64
	mu      sync.RWMutex
}

// NewCachedCounter creates a counter with LRU caching.
// The cacheSize parameter determines how many unique strings to cache.
// Recommended values:
//   - 1000-5000 for typical usage
//   - 10000+ for large conversation applications
func NewCachedCounter(counter TokenCounter, cacheSize int) *CachedCounter {
	return &CachedCounter{
		counter: counter,
		cache:   NewLRUCache(cacheSize),
	}
}

// CountTokens counts tokens with caching.
func (c *CachedCounter) CountTokens(text string) int {
	// Try cache first (read lock)
	if count, ok := c.cache.Get(text); ok {
		c.mu.Lock()
		c.hits++
		c.mu.Unlock()
		return count
	}

	// Cache miss - count tokens
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()

	count := c.counter.CountTokens(text)
	c.cache.Put(text, count)
	return count
}

// GetStats returns cache performance statistics.
func (c *CachedCounter) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    c.cache.Len(),
		MaxSize: c.cache.maxSize,
		Hits:    c.hits,
		Misses:  c.misses,
	}
}

// ClearCache clears the cache and resets statistics.
func (c *CachedCounter) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Clear()
	c.hits = 0
	c.misses = 0
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (c *CachedCounter) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total) * 100
}

// BatchCounter enables efficient batch token counting with caching.
type BatchCounter struct {
	counter TokenCounter
	cache   *LRUCache
	mu      sync.RWMutex
}

// NewBatchCounter creates a counter optimized for batch operations.
func NewBatchCounter(counter TokenCounter, cacheSize int) *BatchCounter {
	return &BatchCounter{
		counter: counter,
		cache:   NewLRUCache(cacheSize),
	}
}

// CountTokensBatch counts tokens for multiple texts efficiently.
// Uses caching and can process texts in parallel for large batches.
func (b *BatchCounter) CountTokensBatch(texts []string) []int {
	results := make([]int, len(texts))

	// For small batches, process serially
	if len(texts) < 100 {
		for i, text := range texts {
			if count, ok := b.cache.Get(text); ok {
				results[i] = count
			} else {
				count := b.counter.CountTokens(text)
				b.cache.Put(text, count)
				results[i] = count
			}
		}
		return results
	}

	// For large batches, use parallel processing
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4) // Limit to 4 concurrent goroutines

	for i, text := range texts {
		wg.Add(1)
		go func(idx int, txt string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if count, ok := b.cache.Get(txt); ok {
				results[idx] = count
			} else {
				count := b.counter.CountTokens(txt)
				b.cache.Put(txt, count)
				results[idx] = count
			}
		}(i, text)
	}

	wg.Wait()
	return results
}

// CountTokensTotal counts total tokens across multiple texts.
func (b *BatchCounter) CountTokensTotal(texts []string) int {
	counts := b.CountTokensBatch(texts)
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

// IncrementalCounter tracks token deltas for efficient updates.
type IncrementalCounter struct {
	counter TokenCounter
	cache   *LRUCache
}

// NewIncrementalCounter creates a counter for incremental updates.
func NewIncrementalCounter(counter TokenCounter, cacheSize int) *IncrementalCounter {
	return &IncrementalCounter{
		counter: counter,
		cache:   NewLRUCache(cacheSize),
	}
}

// CountDelta calculates the token difference when text changes.
// This is more efficient than recounting all tokens.
func (i *IncrementalCounter) CountDelta(oldText, newText string) int {
	var oldCount, newCount int

	// Get old count from cache or compute
	if count, ok := i.cache.Get(oldText); ok {
		oldCount = count
	} else {
		oldCount = i.counter.CountTokens(oldText)
		i.cache.Put(oldText, oldCount)
	}

	// Get new count from cache or compute
	if count, ok := i.cache.Get(newText); ok {
		newCount = count
	} else {
		newCount = i.counter.CountTokens(newText)
		i.cache.Put(newText, newCount)
	}

	return newCount - oldCount
}

// UpdateTotal updates a running total when content changes.
func (i *IncrementalCounter) UpdateTotal(currentTotal int, oldText, newText string) int {
	delta := i.CountDelta(oldText, newText)
	return currentTotal + delta
}
