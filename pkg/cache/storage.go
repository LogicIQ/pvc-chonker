package cache

import (
	"sync"
)

type StorageClassCache struct {
	cache map[string]bool
	mutex sync.RWMutex
}

func NewStorageClassCache() *StorageClassCache {
	return &StorageClassCache{
		cache: make(map[string]bool),
	}
}

func (c *StorageClassCache) Get(name string) (bool, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	expandable, exists := c.cache[name]
	return expandable, exists
}

func (c *StorageClassCache) Set(name string, expandable bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[name] = expandable
}

func (c *StorageClassCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for k := range c.cache {
		delete(c.cache, k)
	}
}
