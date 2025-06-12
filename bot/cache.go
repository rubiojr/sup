package bot

import (
	"time"

	"github.com/rubiojr/kv"
)

type Cache interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
}

type CacheOption func(*kvCache)

func WithExpiry(expiry time.Duration) CacheOption {
	return func(c *kvCache) {
		c.expiry = &expiry
	}
}

type kvCache struct {
	db     kv.Database
	expiry *time.Duration
}

func NewCache(path string, opts ...CacheOption) (*kvCache, error) {
	db, err := kv.New("sqlite", path)
	if err != nil {
		return nil, err
	}

	cache := &kvCache{db: db}
	for _, opt := range opts {
		opt(cache)
	}

	if cache.expiry == nil {
		defaultExpiry := 1 * time.Hour
		cache.expiry = &defaultExpiry
	}

	return cache, nil
}

func (c *kvCache) Get(key []byte) ([]byte, error) {
	return c.db.Get(string(key))
}

func (c *kvCache) Put(key []byte, value []byte) error {
	var expireAt *time.Time
	if c.expiry != nil {
		expiry := time.Now().Add(*c.expiry)
		expireAt = &expiry
	}
	return c.db.Set(string(key), value, expireAt)
}
