package cache

import (
	"sync"
)

type StorageClassCache struct {
	cache   map[string]bool
	fsTypes map[string]string
	mutex   sync.RWMutex
}

func NewStorageClassCache() *StorageClassCache {
	return &StorageClassCache{
		cache:   make(map[string]bool),
		fsTypes: make(map[string]string),
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

func (c *StorageClassCache) GetFsType(name string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	fsType, exists := c.fsTypes[name]
	return fsType, exists
}

func (c *StorageClassCache) SetFsType(name string, fsType string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.fsTypes[name] = fsType
}

func (c *StorageClassCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string]bool)
	c.fsTypes = make(map[string]string)
}
