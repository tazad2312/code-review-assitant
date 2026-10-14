package review

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type cacheEntry struct {
	key      string
	comments []Comment
}

type LRUCache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

func NewLRUCache(capacity int) *LRUCache {
	if capacity < 1 {
		capacity = 1
	}
	return &LRUCache{capacity: capacity, items: make(map[string]*list.Element), order: list.New()}
}

func DiffHash(diff string) string {
	sum := sha256.Sum256([]byte(diff))
	return hex.EncodeToString(sum[:])
}

func (c *LRUCache) Get(key string) ([]Comment, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(element)
	comments := element.Value.(*cacheEntry).comments
	return append([]Comment(nil), comments...), true
}

func (c *LRUCache) Add(key string, comments []Comment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		element.Value.(*cacheEntry).comments = append([]Comment(nil), comments...)
		c.order.MoveToFront(element)
		return
	}
	element := c.order.PushFront(&cacheEntry{key: key, comments: append([]Comment(nil), comments...)})
	c.items[key] = element
	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		delete(c.items, oldest.Value.(*cacheEntry).key)
		c.order.Remove(oldest)
	}
}
