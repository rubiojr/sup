package bot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rubiojr/sup/cache"
)

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}
	if cache == nil {
		t.Fatal("NewCache() returned nil")
	}

	// Verify cache has default expiry set
	if cache.expiry == nil {
		t.Fatal("Cache expiry is nil")
	}
	if *cache.expiry != 1*time.Hour {
		t.Errorf("Expected default expiry of 1h, got %v", *cache.expiry)
	}
}

func TestNewCacheWithExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	customExpiry := 30 * time.Minute
	cache, err := cache.NewCache(cachePath, cache.WithExpiry(customExpiry))
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}
	if cache == nil {
		t.Fatal("NewCache() returned nil")
	}

	// Verify cache has custom expiry set
	if cache.expiry == nil {
		t.Fatal("Cache expiry is nil")
	}
	if *cache.expiry != customExpiry {
		t.Errorf("Expected custom expiry of %v, got %v", customExpiry, *cache.expiry)
	}
}

func TestCachePutGet(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	key := []byte("test_key")
	value := []byte("test_value")

	// Test Put
	err = cache.Put(key, value)
	if err != nil {
		t.Fatalf("Put() returned error: %v", err)
	}

	// Test Get
	retrievedValue, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(retrievedValue))
	}
}

func TestCacheGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	key := []byte("non_existent_key")

	// Test Get for non-existent key
	_, err = cache.Get(key)
	if err == nil {
		t.Fatal("Expected error for non-existent key, got nil")
	}
}

func TestCacheOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	key := []byte("test_key")
	value1 := []byte("first_value")
	value2 := []byte("second_value")

	// Put first value
	err = cache.Put(key, value1)
	if err != nil {
		t.Fatalf("First Put() returned error: %v", err)
	}

	// Put second value (overwrite)
	err = cache.Put(key, value2)
	if err != nil {
		t.Fatalf("Second Put() returned error: %v", err)
	}

	// Get value (should be second value)
	retrievedValue, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}

	if string(retrievedValue) != string(value2) {
		t.Errorf("Expected value %s, got %s", string(value2), string(retrievedValue))
	}
}

func TestCacheMultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Put all values
	for k, v := range testData {
		err := cache.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("Put() returned error for key %s: %v", k, err)
		}
	}

	// Get all values and verify
	for k, expectedV := range testData {
		retrievedValue, err := cache.Get([]byte(k))
		if err != nil {
			t.Fatalf("Get() returned error for key %s: %v", k, err)
		}

		if string(retrievedValue) != expectedV {
			t.Errorf("Expected value %s for key %s, got %s", expectedV, k, string(retrievedValue))
		}
	}
}

func TestCacheEmptyKeyValue(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	// Test empty key
	emptyKey := []byte("")
	value := []byte("test_value")

	err = cache.Put(emptyKey, value)
	if err != nil {
		t.Fatalf("Put() with empty key returned error: %v", err)
	}

	retrievedValue, err := cache.Get(emptyKey)
	if err != nil {
		t.Fatalf("Get() with empty key returned error: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s for empty key, got %s", string(value), string(retrievedValue))
	}

	// Test empty value
	key := []byte("test_key")
	emptyValue := []byte("")

	err = cache.Put(key, emptyValue)
	if err != nil {
		t.Fatalf("Put() with empty value returned error: %v", err)
	}

	retrievedValue, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Get() with empty value returned error: %v", err)
	}

	if string(retrievedValue) != string(emptyValue) {
		t.Errorf("Expected empty value for key, got %s", string(retrievedValue))
	}
}

func TestCachePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	key := []byte("persistence_key")
	value := []byte("persistence_value")

	// Create cache, put value, and close
	{
		cache, err := cache.NewCache(cachePath)
		if err != nil {
			t.Fatalf("NewCache() returned error: %v", err)
		}

		err = cache.Put(key, value)
		if err != nil {
			t.Fatalf("Put() returned error: %v", err)
		}
	}

	// Create new cache instance and verify value persists
	{
		cache, err := cache.NewCache(cachePath)
		if err != nil {
			t.Fatalf("NewCache() for persistence test returned error: %v", err)
		}

		retrievedValue, err := cache.Get(key)
		if err != nil {
			t.Fatalf("Get() for persistence test returned error: %v", err)
		}

		if string(retrievedValue) != string(value) {
			t.Errorf("Expected persisted value %s, got %s", string(value), string(retrievedValue))
		}
	}
}

func TestCacheInvalidPath(t *testing.T) {
	// Try to create cache in invalid directory
	invalidPath := "/root/invalid/path/cache.db"

	_, err := cache.NewCache(invalidPath)
	if err == nil {
		t.Fatal("Expected error for invalid cache path, got nil")
	}
}

func TestWithExpiryOption(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	expiry1 := 1 * time.Hour
	expiry2 := 2 * time.Hour

	// Test single expiry option
	cache1, err := cache.NewCache(cachePath, cache.WithExpiry(expiry1))
	if err != nil {
		t.Fatalf("NewCache() with expiry returned error: %v", err)
	}

	if *cache1.expiry != expiry1 {
		t.Errorf("Expected expiry %v, got %v", expiry1, *cache1.expiry)
	}

	// Test multiple options (last one should win)
	cache2, err := cache.NewCache(cachePath+"2", cache.WithExpiry(expiry1), cache.WithExpiry(expiry2))
	if err != nil {
		t.Fatalf("NewCache() with multiple expiry options returned error: %v", err)
	}

	if *cache2.expiry != expiry2 {
		t.Errorf("Expected expiry %v (last option), got %v", expiry2, *cache2.expiry)
	}
}

func TestCacheBinaryData(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	key := []byte("binary_key")
	// Test with binary data including null bytes
	value := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x00, 0x00}

	err = cache.Put(key, value)
	if err != nil {
		t.Fatalf("Put() with binary data returned error: %v", err)
	}

	retrievedValue, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get() with binary data returned error: %v", err)
	}

	if len(retrievedValue) != len(value) {
		t.Errorf("Expected binary data length %d, got %d", len(value), len(retrievedValue))
	}

	for i, b := range value {
		if i >= len(retrievedValue) || retrievedValue[i] != b {
			t.Errorf("Binary data mismatch at index %d: expected %02x, got %02x", i, b, retrievedValue[i])
		}
	}
}
