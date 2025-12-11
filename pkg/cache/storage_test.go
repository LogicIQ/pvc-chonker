package cache

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStorageClassCache(t *testing.T) {
	cache := NewStorageClassCache()

	// Test empty cache
	if _, exists := cache.Get("nonexistent"); exists {
		t.Error("Expected nonexistent storage class to not exist in cache")
	}

	// Test setting and getting expandable storage class
	cache.Set("expandable-sc", true)
	if expandable, exists := cache.Get("expandable-sc"); !exists || !expandable {
		t.Error("Expected expandable storage class to be cached as expandable")
	}

	// Test setting and getting non-expandable storage class
	cache.Set("non-expandable-sc", false)
	if expandable, exists := cache.Get("non-expandable-sc"); !exists || expandable {
		t.Error("Expected non-expandable storage class to be cached as non-expandable")
	}

	// Test clearing cache
	cache.Clear()
	if _, exists := cache.Get("expandable-sc"); exists {
		t.Error("Expected cache to be empty after clear")
	}
	if _, exists := cache.Get("non-expandable-sc"); exists {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestStorageClassCacheConcurrency(t *testing.T) {
	cache := NewStorageClassCache()

	// Test concurrent access
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("test-sc", i%2 == 0)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("test-sc")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify cache still works
	cache.Set("final-test", true)
	if expandable, exists := cache.Get("final-test"); !exists || !expandable {
		t.Error("Expected cache to work correctly after concurrent access")
	}
}

func helperStorageClass(name string, allowExpansion *bool) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		AllowVolumeExpansion: allowExpansion,
	}
}

func TestStorageClassCacheWithRealObjects(t *testing.T) {
	cache := NewStorageClassCache()

	// Test with expandable storage class
	expandable := true
	sc1 := helperStorageClass("gp3", &expandable)
	cache.Set(sc1.Name, sc1.AllowVolumeExpansion != nil && *sc1.AllowVolumeExpansion)

	if result, exists := cache.Get("gp3"); !exists || !result {
		t.Error("Expected gp3 storage class to be expandable")
	}

	// Test with non-expandable storage class
	nonExpandable := false
	sc2 := helperStorageClass("local", &nonExpandable)
	cache.Set(sc2.Name, sc2.AllowVolumeExpansion != nil && *sc2.AllowVolumeExpansion)

	if result, exists := cache.Get("local"); !exists || result {
		t.Error("Expected local storage class to be non-expandable")
	}

	// Test with nil AllowVolumeExpansion (defaults to false)
	sc3 := helperStorageClass("default", nil)
	cache.Set(sc3.Name, sc3.AllowVolumeExpansion != nil && *sc3.AllowVolumeExpansion)

	if result, exists := cache.Get("default"); !exists || result {
		t.Error("Expected default storage class to be non-expandable when AllowVolumeExpansion is nil")
	}
}
