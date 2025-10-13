package proxy

import (
	"sync"
	"testing"
	"time"
)

func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache()

	if cache == nil {
		t.Fatal("NewMemoryCache() returned nil")
	}

	if cache.entries == nil {
		t.Error("NewMemoryCache() entries map is nil")
	}

	if len(cache.entries) != 0 {
		t.Error("NewMemoryCache() should start with empty entries")
	}
}

func TestMemoryCache_Set_Get(t *testing.T) {
	cache := NewMemoryCache()

	tests := []struct {
		name  string
		key   string
		value any
		ttl   time.Duration
	}{
		{
			name:  "string value",
			key:   "test-key",
			value: "test-value",
			ttl:   1 * time.Hour,
		},
		{
			name:  "int value",
			key:   "int-key",
			value: 42,
			ttl:   1 * time.Hour,
		},
		{
			name:  "struct value",
			key:   "struct-key",
			value: &VersionInfo{Version: "v1.0.0", Time: time.Now()},
			ttl:   1 * time.Hour,
		},
		{
			name:  "slice value",
			key:   "slice-key",
			value: []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			ttl:   1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.Set(tt.key, tt.value, tt.ttl)

			value, exists := cache.Get(tt.key)

			if !exists {
				t.Errorf("Get(%q) exists = false, want true", tt.key)
			}

			if _, isSlice := tt.value.([]string); isSlice {
				gotSlice, ok := value.([]string)
				if !ok {
					t.Errorf("Get(%q) type = %T, want []string", tt.key, value)
				} else if len(gotSlice) != len(tt.value.([]string)) {
					t.Errorf("Get(%q) slice length = %d, want %d", tt.key, len(gotSlice), len(tt.value.([]string)))
				}
			} else if value != tt.value {
				t.Errorf("Get(%q) = %v, want %v", tt.key, value, tt.value)
			}
		})
	}
}

func TestMemoryCache_Get_NonExistent(t *testing.T) {
	cache := NewMemoryCache()

	value, exists := cache.Get("nonexistent-key")

	if exists {
		t.Error("Get() for nonexistent key should return exists = false")
	}

	if value != nil {
		t.Errorf("Get() for nonexistent key should return nil, got %v", value)
	}
}

func TestMemoryCache_Get_Expired(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("expired-key", "expired-value", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	value, exists := cache.Get("expired-key")

	if exists {
		t.Error("Get() for expired key should return exists = false")
	}

	if value != nil {
		t.Errorf("Get() for expired key should return nil, got %v", value)
	}
}

func TestMemoryCache_Set_Overwrite(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("key", "value1", 1*time.Hour)

	cache.Set("key", "value2", 1*time.Hour)

	value, exists := cache.Get("key")

	if !exists {
		t.Error("Get() should return exists = true after overwrite")
	}

	if value != "value2" {
		t.Errorf("Get() = %v, want %v (overwritten value)", value, "value2")
	}
}

func TestMemoryCache_Set_DifferentTTL(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("short-ttl", "value1", 50*time.Millisecond)

	cache.Set("long-ttl", "value2", 1*time.Hour)

	time.Sleep(100 * time.Millisecond)

	if _, exists := cache.Get("short-ttl"); exists {
		t.Error("Short TTL value should be expired")
	}

	if _, exists := cache.Get("long-ttl"); !exists {
		t.Error("Long TTL value should still exist")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("key1", "value1", 1*time.Hour)
	cache.Set("key2", "value2", 1*time.Hour)
	cache.Set("key3", "value3", 1*time.Hour)

	if _, exists := cache.Get("key1"); !exists {
		t.Error("key1 should exist before clear")
	}

	cache.Clear()

	if _, exists := cache.Get("key1"); exists {
		t.Error("key1 should not exist after clear")
	}

	if _, exists := cache.Get("key2"); exists {
		t.Error("key2 should not exist after clear")
	}

	if _, exists := cache.Get("key3"); exists {
		t.Error("key3 should not exist after clear")
	}

	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()

	if count != 0 {
		t.Errorf("Clear() should empty entries map, got %d entries", count)
	}
}

func TestMemoryCache_Clear_EmptyCache(t *testing.T) {
	cache := NewMemoryCache()

	cache.Clear()

	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()

	if count != 0 {
		t.Error("Empty cache should remain empty after Clear()")
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("expire1", "value1", 50*time.Millisecond)
	cache.Set("expire2", "value2", 50*time.Millisecond)

	cache.Set("keep", "value3", 1*time.Hour)

	time.Sleep(100 * time.Millisecond)

	cache.mu.Lock()
	now := time.Now()
	for key, entry := range cache.entries {
		if now.After(entry.expiration) {
			delete(cache.entries, key)
		}
	}
	cache.mu.Unlock()

	if _, exists := cache.Get("expire1"); exists {
		t.Error("expire1 should be cleaned up")
	}

	if _, exists := cache.Get("expire2"); exists {
		t.Error("expire2 should be cleaned up")
	}

	if _, exists := cache.Get("keep"); !exists {
		t.Error("keep should not be cleaned up")
	}
}

func TestMemoryCache_Concurrency(t *testing.T) {
	cache := NewMemoryCache()
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numOperations {
				key := string(rune('a' + (id % 26)))
				cache.Set(key, id*numOperations+j, 1*time.Hour)
			}
		}(i)
	}

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				key := string(rune('a' + (id % 26)))
				cache.Get(key)
			}
		}(i)
	}

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			cache.Clear()
		}()
	}

	wg.Wait()
}

func TestMemoryCache_TypeAssertion(t *testing.T) {
	cache := NewMemoryCache()

	expectedInfo := &VersionInfo{
		Version: "v1.2.3",
		Time:    time.Now(),
	}
	cache.Set("version-info", expectedInfo, 1*time.Hour)

	value, exists := cache.Get("version-info")
	if !exists {
		t.Fatal("Get() should return exists = true")
	}

	info, ok := value.(*VersionInfo)
	if !ok {
		t.Fatalf("Type assertion failed: got type %T, want *VersionInfo", value)
	}

	if info.Version != expectedInfo.Version {
		t.Errorf("Version = %q, want %q", info.Version, expectedInfo.Version)
	}

	expectedVersions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	cache.Set("versions", expectedVersions, 1*time.Hour)

	value, exists = cache.Get("versions")
	if !exists {
		t.Fatal("Get() should return exists = true for versions")
	}

	versions, ok := value.([]string)
	if !ok {
		t.Fatalf("Type assertion failed: got type %T, want []string", value)
	}

	if len(versions) != len(expectedVersions) {
		t.Errorf("len(versions) = %d, want %d", len(versions), len(expectedVersions))
	}
}

func TestMemoryCache_Interface(t *testing.T) {
	var _ Cache = (*MemoryCache)(nil)
	var _ Cache = NewMemoryCache()
}

func TestMemoryCache_ZeroTTL(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("zero-ttl", "value", 0)

	time.Sleep(1 * time.Millisecond)

	value, exists := cache.Get("zero-ttl")
	if exists {
		t.Error("Zero TTL value should be expired")
	}

	if value != nil {
		t.Errorf("Zero TTL value should return nil, got %v", value)
	}
}

func TestMemoryCache_NegativeTTL(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("negative-ttl", "value", -1*time.Hour)

	value, exists := cache.Get("negative-ttl")
	if exists {
		t.Error("Negative TTL value should be expired")
	}

	if value != nil {
		t.Errorf("Negative TTL value should return nil, got %v", value)
	}
}

func TestMemoryCache_UpdateTTL(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("key", "value1", 50*time.Millisecond)

	time.Sleep(30 * time.Millisecond)

	cache.Set("key", "value2", 1*time.Hour)

	time.Sleep(30 * time.Millisecond)

	value, exists := cache.Get("key")
	if !exists {
		t.Error("Updated TTL value should still exist")
	}

	if value != "value2" {
		t.Errorf("Get() = %v, want %v", value, "value2")
	}
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache()

	b.ResetTimer()
	for b.Loop() {
		cache.Set("benchmark-key", "benchmark-value", 1*time.Hour)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache()
	cache.Set("benchmark-key", "benchmark-value", 1*time.Hour)

	b.ResetTimer()
	for b.Loop() {
		cache.Get("benchmark-key")
	}
}

func BenchmarkMemoryCache_Set_Parallel(b *testing.B) {
	cache := NewMemoryCache()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set("key", i, 1*time.Hour)
			i++
		}
	})
}

func BenchmarkMemoryCache_Get_Parallel(b *testing.B) {
	cache := NewMemoryCache()
	cache.Set("benchmark-key", "benchmark-value", 1*time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get("benchmark-key")
		}
	})
}

func BenchmarkMemoryCache_Mixed_Parallel(b *testing.B) {
	cache := NewMemoryCache()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				cache.Set("key", i, 1*time.Hour)
			} else {
				cache.Get("key")
			}
			i++
		}
	})
}
